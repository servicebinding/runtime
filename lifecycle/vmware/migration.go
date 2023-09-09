/*
Copyright 2023 the original author or authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package vmware

import (
	"context"
	"errors"
	"fmt"
	"regexp"

	"github.com/go-logr/logr"
	"github.com/vmware-labs/reconciler-runtime/reconcilers"
	corev1 "k8s.io/api/core/v1"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/servicebinding/runtime/apis/duck"
	servicebindingv1beta1 "github.com/servicebinding/runtime/apis/v1beta1"
	"github.com/servicebinding/runtime/lifecycle"
)

func InstallMigrationHooks(hooks lifecycle.ServiceBindingHooks) lifecycle.ServiceBindingHooks {
	serviceBindingPostProjection := hooks.ServiceBindingPostProjection
	hooks.ServiceBindingPostProjection = func(ctx context.Context, binding *servicebindingv1beta1.ServiceBinding) error {
		if err := CleanupServiceBinding(ctx, binding); err != nil {
			return err
		}
		if serviceBindingPostProjection != nil {
			return serviceBindingPostProjection(ctx, binding)
		}
		return nil
	}

	workloadPreProjection := hooks.WorkloadPreProjection
	hooks.WorkloadPreProjection = func(ctx context.Context, workload runtime.Object) error {
		if err := CleanupWorkload(ctx, workload); err != nil {
			return err
		}
		if workloadPreProjection != nil {
			return workloadPreProjection(ctx, workload)
		}
		return nil
	}

	return hooks
}

func CleanupServiceBinding(ctx context.Context, binding *servicebindingv1beta1.ServiceBinding) error {
	if reconcilers.RetrieveRequest(ctx).Name == "" {
		// we're not in a reconciler
		return nil
	}
	c, err := reconcilers.RetrieveConfig(ctx)
	if err != nil {
		// we're not in a reconciler
		return nil
	}

	log := logr.FromContextOrDiscard(ctx).
		WithName("VMware").
		WithName("CleanupServiceBinding")
	ctx = logr.NewContext(ctx, log)

	// drop ProjectionReady condition (Ready and ServiceAvailable exist in both implementations)
	conds := []metav1.Condition{}
	for _, c := range binding.Status.Conditions {
		if c.Type == "ProjectionReady" {
			continue
		}
		conds = append(conds, c)
	}
	binding.Status.Conditions = conds

	// drop internal resource
	mapping, err := c.RESTMapper().RESTMapping(schema.GroupKind{Group: "internal.bindings.labs.vmware.com", Kind: "ServiceBindingProjection"}, "v1alpha1")
	if err != nil || mapping == nil {
		if !errors.Is(err, &meta.NoKindMatchError{}) {
			return err
		} else {
			return nil
		}
	}

	sbp := &unstructured.Unstructured{}
	sbp.SetAPIVersion("internal.bindings.labs.vmware.com/v1alpha1")
	sbp.SetKind("ServiceBindingProjection")
	if err := c.Get(ctx, client.ObjectKey{Namespace: binding.Namespace, Name: binding.Name}, sbp); err != nil {
		if !apierrs.IsNotFound(err) {
			log.Error(err, "unable to load ServiceBindingProjection")
		}
		return nil
	}
	if len(sbp.GetFinalizers()) != 0 {
		// drop all finalizers at once
		finalizer := sbp.GetFinalizers()[0]
		sbp.SetFinalizers([]string{finalizer})
		if err := reconcilers.ClearFinalizer(ctx, sbp, finalizer); err != nil {
			log.Error(err, "unable to clear finalizer for ServiceBindingProjection", "finalizer", finalizer)
		}
	}
	if sbp.GetDeletionTimestamp() == nil {
		if err := c.Delete(ctx, sbp); err != nil {
			log.Error(err, "unable to delete ServiceBindingProjection")
		}
	}

	return nil
}

var bindingAnnotationRE = regexp.MustCompile(`^internal\.bindings\.labs\.vmware\.com/projection-[0-9a-f]{40}$`)

func CleanupWorkload(ctx context.Context, workload runtime.Object) error {
	log := logr.FromContextOrDiscard(ctx).
		WithName("VMware").
		WithName("CleanupWorkload")
	ctx = logr.NewContext(ctx, log)

	cast := &reconcilers.CastResource[client.Object, *duck.PodSpecable]{
		Reconciler: &reconcilers.SyncReconciler[*duck.PodSpecable]{
			Sync: func(ctx context.Context, workload *duck.PodSpecable) error {
				// the VMware implementation only bound into PodSpecable resources
				for k, v := range workload.Annotations {
					if !bindingAnnotationRE.MatchString(k) {
						continue
					}

					bindingDigest := k[45:]
					secretName := v
					cleanupBinding(workload, bindingDigest, secretName)
				}

				return nil
			},
		},
	}

	if _, err := cast.Reconcile(ctx, workload.(client.Object)); err != nil {
		// bail out, but don't fail
		log.Error(err, "unexpected error during cleanup")
		return nil
	}

	return nil
}

func cleanupBinding(workload *duck.PodSpecable, bindingDigest, secretName string) {
	volumeName := fmt.Sprintf("binding-%s", bindingDigest)

	volumes := []corev1.Volume{}
	for _, volume := range workload.Spec.Template.Spec.Volumes {
		if volume.Name != volumeName {
			volumes = append(volumes, volume)
		}
	}
	workload.Spec.Template.Spec.Volumes = volumes

	for i := range workload.Spec.Template.Spec.Containers {
		cleanupBindingContainer(&workload.Spec.Template.Spec.Containers[i], bindingDigest, secretName)
	}
	for i := range workload.Spec.Template.Spec.InitContainers {
		cleanupBindingContainer(&workload.Spec.Template.Spec.InitContainers[i], bindingDigest, secretName)
	}

	delete(workload.Annotations, fmt.Sprintf("internal.bindings.labs.vmware.com/projection-%s", bindingDigest))
	delete(workload.Annotations, fmt.Sprintf("internal.bindings.labs.vmware.com/projection-%s-type", bindingDigest))
	delete(workload.Annotations, fmt.Sprintf("internal.bindings.labs.vmware.com/projection-%s-provider", bindingDigest))
}

func cleanupBindingContainer(container *corev1.Container, bindingDigest, secretName string) {
	volumeName := fmt.Sprintf("binding-%s", bindingDigest)
	typeFieldPath := fmt.Sprintf("metadata.annotations['internal.bindings.labs.vmware.com/projection-%s-type']", bindingDigest)
	providerFieldPath := fmt.Sprintf("metadata.annotations['internal.bindings.labs.vmware.com/projection-%s-provider']", bindingDigest)

	volumeMounts := []corev1.VolumeMount{}
	for _, volumeMount := range container.VolumeMounts {
		if volumeMount.Name != volumeName {
			volumeMounts = append(volumeMounts, volumeMount)
		}
	}
	container.VolumeMounts = volumeMounts

	env := []corev1.EnvVar{}
	for _, envvar := range container.Env {
		isFromBoundSecret := envvar.ValueFrom != nil && envvar.ValueFrom.SecretKeyRef != nil && envvar.ValueFrom.SecretKeyRef.Name == secretName
		isFromTypeOverride := envvar.ValueFrom != nil && envvar.ValueFrom.FieldRef != nil && envvar.ValueFrom.FieldRef.FieldPath == typeFieldPath
		isFromProviderOverride := envvar.ValueFrom != nil && envvar.ValueFrom.FieldRef != nil && envvar.ValueFrom.FieldRef.FieldPath == providerFieldPath
		if !isFromBoundSecret && !isFromTypeOverride && !isFromProviderOverride {
			env = append(env, envvar)
		}
	}
	container.Env = env
}

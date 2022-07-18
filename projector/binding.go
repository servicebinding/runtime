/*
Copyright 2021 The Kubernetes Authors.

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

package projector

import (
	"context"
	"fmt"
	"path"
	"sort"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/sets"

	servicebindingv1beta1 "github.com/servicebinding/service-binding-controller/apis/v1beta1"
)

const (
	ServiceBindingRootEnv    = "SERVICE_BINDING_ROOT"
	Group                    = "projector.servicebinding.io"
	VolumePrefix             = "servicebinding-"
	SecretAnnotationPrefix   = Group + "/secret-"
	TypeAnnotationPrefix     = Group + "/type-"
	ProviderAnnotationPrefix = Group + "/provider-"
)

var _ ServiceBindingProjector = (*serviceBindingProjector)(nil)

type serviceBindingProjector struct {
	mappingSource MappingSource
}

// New creates a service binding projector configured for the mapping source. The binding projector is typically created
// once and applied to multiple workloads.
func New(mappingSource MappingSource) ServiceBindingProjector {
	return &serviceBindingProjector{
		mappingSource: mappingSource,
	}
}

func (p *serviceBindingProjector) Project(ctx context.Context, binding *servicebindingv1beta1.ServiceBinding, workload runtime.Object) error {
	mapping, err := p.mappingSource.LookupMapping(ctx, workload)
	if err != nil {
		return err
	}
	mpt, err := NewMetaPodTemplate(ctx, workload, mapping)
	if err != nil {
		return err
	}
	p.project(binding, mpt)
	return mpt.WriteToWorkload(ctx)
}

func (p *serviceBindingProjector) Unproject(ctx context.Context, binding *servicebindingv1beta1.ServiceBinding, workload runtime.Object) error {
	mapping, err := p.mappingSource.LookupMapping(ctx, workload)
	if err != nil {
		return err
	}
	mpt, err := NewMetaPodTemplate(ctx, workload, mapping)
	if err != nil {
		return err
	}
	p.unproject(binding, mpt)
	return mpt.WriteToWorkload(ctx)
}

func (p *serviceBindingProjector) project(binding *servicebindingv1beta1.ServiceBinding, mpt *metaPodTemplate) {
	// rather than attempt to merge an existing binding, unproject it
	p.unproject(binding, mpt)

	if p.secretName(binding) == "" {
		// no secret to bind
		return
	}
	p.projectVolume(binding, mpt)
	for i := range mpt.Containers {
		p.projectContainer(binding, mpt, &mpt.Containers[i])
	}
}

func (p *serviceBindingProjector) unproject(binding *servicebindingv1beta1.ServiceBinding, mpt *metaPodTemplate) {
	p.unprojectVolume(binding, mpt)
	for i := range mpt.Containers {
		p.unprojectContainer(binding, mpt, &mpt.Containers[i])
	}

	// cleanup annotations
	delete(mpt.Annotations, p.secretAnnotationName(binding))
	delete(mpt.Annotations, p.typeAnnotationName(binding))
	delete(mpt.Annotations, p.providerAnnotationName(binding))
}

func (p *serviceBindingProjector) projectVolume(binding *servicebindingv1beta1.ServiceBinding, mpt *metaPodTemplate) {
	volume := corev1.Volume{
		Name: p.volumeName(binding),
		VolumeSource: corev1.VolumeSource{
			Projected: &corev1.ProjectedVolumeSource{
				Sources: []corev1.VolumeProjection{
					{
						Secret: &corev1.SecretProjection{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: p.secretAnnotation(binding, mpt),
							},
						},
					},
				},
			},
		},
	}
	if binding.Spec.Type != "" {
		volume.VolumeSource.Projected.Sources = append(volume.VolumeSource.Projected.Sources,
			corev1.VolumeProjection{
				DownwardAPI: &corev1.DownwardAPIProjection{
					Items: []corev1.DownwardAPIVolumeFile{
						{
							Path: "type",
							FieldRef: &corev1.ObjectFieldSelector{
								FieldPath: fmt.Sprintf("metadata.annotations['%s']", p.typeAnnotation(binding, mpt)),
							},
						},
					},
				},
			},
		)
	}
	if binding.Spec.Provider != "" {
		volume.VolumeSource.Projected.Sources = append(volume.VolumeSource.Projected.Sources,
			corev1.VolumeProjection{
				DownwardAPI: &corev1.DownwardAPIProjection{
					Items: []corev1.DownwardAPIVolumeFile{
						{
							Path: "provider",
							FieldRef: &corev1.ObjectFieldSelector{
								FieldPath: fmt.Sprintf("metadata.annotations['%s']", p.providerAnnotation(binding, mpt)),
							},
						},
					},
				},
			},
		)
	}

	mpt.Volumes = append(mpt.Volumes, volume)

	// sort projected volumes
	sort.SliceStable(mpt.Volumes, func(i, j int) bool {
		ii := mpt.Volumes[i]
		jj := mpt.Volumes[j]
		ip := strings.HasPrefix(ii.Name, VolumePrefix)
		jp := strings.HasPrefix(jj.Name, VolumePrefix)
		if ip && jp {
			// sort projected items by name
			return ii.Name < jj.Name
		}
		if jp {
			// keep projected items after non-projected items
			return !ip
		}
		// preserve order of non-projected items
		return false
	})
}

func (p *serviceBindingProjector) unprojectVolume(binding *servicebindingv1beta1.ServiceBinding, mpt *metaPodTemplate) {
	volumes := []corev1.Volume{}
	projected := p.volumeName(binding)
	for _, v := range mpt.Volumes {
		if v.Name != projected {
			volumes = append(volumes, v)
		}
	}
	mpt.Volumes = volumes
}

func (p *serviceBindingProjector) projectContainer(binding *servicebindingv1beta1.ServiceBinding, mpt *metaPodTemplate, mc *metaContainer) {
	if !p.isContainerBindable(binding, mc) {
		return
	}
	p.projectVolumeMount(binding, mc)
	p.projectEnv(binding, mpt, mc)
}

func (p *serviceBindingProjector) unprojectContainer(binding *servicebindingv1beta1.ServiceBinding, mpt *metaPodTemplate, mc *metaContainer) {
	p.unprojectVolumeMount(binding, mc)
	p.unprojectEnv(binding, mpt, mc)
}

func (p *serviceBindingProjector) projectVolumeMount(binding *servicebindingv1beta1.ServiceBinding, mc *metaContainer) {
	mc.VolumeMounts = append(mc.VolumeMounts, corev1.VolumeMount{
		Name:      p.volumeName(binding),
		ReadOnly:  true,
		MountPath: path.Join(p.serviceBindingRoot(mc), binding.Spec.Name),
	})

	// sort projected volume mounts
	sort.SliceStable(mc.VolumeMounts, func(i, j int) bool {
		ii := mc.VolumeMounts[i]
		jj := mc.VolumeMounts[j]
		ip := strings.HasPrefix(ii.Name, VolumePrefix)
		jp := strings.HasPrefix(jj.Name, VolumePrefix)
		if ip && jp {
			// sort projected items by name
			return ii.Name < jj.Name
		}
		if jp {
			// keep projected items after non-projected items
			return !ip
		}
		// preserve order of non-projected items
		return false
	})
}

func (p *serviceBindingProjector) unprojectVolumeMount(binding *servicebindingv1beta1.ServiceBinding, mc *metaContainer) {
	mounts := []corev1.VolumeMount{}
	projected := p.volumeName(binding)
	for _, m := range mc.VolumeMounts {
		if m.Name != projected {
			mounts = append(mounts, m)
		}
	}
	mc.VolumeMounts = mounts
}

func (p *serviceBindingProjector) projectEnv(binding *servicebindingv1beta1.ServiceBinding, mpt *metaPodTemplate, mc *metaContainer) {
	for _, e := range binding.Spec.Env {
		if e.Key == "type" && binding.Spec.Type != "" {
			mc.Env = append(mc.Env, corev1.EnvVar{
				Name: e.Name,
				ValueFrom: &corev1.EnvVarSource{
					FieldRef: &corev1.ObjectFieldSelector{
						FieldPath: fmt.Sprintf("metadata.annotations['%s']", p.typeAnnotation(binding, mpt)),
					},
				},
			})
			continue
		}
		if e.Key == "provider" && binding.Spec.Provider != "" {
			mc.Env = append(mc.Env, corev1.EnvVar{
				Name: e.Name,
				ValueFrom: &corev1.EnvVarSource{
					FieldRef: &corev1.ObjectFieldSelector{
						FieldPath: fmt.Sprintf("metadata.annotations['%s']", p.providerAnnotation(binding, mpt)),
					},
				},
			})
			continue
		}
		mc.Env = append(mc.Env, corev1.EnvVar{
			Name: e.Name,
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: p.secretAnnotation(binding, mpt),
					},
					Key: e.Key,
				},
			},
		})
	}

	// sort projected env vars
	secrets := p.knownProjectedSecrets(mpt)
	sort.SliceStable(mc.Env, func(i, j int) bool {
		ii := mc.Env[i]
		jj := mc.Env[j]
		ip := p.isProjectedEnv(ii, secrets)
		jp := p.isProjectedEnv(jj, secrets)
		if ip && jp {
			// sort projected items by name
			return ii.Name < jj.Name
		}
		if jp {
			// keep projected items after non-projected items
			return !ip
		}
		// preserve order of non-projected items
		return false
	})
}

func (p *serviceBindingProjector) unprojectEnv(binding *servicebindingv1beta1.ServiceBinding, mpt *metaPodTemplate, mc *metaContainer) {
	env := []corev1.EnvVar{}
	secret := mpt.Annotations[p.secretAnnotationName(binding)]
	typeFieldPath := fmt.Sprintf("metadata.annotations['%s']", p.typeAnnotationName(binding))
	providerFieldPath := fmt.Sprintf("metadata.annotations['%s']", p.providerAnnotationName(binding))
	for _, e := range mc.Env {
		// NB we do not remove the SERVICE_BINDING_ROOT env var since we don't know if someone else is depending on it
		remove := false
		if e.ValueFrom != nil && e.ValueFrom.SecretKeyRef != nil && e.ValueFrom.SecretKeyRef.Name == secret {
			// projected from secret
			remove = true
		}
		if e.ValueFrom != nil && e.ValueFrom.FieldRef != nil {
			if e.ValueFrom.FieldRef.FieldPath == typeFieldPath {
				// custom type env var
				remove = true
			}
			if e.ValueFrom.FieldRef.FieldPath == providerFieldPath {
				// custom provider env var
				remove = true
			}
		}
		if !remove {
			env = append(env, e)
		}
	}
	mc.Env = env
}

func (p *serviceBindingProjector) isContainerBindable(binding *servicebindingv1beta1.ServiceBinding, mc *metaContainer) bool {
	if len(binding.Spec.Workload.Containers) == 0 || mc.Name == nil {
		return true
	}
	for _, name := range binding.Spec.Workload.Containers {
		if name == *mc.Name {
			return true
		}
	}
	return false
}

func (p *serviceBindingProjector) serviceBindingRoot(mc *metaContainer) string {
	for _, e := range mc.Env {
		if e.Name == ServiceBindingRootEnv {
			return e.Value
		}
	}
	// define default value
	serviceBindingRoot := corev1.EnvVar{
		Name:  ServiceBindingRootEnv,
		Value: "/bindings",
	}
	mc.Env = append(mc.Env, serviceBindingRoot)
	return serviceBindingRoot.Value
}

func (p *serviceBindingProjector) isProjectedEnv(e corev1.EnvVar, secrets sets.String) bool {
	if e.ValueFrom != nil && e.ValueFrom.SecretKeyRef != nil && secrets.Has(e.ValueFrom.SecretKeyRef.Name) {
		// projected from secret
		return true
	}
	if e.ValueFrom != nil && e.ValueFrom.FieldRef != nil && strings.HasPrefix(e.ValueFrom.FieldRef.FieldPath, fmt.Sprintf("metadata.annotations['%s", Group)) {
		// projected custom type or annotation
		return true
	}
	return false
}

func (p *serviceBindingProjector) knownProjectedSecrets(mpt *metaPodTemplate) sets.String {
	secrets := sets.NewString()
	for k, v := range mpt.Annotations {
		if strings.HasPrefix(k, SecretAnnotationPrefix) {
			secrets.Insert(v)
		}
	}
	return secrets
}

func (p *serviceBindingProjector) secretName(binding *servicebindingv1beta1.ServiceBinding) string {
	if binding.Status.Binding == nil {
		return ""
	}
	return binding.Status.Binding.Name
}

func (p *serviceBindingProjector) secretAnnotation(binding *servicebindingv1beta1.ServiceBinding, mpt *metaPodTemplate) string {
	key := p.secretAnnotationName(binding)
	secret := p.secretName(binding)
	if secret == "" {
		return ""
	}
	mpt.Annotations[key] = secret
	return secret
}

func (p *serviceBindingProjector) secretAnnotationName(binding *servicebindingv1beta1.ServiceBinding) string {
	return fmt.Sprintf("%s%s", SecretAnnotationPrefix, binding.UID)
}

func (p *serviceBindingProjector) volumeName(binding *servicebindingv1beta1.ServiceBinding) string {
	return fmt.Sprintf("%s%s", VolumePrefix, binding.UID)
}

func (p *serviceBindingProjector) typeAnnotation(binding *servicebindingv1beta1.ServiceBinding, mpt *metaPodTemplate) string {
	key := p.typeAnnotationName(binding)
	mpt.Annotations[key] = binding.Spec.Type
	return key
}

func (p *serviceBindingProjector) typeAnnotationName(binding *servicebindingv1beta1.ServiceBinding) string {
	return fmt.Sprintf("%s%s", TypeAnnotationPrefix, binding.UID)
}

func (p *serviceBindingProjector) providerAnnotation(binding *servicebindingv1beta1.ServiceBinding, mpt *metaPodTemplate) string {
	key := p.providerAnnotationName(binding)
	mpt.Annotations[key] = binding.Spec.Provider
	return key
}

func (p *serviceBindingProjector) providerAnnotationName(binding *servicebindingv1beta1.ServiceBinding) string {
	return fmt.Sprintf("%s%s", ProviderAnnotationPrefix, binding.UID)
}

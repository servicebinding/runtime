/*
Copyright 2022 the original author or authors.

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

package controllers

import (
	"context"
	"fmt"

	"github.com/vmware-labs/reconciler-runtime/apis"
	"github.com/vmware-labs/reconciler-runtime/reconcilers"
	corev1 "k8s.io/api/core/v1"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	ctlr "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	servicebindingv1beta1 "github.com/servicebinding/runtime/apis/v1beta1"
	"github.com/servicebinding/runtime/lifecycle"
)

//+kubebuilder:rbac:groups=servicebinding.io,resources=servicebindings,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=servicebinding.io,resources=servicebindings/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=servicebinding.io,resources=servicebindings/finalizers,verbs=update
//+kubebuilder:rbac:groups=core,resources=events,verbs=get;list;watch;create;update;patch;delete

// ServiceBindingReconciler reconciles a ServiceBinding object
func ServiceBindingReconciler(c reconcilers.Config, hooks lifecycle.ServiceBindingHooks) *reconcilers.ResourceReconciler[*servicebindingv1beta1.ServiceBinding] {
	return &reconcilers.ResourceReconciler[*servicebindingv1beta1.ServiceBinding]{
		Reconciler: &reconcilers.WithFinalizer[*servicebindingv1beta1.ServiceBinding]{
			Finalizer: servicebindingv1beta1.GroupVersion.Group + "/finalizer",
			Reconciler: reconcilers.Sequence[*servicebindingv1beta1.ServiceBinding]{
				ResolveBindingSecret(hooks),
				ResolveWorkloads(hooks),
				ProjectBinding(hooks),
				PatchWorkloads(),
			},
		},

		Config: c,
	}
}

func ResolveBindingSecret(hooks lifecycle.ServiceBindingHooks) reconcilers.SubReconciler[*servicebindingv1beta1.ServiceBinding] {
	return &reconcilers.SyncReconciler[*servicebindingv1beta1.ServiceBinding]{
		Name: "ResolveBindingSecret",
		Sync: func(ctx context.Context, resource *servicebindingv1beta1.ServiceBinding) error {
			c := reconcilers.RetrieveConfigOrDie(ctx)

			ref := corev1.ObjectReference{
				APIVersion: resource.Spec.Service.APIVersion,
				Kind:       resource.Spec.Service.Kind,
				Namespace:  resource.Namespace,
				Name:       resource.Spec.Service.Name,
			}
			secretName, err := hooks.GetResolver(TrackingClient(c)).LookupBindingSecret(ctx, ref)
			if err != nil {
				if apierrs.IsNotFound(err) {
					// leave Unknown, the provisioned service may be created shortly
					resource.GetConditionManager().MarkUnknown(servicebindingv1beta1.ServiceBindingConditionServiceAvailable, "ServiceNotFound", "the service was not found")
					return nil
				}
				if apierrs.IsForbidden(err) {
					// set False, the operator needs to give access to the resource
					// see https://servicebinding.io/spec/core/1.0.0/#considerations-for-role-based-access-control-rbac
					resource.GetConditionManager().MarkFalse(servicebindingv1beta1.ServiceBindingConditionServiceAvailable, "ServiceForbidden", "the controller does not have permission to get the service")
					return nil
				}
				// TODO handle other err cases
				return err
			}

			if secretName != "" {
				// success
				resource.GetConditionManager().MarkTrue(servicebindingv1beta1.ServiceBindingConditionServiceAvailable, "ResolvedBindingSecret", "")
				previousSecretName := ""
				if resource.Status.Binding != nil {
					previousSecretName = resource.Status.Binding.Name
				}
				resource.Status.Binding = &servicebindingv1beta1.ServiceBindingSecretReference{Name: secretName}
				if previousSecretName != secretName {
					// stop processing subreconcilers, we need to allow the secret to be updated on
					// the API Server so that webhook calls for workload that are targeted by the
					// binding are able to see this secret. The next turn of the reconciler for
					// this resource is automatically triggered by the change of status. We do not
					// want to requeue as that may cause the resource to be re-reconciled before
					// the informer cache is updated.
					return reconcilers.HaltSubReconcilers
				}
			} else {
				// leave Unknown, not success but also not an error
				resource.GetConditionManager().MarkUnknown(servicebindingv1beta1.ServiceBindingConditionServiceAvailable, "ServiceMissingBinding", "the service was found, but did not contain a binding secret")
				// TODO should we clear the existing binding?
				resource.Status.Binding = nil
			}

			return nil
		},
	}
}

func ResolveWorkloads(hooks lifecycle.ServiceBindingHooks) reconcilers.SubReconciler[*servicebindingv1beta1.ServiceBinding] {
	return &reconcilers.SyncReconciler[*servicebindingv1beta1.ServiceBinding]{
		Name:                   "ResolveWorkloads",
		SyncDuringFinalization: true,
		SyncWithResult: func(ctx context.Context, resource *servicebindingv1beta1.ServiceBinding) (reconcile.Result, error) {
			c := reconcilers.RetrieveConfigOrDie(ctx)

			ref := corev1.ObjectReference{
				APIVersion: resource.Spec.Workload.APIVersion,
				Kind:       resource.Spec.Workload.Kind,
				Namespace:  resource.Namespace,
				Name:       resource.Spec.Workload.Name,
			}
			workloads, err := hooks.GetResolver(TrackingClient(c)).LookupWorkloads(ctx, ref, resource.Spec.Workload.Selector)
			if err != nil {
				if apierrs.IsNotFound(err) {
					// leave Unknown, the workload may be created shortly
					resource.GetConditionManager().MarkUnknown(servicebindingv1beta1.ServiceBindingConditionWorkloadProjected, "WorkloadNotFound", "the workload was not found")
					// TODO use track rather than requeue
					return reconcile.Result{Requeue: true}, nil
				}
				if apierrs.IsForbidden(err) {
					// set False, the operator needs to give access to the resource
					// see https://servicebinding.io/spec/core/1.0.0/#considerations-for-role-based-access-control-rbac-1
					if resource.Spec.Workload.Name == "" {
						resource.GetConditionManager().MarkFalse(servicebindingv1beta1.ServiceBindingConditionWorkloadProjected, "WorkloadForbidden", "the controller does not have permission to list the workloads")
					} else {
						resource.GetConditionManager().MarkFalse(servicebindingv1beta1.ServiceBindingConditionWorkloadProjected, "WorkloadForbidden", "the controller does not have permission to get the workload")
					}
					// TODO use track rather than requeue
					return reconcile.Result{Requeue: true}, nil
				}
				// TODO handle other err cases
				return reconcile.Result{}, err
			}

			StashWorkloads(ctx, workloads)

			return reconcile.Result{}, nil
		},
	}
}

//+kubebuilder:rbac:groups=servicebinding.io,resources=clusterworkloadresourcemappings,verbs=get;list;watch

func ProjectBinding(hooks lifecycle.ServiceBindingHooks) reconcilers.SubReconciler[*servicebindingv1beta1.ServiceBinding] {
	return &reconcilers.SyncReconciler[*servicebindingv1beta1.ServiceBinding]{
		Name:                   "ProjectBinding",
		SyncDuringFinalization: true,
		Sync: func(ctx context.Context, resource *servicebindingv1beta1.ServiceBinding) error {
			c := reconcilers.RetrieveConfigOrDie(ctx)
			projector := hooks.GetProjector(hooks.GetResolver(TrackingClient(c)))

			workloads := RetrieveWorkloads(ctx)
			projectedWorkloads := make([]runtime.Object, len(workloads))

			if f := hooks.ServiceBindingPreProjection; f != nil {
				if err := f(ctx, resource); err != nil {
					return err
				}
			}
			for i := range workloads {
				workload := workloads[i].DeepCopyObject()

				if f := hooks.WorkloadPreProjection; f != nil {
					if err := f(ctx, workload); err != nil {
						return err
					}
				}
				if !resource.DeletionTimestamp.IsZero() {
					if err := projector.Unproject(ctx, resource, workload); err != nil {
						return err
					}
				} else {
					if err := projector.Project(ctx, resource, workload); err != nil {
						return err
					}
				}
				if f := hooks.WorkloadPostProjection; f != nil {
					if err := f(ctx, workload); err != nil {
						return err
					}
				}

				projectedWorkloads[i] = workload
			}
			if f := hooks.ServiceBindingPostProjection; f != nil {
				if err := f(ctx, resource); err != nil {
					return err
				}
			}

			StashProjectedWorkloads(ctx, projectedWorkloads)

			return nil
		},

		Setup: func(ctx context.Context, mgr ctlr.Manager, bldr *builder.Builder) error {
			bldr.Watches(&servicebindingv1beta1.ClusterWorkloadResourceMapping{}, handler.Funcs{})
			return nil
		},
	}
}

func PatchWorkloads() reconcilers.SubReconciler[*servicebindingv1beta1.ServiceBinding] {
	workloadManager := &reconcilers.ResourceManager[*unstructured.Unstructured]{
		Name: "PatchWorkloads",
		MergeBeforeUpdate: func(current, desired *unstructured.Unstructured) {
			current.SetUnstructuredContent(desired.UnstructuredContent())
		},
	}

	return &reconcilers.SyncReconciler[*servicebindingv1beta1.ServiceBinding]{
		Name:                   "PatchWorkloads",
		SyncDuringFinalization: true,
		Sync: func(ctx context.Context, resource *servicebindingv1beta1.ServiceBinding) error {
			workloads := RetrieveWorkloads(ctx)
			projectedWorkloads := RetrieveProjectedWorkloads(ctx)

			if len(workloads) != len(projectedWorkloads) {
				panic(fmt.Errorf("workloads and projectedWorkloads must have the same number of items"))
			}

			for i := range workloads {
				workload := workloads[i].(*unstructured.Unstructured)
				projectedWorkload := projectedWorkloads[i].(*unstructured.Unstructured)
				if workload.GetUID() != projectedWorkload.GetUID() || workload.GetResourceVersion() != projectedWorkload.GetResourceVersion() {
					panic(fmt.Errorf("workload and projectedWorkload must have the same uid and resourceVersion"))
				}

				if _, err := workloadManager.Manage(ctx, resource, workload, projectedWorkload); err != nil {
					if apierrs.IsNotFound(err) {
						// someone must have deleted the workload while we were operating on it
						continue
					}
					if apierrs.IsForbidden(err) {
						// set False, the operator needs to give access to the resource
						// see https://servicebinding.io/spec/core/1.0.0/#considerations-for-role-based-access-control-rbac-1
						resource.GetConditionManager().MarkFalse(servicebindingv1beta1.ServiceBindingConditionWorkloadProjected, "WorkloadForbidden", "the controller does not have permission to update the workloads")
						return nil
					}
					// TODO handle other err cases
					return err
				}
			}

			// update the WorkloadProjected condition to indicate success, but only if the condition has not already been set with another status
			if cond := resource.Status.GetCondition(servicebindingv1beta1.ServiceBindingConditionWorkloadProjected); apis.ConditionIsUnknown(cond) && cond.Reason == "Initializing" {
				resource.GetConditionManager().MarkTrue(servicebindingv1beta1.ServiceBindingConditionWorkloadProjected, "WorkloadProjected", "")
			}

			return nil
		},
	}
}

const WorkloadsStashKey reconcilers.StashKey = "servicebinding.io:workloads"

func StashWorkloads(ctx context.Context, workloads []runtime.Object) {
	reconcilers.StashValue(ctx, WorkloadsStashKey, workloads)
}

func RetrieveWorkloads(ctx context.Context) []runtime.Object {
	value := reconcilers.RetrieveValue(ctx, WorkloadsStashKey)
	if workloads, ok := value.([]runtime.Object); ok {
		return workloads
	}
	return nil
}

const ProjectedWorkloadsStashKey reconcilers.StashKey = "servicebinding.io:projected-workloads"

func StashProjectedWorkloads(ctx context.Context, workloads []runtime.Object) {
	reconcilers.StashValue(ctx, ProjectedWorkloadsStashKey, workloads)
}

func RetrieveProjectedWorkloads(ctx context.Context) []runtime.Object {
	value := reconcilers.RetrieveValue(ctx, ProjectedWorkloadsStashKey)
	if workloads, ok := value.([]runtime.Object); ok {
		return workloads
	}
	return nil
}

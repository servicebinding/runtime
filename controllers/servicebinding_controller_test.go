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

package controllers_test

import (
	"fmt"
	"testing"

	dieappsv1 "dies.dev/apis/apps/v1"
	diecorev1 "dies.dev/apis/core/v1"
	diemetav1 "dies.dev/apis/meta/v1"
	"github.com/vmware-labs/reconciler-runtime/reconcilers"
	rtesting "github.com/vmware-labs/reconciler-runtime/testing"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	servicebindingv1beta1 "github.com/servicebinding/runtime/apis/v1beta1"
	"github.com/servicebinding/runtime/controllers"
	dieservicebindingv1beta1 "github.com/servicebinding/runtime/dies/v1beta1"
)

func TestServiceBindingReconciler(t *testing.T) {
	namespace := "test-namespace"
	name := "my-binding"
	uid := types.UID("dde10100-d7b3-4cba-9430-51d60a8612a6")
	secretName := "my-secret"
	key := types.NamespacedName{Namespace: namespace, Name: name}

	scheme := runtime.NewScheme()
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(servicebindingv1beta1.AddToScheme(scheme))

	now := metav1.Now().Rfc3339Copy()

	serviceBinding := dieservicebindingv1beta1.ServiceBindingBlank.
		MetadataDie(func(d *diemetav1.ObjectMetaDie) {
			d.Namespace(namespace)
			d.Name(name)
			d.UID(uid)
		}).
		SpecDie(func(d *dieservicebindingv1beta1.ServiceBindingSpecDie) {
			d.ServiceDie(func(d *dieservicebindingv1beta1.ServiceBindingServiceReferenceDie) {
				d.APIVersion("v1")
				d.Kind("Secret")
				d.Name(secretName)
			})
			d.WorkloadDie(func(d *dieservicebindingv1beta1.ServiceBindingWorkloadReferenceDie) {
				d.APIVersion("apps/v1")
				d.Kind("Deployment")
				d.Name("my-workload")
			})
		})

	workloadMapping := dieservicebindingv1beta1.ClusterWorkloadResourceMappingBlank.
		MetadataDie(func(d *diemetav1.ObjectMetaDie) {
			d.Name("deployments.apps")
		})

	workload := dieappsv1.DeploymentBlank.
		DieStamp(func(r *appsv1.Deployment) {
			r.APIVersion = "apps/v1"
			r.Kind = "Deployment"
		}).
		MetadataDie(func(d *diemetav1.ObjectMetaDie) {
			d.Namespace(namespace)
			d.Name("my-workload")
		}).
		SpecDie(func(d *dieappsv1.DeploymentSpecDie) {
			d.TemplateDie(func(d *diecorev1.PodTemplateSpecDie) {
				d.SpecDie(func(d *diecorev1.PodSpecDie) {
					d.ContainerDie("my-container", func(d *diecorev1.ContainerDie) {
						d.Image("scratch")
					})
				})
			})
		})
	projectedWorkload := workload.
		SpecDie(func(d *dieappsv1.DeploymentSpecDie) {
			d.TemplateDie(func(d *diecorev1.PodTemplateSpecDie) {
				d.MetadataDie(func(d *diemetav1.ObjectMetaDie) {
					d.AddAnnotation(fmt.Sprintf("projector.servicebinding.io/secret-%s", uid), secretName)
				})
				d.SpecDie(func(d *diecorev1.PodSpecDie) {
					d.ContainerDie("my-container", func(d *diecorev1.ContainerDie) {
						d.EnvDie("SERVICE_BINDING_ROOT", func(d *diecorev1.EnvVarDie) {
							d.Value("/bindings")
						})
						d.VolumeMountDie(fmt.Sprintf("servicebinding-%s", uid), func(d *diecorev1.VolumeMountDie) {
							d.MountPath(fmt.Sprintf("/bindings/%s", name))
							d.ReadOnly(true)
						})
					})
					d.VolumeDie(fmt.Sprintf("servicebinding-%s", uid), func(d *diecorev1.VolumeDie) {
						d.ProjectedDie(func(d *diecorev1.ProjectedVolumeSourceDie) {
							d.SourcesDie(
								diecorev1.VolumeProjectionBlank.
									SecretDie(func(d *diecorev1.SecretProjectionDie) {
										d.Name(secretName)
									}),
							)
						})
					})
				})
			})
		})
	// TODO find a better way to avoid empty vs nil objects that are lost in the unstructured conversion
	unprojectedWorkload := workload.
		SpecDie(func(d *dieappsv1.DeploymentSpecDie) {
			d.TemplateDie(func(d *diecorev1.PodTemplateSpecDie) {
				d.SpecDie(func(d *diecorev1.PodSpecDie) {
					d.ContainerDie("my-container", func(d *diecorev1.ContainerDie) {
						d.EnvDie("SERVICE_BINDING_ROOT", func(d *diecorev1.EnvVarDie) {
							d.Value("/bindings")
						})
					})
				})
			})
		}).DieReleaseUnstructured()
	unstructured.SetNestedMap(unprojectedWorkload.UnstructuredContent(), map[string]interface{}{}, "spec", "template", "metadata", "annotations")
	containers, _, _ := unstructured.NestedSlice(unprojectedWorkload.UnstructuredContent(), "spec", "template", "spec", "containers")
	unstructured.SetNestedSlice(containers[0].(map[string]interface{}), []interface{}{}, "volumeMounts")
	unstructured.SetNestedSlice(unprojectedWorkload.UnstructuredContent(), containers, "spec", "template", "spec", "containers")
	unstructured.SetNestedSlice(unprojectedWorkload.UnstructuredContent(), []interface{}{}, "spec", "template", "spec", "volumes")

	rts := rtesting.ReconcilerTests{
		"in sync": {
			Key: key,
			GivenObjects: []client.Object{
				serviceBinding.
					MetadataDie(func(d *diemetav1.ObjectMetaDie) {
						d.Finalizers("servicebinding.io/finalizer")
					}).
					StatusDie(func(d *dieservicebindingv1beta1.ServiceBindingStatusDie) {
						d.ConditionsDie(
							dieservicebindingv1beta1.ServiceBindingConditionReady.True().Reason("ServiceBound"),
							dieservicebindingv1beta1.ServiceBindingConditionServiceAvailable.True().Reason("ResolvedBindingSecret"),
							dieservicebindingv1beta1.ServiceBindingConditionWorkloadProjected.True().Reason("WorkloadProjected"),
						)
						d.BindingDie(func(d *dieservicebindingv1beta1.ServiceBindingSecretReferenceDie) {
							d.Name(secretName)
						})
					}),
				projectedWorkload,
			},
			ExpectTracks: []rtesting.TrackRequest{
				rtesting.NewTrackRequest(projectedWorkload, serviceBinding, scheme),
				rtesting.NewTrackRequest(workloadMapping, serviceBinding, scheme),
			},
		},
		"newly created": {
			Key: key,
			GivenObjects: []client.Object{
				serviceBinding,
				workload,
			},
			ExpectTracks: []rtesting.TrackRequest{
				rtesting.NewTrackRequest(projectedWorkload, serviceBinding, scheme),
				rtesting.NewTrackRequest(workloadMapping, serviceBinding, scheme),
			},
			ExpectEvents: []rtesting.Event{
				rtesting.NewEvent(serviceBinding, scheme, corev1.EventTypeNormal, "FinalizerPatched", "Patched finalizer %q", "servicebinding.io/finalizer"),
				rtesting.NewEvent(serviceBinding, scheme, corev1.EventTypeNormal, "Updated", "Updated Deployment %q", "my-workload"),
				rtesting.NewEvent(serviceBinding, scheme, corev1.EventTypeNormal, "StatusUpdated", "Updated status"),
			},
			ExpectPatches: []rtesting.PatchRef{
				{
					Group:     "servicebinding.io",
					Kind:      "ServiceBinding",
					Namespace: serviceBinding.GetNamespace(),
					Name:      serviceBinding.GetName(),
					PatchType: types.MergePatchType,
					Patch:     []byte(`{"metadata":{"finalizers":["servicebinding.io/finalizer"],"resourceVersion":"999"}}`),
				},
			},
			ExpectUpdates: []client.Object{
				projectedWorkload.DieReleaseUnstructured().(client.Object),
			},
			ExpectStatusUpdates: []client.Object{
				serviceBinding.
					StatusDie(func(d *dieservicebindingv1beta1.ServiceBindingStatusDie) {
						d.ConditionsDie(
							dieservicebindingv1beta1.ServiceBindingConditionReady.True().Reason("ServiceBound"),
							dieservicebindingv1beta1.ServiceBindingConditionServiceAvailable.True().Reason("ResolvedBindingSecret"),
							dieservicebindingv1beta1.ServiceBindingConditionWorkloadProjected.True().Reason("WorkloadProjected"),
						)
						d.BindingDie(func(d *dieservicebindingv1beta1.ServiceBindingSecretReferenceDie) {
							d.Name(secretName)
						})
					}),
			},
		},
		"terminating": {
			Key: key,
			GivenObjects: []client.Object{
				serviceBinding.
					MetadataDie(func(d *diemetav1.ObjectMetaDie) {
						d.DeletionTimestamp(&now)
						d.Finalizers("servicebinding.io/finalizer")
					}),
				projectedWorkload,
			},
			ExpectTracks: []rtesting.TrackRequest{
				rtesting.NewTrackRequest(projectedWorkload, serviceBinding, scheme),
				rtesting.NewTrackRequest(workloadMapping, serviceBinding, scheme),
			},
			ExpectEvents: []rtesting.Event{
				rtesting.NewEvent(serviceBinding, scheme, corev1.EventTypeNormal, "Updated", "Updated Deployment %q", "my-workload"),
				rtesting.NewEvent(serviceBinding, scheme, corev1.EventTypeNormal, "FinalizerPatched", "Patched finalizer %q", "servicebinding.io/finalizer"),
			},
			ExpectPatches: []rtesting.PatchRef{
				{
					Group:     "servicebinding.io",
					Kind:      "ServiceBinding",
					Namespace: serviceBinding.GetNamespace(),
					Name:      serviceBinding.GetName(),
					PatchType: types.MergePatchType,
					Patch:     []byte(`{"metadata":{"finalizers":null,"resourceVersion":"999"}}`),
				},
			},
			ExpectUpdates: []client.Object{
				unprojectedWorkload.(client.Object),
			},
		},
	}

	rts.Run(t, scheme, func(t *testing.T, tc *rtesting.ReconcilerTestCase, c reconcilers.Config) reconcile.Reconciler {
		restMapper := c.RESTMapper().(*meta.DefaultRESTMapper)
		restMapper.Add(schema.GroupVersionKind{Group: "apps", Version: "v1", Kind: "Deployment"}, meta.RESTScopeNamespace)
		return controllers.ServiceBindingReconciler(c)
	})
}

func TestResolveBindingSecret(t *testing.T) {
	namespace := "test-namespace"
	name := "my-binding"

	scheme := runtime.NewScheme()
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(servicebindingv1beta1.AddToScheme(scheme))

	serviceBinding := dieservicebindingv1beta1.ServiceBindingBlank.
		MetadataDie(func(d *diemetav1.ObjectMetaDie) {
			d.Namespace(namespace)
			d.Name(name)
		})

	secretName := "my-secret"
	directSecretRef := dieservicebindingv1beta1.ServiceBindingServiceReferenceBlank.
		APIVersion("v1").
		Kind("Secret").
		Name(secretName)
	serviceRef := dieservicebindingv1beta1.ServiceBindingServiceReferenceBlank.
		APIVersion("example/v1").
		Kind("MyProvisionedService").
		Name("my-service")

	notProvisionedService := &unstructured.Unstructured{}
	notProvisionedService.SetAPIVersion("example/v1")
	notProvisionedService.SetKind("MyProvisionedService")
	notProvisionedService.SetNamespace(namespace)
	notProvisionedService.SetName("my-service")
	provisionedService := notProvisionedService.DeepCopy()
	provisionedService.UnstructuredContent()["status"] = map[string]interface{}{
		"binding": map[string]interface{}{
			"name": secretName,
		},
	}

	rts := rtesting.SubReconcilerTests{
		"resolve direct secret": {
			Resource: serviceBinding.
				SpecDie(func(d *dieservicebindingv1beta1.ServiceBindingSpecDie) {
					d.Service(directSecretRef.DieRelease())
				}),
			ExpectResource: serviceBinding.
				SpecDie(func(d *dieservicebindingv1beta1.ServiceBindingSpecDie) {
					d.Service(directSecretRef.DieRelease())
				}).
				StatusDie(func(d *dieservicebindingv1beta1.ServiceBindingStatusDie) {
					d.BindingDie(func(d *dieservicebindingv1beta1.ServiceBindingSecretReferenceDie) {
						d.Name(secretName)
					})
					d.ConditionsDie(
						dieservicebindingv1beta1.ServiceBindingConditionServiceAvailable.
							True().Reason("ResolvedBindingSecret"),
					)
				}),
		},
		"service is a provisioned service": {
			Resource: serviceBinding.
				SpecDie(func(d *dieservicebindingv1beta1.ServiceBindingSpecDie) {
					d.Service(serviceRef.DieRelease())
				}),
			GivenObjects: []client.Object{
				provisionedService,
			},
			ExpectResource: serviceBinding.
				SpecDie(func(d *dieservicebindingv1beta1.ServiceBindingSpecDie) {
					d.Service(serviceRef.DieRelease())
				}).
				StatusDie(func(d *dieservicebindingv1beta1.ServiceBindingStatusDie) {
					d.BindingDie(func(d *dieservicebindingv1beta1.ServiceBindingSecretReferenceDie) {
						d.Name(secretName)
					})
					d.ConditionsDie(
						dieservicebindingv1beta1.ServiceBindingConditionServiceAvailable.
							True().Reason("ResolvedBindingSecret"),
					)
				}),
			ExpectTracks: []rtesting.TrackRequest{
				rtesting.NewTrackRequest(provisionedService, serviceBinding, scheme),
			},
		},
		"service is not a provisioned service": {
			Resource: serviceBinding.
				SpecDie(func(d *dieservicebindingv1beta1.ServiceBindingSpecDie) {
					d.Service(serviceRef.DieRelease())
				}),
			GivenObjects: []client.Object{
				notProvisionedService,
			},
			ExpectResource: serviceBinding.
				SpecDie(func(d *dieservicebindingv1beta1.ServiceBindingSpecDie) {
					d.Service(serviceRef.DieRelease())
				}).
				StatusDie(func(d *dieservicebindingv1beta1.ServiceBindingStatusDie) {
					d.ConditionsDie(
						dieservicebindingv1beta1.ServiceBindingConditionReady.
							Reason("ServiceMissingBinding").
							Message("the service was found, but did not contain a binding secret"),
						dieservicebindingv1beta1.ServiceBindingConditionServiceAvailable.
							Reason("ServiceMissingBinding").
							Message("the service was found, but did not contain a binding secret"),
					)
				}),
			ExpectTracks: []rtesting.TrackRequest{
				rtesting.NewTrackRequest(provisionedService, serviceBinding, scheme),
			},
		},
		"service not found": {
			Resource: serviceBinding.
				SpecDie(func(d *dieservicebindingv1beta1.ServiceBindingSpecDie) {
					d.Service(serviceRef.DieRelease())
				}),
			WithReactors: []rtesting.ReactionFunc{
				rtesting.InduceFailure("get", "MyProvisionedService", rtesting.InduceFailureOpts{
					Error: apierrs.NewNotFound(schema.GroupResource{}, "my-service"),
				}),
			},
			ExpectResource: serviceBinding.
				SpecDie(func(d *dieservicebindingv1beta1.ServiceBindingSpecDie) {
					d.Service(serviceRef.DieRelease())
				}).
				StatusDie(func(d *dieservicebindingv1beta1.ServiceBindingStatusDie) {
					d.ConditionsDie(
						dieservicebindingv1beta1.ServiceBindingConditionReady.
							Reason("ServiceNotFound").
							Message("the service was not found"),
						dieservicebindingv1beta1.ServiceBindingConditionServiceAvailable.
							Reason("ServiceNotFound").
							Message("the service was not found"),
					)
				}),
			ExpectTracks: []rtesting.TrackRequest{
				rtesting.NewTrackRequest(provisionedService, serviceBinding, scheme),
			},
		},
		"service forbidden": {
			Resource: serviceBinding.
				SpecDie(func(d *dieservicebindingv1beta1.ServiceBindingSpecDie) {
					d.Service(serviceRef.DieRelease())
				}),
			WithReactors: []rtesting.ReactionFunc{
				rtesting.InduceFailure("get", "MyProvisionedService", rtesting.InduceFailureOpts{
					Error: apierrs.NewForbidden(schema.GroupResource{}, "my-service", fmt.Errorf("test forbidden")),
				}),
			},
			ExpectResource: serviceBinding.
				SpecDie(func(d *dieservicebindingv1beta1.ServiceBindingSpecDie) {
					d.Service(serviceRef.DieRelease())
				}).
				StatusDie(func(d *dieservicebindingv1beta1.ServiceBindingStatusDie) {
					d.ConditionsDie(
						dieservicebindingv1beta1.ServiceBindingConditionReady.
							False().
							Reason("ServiceForbidden").
							Message("the controller does not have permission to get the service"),
						dieservicebindingv1beta1.ServiceBindingConditionServiceAvailable.
							False().
							Reason("ServiceForbidden").
							Message("the controller does not have permission to get the service"),
					)
				}),
			ExpectTracks: []rtesting.TrackRequest{
				rtesting.NewTrackRequest(provisionedService, serviceBinding, scheme),
			},
		},
		"service generic get error": {
			Resource: serviceBinding.
				SpecDie(func(d *dieservicebindingv1beta1.ServiceBindingSpecDie) {
					d.Service(serviceRef.DieRelease())
				}),
			WithReactors: []rtesting.ReactionFunc{
				rtesting.InduceFailure("get", "MyProvisionedService"),
			},
			ShouldErr: true,
			ExpectTracks: []rtesting.TrackRequest{
				rtesting.NewTrackRequest(provisionedService, serviceBinding, scheme),
			},
		},
	}

	rts.Run(t, scheme, func(t *testing.T, tc *rtesting.SubReconcilerTestCase, c reconcilers.Config) reconcilers.SubReconciler {
		return controllers.ResolveBindingSecret()
	})
}

func TestResolveWorkload(t *testing.T) {
	namespace := "test-namespace"
	name := "my-binding"

	scheme := runtime.NewScheme()
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(servicebindingv1beta1.AddToScheme(scheme))

	serviceBinding := dieservicebindingv1beta1.ServiceBindingBlank.
		MetadataDie(func(d *diemetav1.ObjectMetaDie) {
			d.Namespace(namespace)
			d.Name(name)
		})

	workload := dieappsv1.DeploymentBlank.
		APIVersion("apps/v1").
		Kind("Deployment")
	workload1 := workload.
		MetadataDie(func(d *diemetav1.ObjectMetaDie) {
			d.Namespace(namespace)
			d.Name("my-workload-1")
			d.AddLabel("app", "my")
		})
	workload2 := workload.
		MetadataDie(func(d *diemetav1.ObjectMetaDie) {
			d.Namespace(namespace)
			d.Name("my-workload-2")
			d.AddLabel("app", "my")
		})
	workload3 := workload.
		MetadataDie(func(d *diemetav1.ObjectMetaDie) {
			d.Namespace(namespace)
			d.Name("not-my-workload")
			d.AddLabel("app", "not")
		})

	rts := rtesting.SubReconcilerTests{
		"resolve named workload": {
			Resource: serviceBinding.
				SpecDie(func(d *dieservicebindingv1beta1.ServiceBindingSpecDie) {
					d.WorkloadDie(func(d *dieservicebindingv1beta1.ServiceBindingWorkloadReferenceDie) {
						d.APIVersion("apps/v1")
						d.Kind("Deployment")
						d.Name("my-workload-1")
					})
				}),
			GivenObjects: []client.Object{
				workload1,
				workload2,
				workload3,
			},
			ExpectTracks: []rtesting.TrackRequest{
				rtesting.NewTrackRequest(workload1, serviceBinding, scheme),
			},
			ExpectStashedValues: map[reconcilers.StashKey]interface{}{
				controllers.WorkloadsStashKey: []runtime.Object{
					workload1.DieReleaseUnstructured(),
				},
			},
		},
		"resolve named workload not found": {
			Resource: serviceBinding.
				SpecDie(func(d *dieservicebindingv1beta1.ServiceBindingSpecDie) {
					d.WorkloadDie(func(d *dieservicebindingv1beta1.ServiceBindingWorkloadReferenceDie) {
						d.APIVersion("apps/v1")
						d.Kind("Deployment")
						d.Name("my-workload-1")
					})
				}),
			ExpectedResult: reconcile.Result{Requeue: true},
			ExpectResource: serviceBinding.
				SpecDie(func(d *dieservicebindingv1beta1.ServiceBindingSpecDie) {
					d.WorkloadDie(func(d *dieservicebindingv1beta1.ServiceBindingWorkloadReferenceDie) {
						d.APIVersion("apps/v1")
						d.Kind("Deployment")
						d.Name("my-workload-1")
					})
				}).
				StatusDie(func(d *dieservicebindingv1beta1.ServiceBindingStatusDie) {
					d.ConditionsDie(
						dieservicebindingv1beta1.ServiceBindingConditionReady.
							Reason("WorkloadNotFound").Message("the workload was not found"),
						dieservicebindingv1beta1.ServiceBindingConditionWorkloadProjected.
							Reason("WorkloadNotFound").Message("the workload was not found"),
					)
				}),
			ExpectTracks: []rtesting.TrackRequest{
				rtesting.NewTrackRequest(workload1, serviceBinding, scheme),
			},
		},
		"resolve named workload forbidden": {
			Resource: serviceBinding.
				SpecDie(func(d *dieservicebindingv1beta1.ServiceBindingSpecDie) {
					d.WorkloadDie(func(d *dieservicebindingv1beta1.ServiceBindingWorkloadReferenceDie) {
						d.APIVersion("apps/v1")
						d.Kind("Deployment")
						d.Name("my-workload-1")
					})
				}),
			GivenObjects: []client.Object{
				workload1,
				workload2,
				workload3,
			},
			WithReactors: []rtesting.ReactionFunc{
				rtesting.InduceFailure("get", "Deployment", rtesting.InduceFailureOpts{
					Error: apierrs.NewForbidden(schema.GroupResource{}, "my-workload-1", fmt.Errorf("test forbidden")),
				}),
			},
			ExpectedResult: reconcile.Result{Requeue: true},
			ExpectResource: serviceBinding.
				SpecDie(func(d *dieservicebindingv1beta1.ServiceBindingSpecDie) {
					d.WorkloadDie(func(d *dieservicebindingv1beta1.ServiceBindingWorkloadReferenceDie) {
						d.APIVersion("apps/v1")
						d.Kind("Deployment")
						d.Name("my-workload-1")
					})
				}).
				StatusDie(func(d *dieservicebindingv1beta1.ServiceBindingStatusDie) {
					d.ConditionsDie(
						dieservicebindingv1beta1.ServiceBindingConditionReady.
							False().
							Reason("WorkloadForbidden").
							Message("the controller does not have permission to get the workload"),
						dieservicebindingv1beta1.ServiceBindingConditionWorkloadProjected.
							False().
							Reason("WorkloadForbidden").
							Message("the controller does not have permission to get the workload"),
					)
				}),
			ExpectTracks: []rtesting.TrackRequest{
				rtesting.NewTrackRequest(workload1, serviceBinding, scheme),
			},
		},
		"resolve selected workload": {
			Resource: serviceBinding.
				SpecDie(func(d *dieservicebindingv1beta1.ServiceBindingSpecDie) {
					d.WorkloadDie(func(d *dieservicebindingv1beta1.ServiceBindingWorkloadReferenceDie) {
						d.APIVersion("apps/v1")
						d.Kind("Deployment")
						d.SelectorDie(func(d *diemetav1.LabelSelectorDie) {
							d.AddMatchLabel("app", "my")
						})
					})
				}),
			GivenObjects: []client.Object{
				workload1,
				workload2,
				workload3,
			},
			ExpectStashedValues: map[reconcilers.StashKey]interface{}{
				controllers.WorkloadsStashKey: []runtime.Object{
					workload1.DieReleaseUnstructured(),
					workload2.DieReleaseUnstructured(),
				},
			},
		},
		"resolve selected workload not found": {
			GivenObjects: []client.Object{},
			Resource: serviceBinding.
				SpecDie(func(d *dieservicebindingv1beta1.ServiceBindingSpecDie) {
					d.WorkloadDie(func(d *dieservicebindingv1beta1.ServiceBindingWorkloadReferenceDie) {
						d.APIVersion("apps/v1")
						d.Kind("Deployment")
						d.SelectorDie(func(d *diemetav1.LabelSelectorDie) {
							d.AddMatchLabel("app", "my")
						})
					})
				}),
			ExpectStashedValues: map[reconcilers.StashKey]interface{}{
				controllers.WorkloadsStashKey: []runtime.Object{},
			},
		},
		"resolve selected workload forbidden": {
			Resource: serviceBinding.
				SpecDie(func(d *dieservicebindingv1beta1.ServiceBindingSpecDie) {
					d.WorkloadDie(func(d *dieservicebindingv1beta1.ServiceBindingWorkloadReferenceDie) {
						d.APIVersion("apps/v1")
						d.Kind("Deployment")
						d.SelectorDie(func(d *diemetav1.LabelSelectorDie) {
							d.AddMatchLabel("app", "my")
						})
					})
				}),
			GivenObjects: []client.Object{
				workload1,
				workload2,
				workload3,
			},
			WithReactors: []rtesting.ReactionFunc{
				rtesting.InduceFailure("list", "DeploymentList", rtesting.InduceFailureOpts{
					Error: apierrs.NewForbidden(schema.GroupResource{}, "", fmt.Errorf("test forbidden")),
				}),
			},
			ExpectedResult: reconcile.Result{Requeue: true},
			ExpectResource: serviceBinding.
				SpecDie(func(d *dieservicebindingv1beta1.ServiceBindingSpecDie) {
					d.WorkloadDie(func(d *dieservicebindingv1beta1.ServiceBindingWorkloadReferenceDie) {
						d.APIVersion("apps/v1")
						d.Kind("Deployment")
						d.SelectorDie(func(d *diemetav1.LabelSelectorDie) {
							d.AddMatchLabel("app", "my")
						})
					})
				}).
				StatusDie(func(d *dieservicebindingv1beta1.ServiceBindingStatusDie) {
					d.ConditionsDie(
						dieservicebindingv1beta1.ServiceBindingConditionReady.
							False().
							Reason("WorkloadForbidden").
							Message("the controller does not have permission to list the workloads"),
						dieservicebindingv1beta1.ServiceBindingConditionWorkloadProjected.
							False().
							Reason("WorkloadForbidden").
							Message("the controller does not have permission to list the workloads"),
					)
				}),
		},
	}

	rts.Run(t, scheme, func(t *testing.T, tc *rtesting.SubReconcilerTestCase, c reconcilers.Config) reconcilers.SubReconciler {
		return controllers.ResolveWorkloads()
	})
}

func TestProjectBinding(t *testing.T) {
	namespace := "test-namespace"
	name := "my-binding"
	uid := types.UID("dde10100-d7b3-4cba-9430-51d60a8612a6")
	secretName := "my-secret"

	scheme := runtime.NewScheme()
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(servicebindingv1beta1.AddToScheme(scheme))

	now := metav1.Now().Rfc3339Copy()

	serviceBinding := dieservicebindingv1beta1.ServiceBindingBlank.
		MetadataDie(func(d *diemetav1.ObjectMetaDie) {
			d.Namespace(namespace)
			d.Name(name)
			d.UID(uid)
		}).
		SpecDie(func(d *dieservicebindingv1beta1.ServiceBindingSpecDie) {
			d.Name(name)
			d.WorkloadDie(func(d *dieservicebindingv1beta1.ServiceBindingWorkloadReferenceDie) {
				d.APIVersion("apps/v1")
				d.Kind("Deployment")
				d.Name("my-workload")
			})
		}).
		StatusDie(func(d *dieservicebindingv1beta1.ServiceBindingStatusDie) {
			d.BindingDie(func(d *dieservicebindingv1beta1.ServiceBindingSecretReferenceDie) {
				d.Name(secretName)
			})
		})

	workloadMapping := dieservicebindingv1beta1.ClusterWorkloadResourceMappingBlank.
		MetadataDie(func(d *diemetav1.ObjectMetaDie) {
			d.Name("deployments.apps")
		})

	workload := dieappsv1.DeploymentBlank.
		DieStamp(func(r *appsv1.Deployment) {
			r.APIVersion = "apps/v1"
			r.Kind = "Deployment"
		}).
		MetadataDie(func(d *diemetav1.ObjectMetaDie) {
			d.Namespace(namespace)
			d.Name("my-workload")
		}).
		SpecDie(func(d *dieappsv1.DeploymentSpecDie) {
			d.TemplateDie(func(d *diecorev1.PodTemplateSpecDie) {
				d.SpecDie(func(d *diecorev1.PodSpecDie) {
					d.ContainerDie("my-container", func(d *diecorev1.ContainerDie) {
						d.Image("scratch")
					})
				})
			})
		})
	projectedWorkload := workload.
		SpecDie(func(d *dieappsv1.DeploymentSpecDie) {
			d.TemplateDie(func(d *diecorev1.PodTemplateSpecDie) {
				d.MetadataDie(func(d *diemetav1.ObjectMetaDie) {
					d.AddAnnotation(fmt.Sprintf("projector.servicebinding.io/secret-%s", uid), secretName)
				})
				d.SpecDie(func(d *diecorev1.PodSpecDie) {
					d.ContainerDie("my-container", func(d *diecorev1.ContainerDie) {
						d.EnvDie("SERVICE_BINDING_ROOT", func(d *diecorev1.EnvVarDie) {
							d.Value("/bindings")
						})
						d.VolumeMountDie(fmt.Sprintf("servicebinding-%s", uid), func(d *diecorev1.VolumeMountDie) {
							d.MountPath(fmt.Sprintf("/bindings/%s", name))
							d.ReadOnly(true)
						})
					})
					d.VolumeDie(fmt.Sprintf("servicebinding-%s", uid), func(d *diecorev1.VolumeDie) {
						d.ProjectedDie(func(d *diecorev1.ProjectedVolumeSourceDie) {
							d.SourcesDie(
								diecorev1.VolumeProjectionBlank.
									SecretDie(func(d *diecorev1.SecretProjectionDie) {
										d.Name(secretName)
									}),
							)
						})
					})
				})
			})
		})
	// TODO find a better way to avoid empty vs nil objects that are lost in the unstructured conversion
	unprojectedWorkload := workload.
		SpecDie(func(d *dieappsv1.DeploymentSpecDie) {
			d.TemplateDie(func(d *diecorev1.PodTemplateSpecDie) {
				d.SpecDie(func(d *diecorev1.PodSpecDie) {
					d.ContainerDie("my-container", func(d *diecorev1.ContainerDie) {
						d.EnvDie("SERVICE_BINDING_ROOT", func(d *diecorev1.EnvVarDie) {
							d.Value("/bindings")
						})
					})
				})
			})
		}).DieReleaseUnstructured()
	unstructured.SetNestedMap(unprojectedWorkload.UnstructuredContent(), map[string]interface{}{}, "spec", "template", "metadata", "annotations")
	containers, _, _ := unstructured.NestedSlice(unprojectedWorkload.UnstructuredContent(), "spec", "template", "spec", "containers")
	unstructured.SetNestedSlice(containers[0].(map[string]interface{}), []interface{}{}, "volumeMounts")
	unstructured.SetNestedSlice(unprojectedWorkload.UnstructuredContent(), containers, "spec", "template", "spec", "containers")
	unstructured.SetNestedSlice(unprojectedWorkload.UnstructuredContent(), []interface{}{}, "spec", "template", "spec", "volumes")

	rts := rtesting.SubReconcilerTests{
		"project workload": {
			Resource: serviceBinding.
				SpecDie(func(d *dieservicebindingv1beta1.ServiceBindingSpecDie) {
					d.WorkloadDie(func(d *dieservicebindingv1beta1.ServiceBindingWorkloadReferenceDie) {
						d.APIVersion("apps/v1")
						d.Kind("Deployment")
						d.Name("my-workload-1")
					})
				}),
			GivenStashedValues: map[reconcilers.StashKey]interface{}{
				controllers.WorkloadsStashKey: []runtime.Object{
					workload.DieReleaseUnstructured(),
				},
			},
			ExpectStashedValues: map[reconcilers.StashKey]interface{}{
				controllers.ProjectedWorkloadsStashKey: []runtime.Object{
					projectedWorkload.DieReleaseUnstructured(),
				},
			},
			ExpectTracks: []rtesting.TrackRequest{
				rtesting.NewTrackRequest(workloadMapping, serviceBinding, scheme),
			},
		},
		"unproject terminating workload": {
			Resource: serviceBinding.
				MetadataDie(func(d *diemetav1.ObjectMetaDie) {
					d.DeletionTimestamp(&now)
				}).
				SpecDie(func(d *dieservicebindingv1beta1.ServiceBindingSpecDie) {
					d.WorkloadDie(func(d *dieservicebindingv1beta1.ServiceBindingWorkloadReferenceDie) {
						d.APIVersion("apps/v1")
						d.Kind("Deployment")
						d.Name("my-workload-1")
					})
				}),
			GivenStashedValues: map[reconcilers.StashKey]interface{}{
				controllers.WorkloadsStashKey: []runtime.Object{
					projectedWorkload.DieReleaseUnstructured(),
				},
			},
			ExpectStashedValues: map[reconcilers.StashKey]interface{}{
				controllers.ProjectedWorkloadsStashKey: []runtime.Object{
					unprojectedWorkload,
				},
			},
			ExpectTracks: []rtesting.TrackRequest{
				rtesting.NewTrackRequest(workloadMapping, serviceBinding, scheme),
			},
		},
	}

	rts.Run(t, scheme, func(t *testing.T, tc *rtesting.SubReconcilerTestCase, c reconcilers.Config) reconcilers.SubReconciler {
		restMapper := c.RESTMapper().(*meta.DefaultRESTMapper)
		restMapper.Add(schema.GroupVersionKind{Group: "apps", Version: "v1", Kind: "Deployment"}, meta.RESTScopeNamespace)
		return controllers.ProjectBinding()
	})
}

func TestPatchWorkloads(t *testing.T) {
	namespace := "test-namespace"
	name := "my-binding"
	uid := types.UID("dde10100-d7b3-4cba-9430-51d60a8612a6")

	now := metav1.Now().Rfc3339Copy()
	scheme := runtime.NewScheme()
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(servicebindingv1beta1.AddToScheme(scheme))

	serviceBinding := dieservicebindingv1beta1.ServiceBindingBlank.
		MetadataDie(func(d *diemetav1.ObjectMetaDie) {
			d.Namespace(namespace)
			d.Name(name)
		}).
		StatusDie(func(d *dieservicebindingv1beta1.ServiceBindingStatusDie) {
			d.ConditionsDie(
				dieservicebindingv1beta1.ServiceBindingConditionReady,
				dieservicebindingv1beta1.ServiceBindingConditionServiceAvailable.True().Reason("ResolvedBindingSecret"),
				dieservicebindingv1beta1.ServiceBindingConditionWorkloadProjected,
			)
		})

	workload := dieappsv1.DeploymentBlank.
		DieStamp(func(r *appsv1.Deployment) {
			r.APIVersion = "apps/v1"
			r.Kind = "Deployment"
		}).
		MetadataDie(func(d *diemetav1.ObjectMetaDie) {
			d.Namespace(namespace)
			d.Name("my-workload")
			d.CreationTimestamp(now)
			d.UID(uid)
		}).
		SpecDie(func(d *dieappsv1.DeploymentSpecDie) {
			d.TemplateDie(func(d *diecorev1.PodTemplateSpecDie) {
				d.SpecDie(func(d *diecorev1.PodSpecDie) {
					d.ContainerDie("my-container", func(d *diecorev1.ContainerDie) {
						d.Image("scratch")
					})
				})
			})
		})

	rts := rtesting.SubReconcilerTests{
		"in sync": {
			Resource: serviceBinding.
				StatusDie(func(d *dieservicebindingv1beta1.ServiceBindingStatusDie) {
					d.ConditionsDie(
						dieservicebindingv1beta1.ServiceBindingConditionReady.True().Reason("ServiceBound"),
						dieservicebindingv1beta1.ServiceBindingConditionServiceAvailable.True().Reason("ResolvedBindingSecret"),
						dieservicebindingv1beta1.ServiceBindingConditionWorkloadProjected.True().Reason("WorkloadProjected"),
					)
				}),
			GivenObjects: []client.Object{
				workload,
			},
			GivenStashedValues: map[reconcilers.StashKey]interface{}{
				controllers.WorkloadsStashKey: []runtime.Object{
					workload.DieReleaseUnstructured(),
				},
				controllers.ProjectedWorkloadsStashKey: []runtime.Object{
					workload.DieReleaseUnstructured(),
				},
			},
		},
		"update workload": {
			Resource: serviceBinding,
			GivenObjects: []client.Object{
				workload,
			},
			GivenStashedValues: map[reconcilers.StashKey]interface{}{
				controllers.WorkloadsStashKey: []runtime.Object{
					workload.DieReleaseUnstructured(),
				},
				controllers.ProjectedWorkloadsStashKey: []runtime.Object{
					workload.
						SpecDie(func(d *dieappsv1.DeploymentSpecDie) {
							// not something a binding would ever project, but good enough for a test
							d.Paused(true)
						}).
						DieReleaseUnstructured(),
				},
			},
			ExpectResource: serviceBinding.
				StatusDie(func(d *dieservicebindingv1beta1.ServiceBindingStatusDie) {
					d.ConditionsDie(
						dieservicebindingv1beta1.ServiceBindingConditionReady.True().Reason("ServiceBound"),
						dieservicebindingv1beta1.ServiceBindingConditionServiceAvailable.True().Reason("ResolvedBindingSecret"),
						dieservicebindingv1beta1.ServiceBindingConditionWorkloadProjected.True().Reason("WorkloadProjected"),
					)
				}),
			ExpectEvents: []rtesting.Event{
				rtesting.NewEvent(serviceBinding, scheme, corev1.EventTypeNormal, "Updated", "Updated Deployment %q", "my-workload"),
			},
			ExpectUpdates: []client.Object{
				workload.
					SpecDie(func(d *dieappsv1.DeploymentSpecDie) {
						// not something a binding would ever project, but good enough for a test
						d.Paused(true)
					}).DieReleaseUnstructured().(client.Object),
			},
		},
		"update workload ignoring not found errors": {
			Resource: serviceBinding,
			GivenStashedValues: map[reconcilers.StashKey]interface{}{
				controllers.WorkloadsStashKey: []runtime.Object{
					workload.DieReleaseUnstructured(),
				},
				controllers.ProjectedWorkloadsStashKey: []runtime.Object{
					workload.
						SpecDie(func(d *dieappsv1.DeploymentSpecDie) {
							// not something a binding would ever project, but good enough for a test
							d.Paused(true)
						}).
						DieReleaseUnstructured(),
				},
			},
			ExpectResource: serviceBinding.
				StatusDie(func(d *dieservicebindingv1beta1.ServiceBindingStatusDie) {
					d.ConditionsDie(
						dieservicebindingv1beta1.ServiceBindingConditionReady.True().Reason("ServiceBound"),
						dieservicebindingv1beta1.ServiceBindingConditionServiceAvailable.True().Reason("ResolvedBindingSecret"),
						dieservicebindingv1beta1.ServiceBindingConditionWorkloadProjected.True().Reason("WorkloadProjected"),
					)
				}),
			ExpectEvents: []rtesting.Event{
				rtesting.NewEvent(serviceBinding, scheme, corev1.EventTypeWarning, "UpdateFailed", "Failed to update Deployment %q: deployments.apps %q not found", "my-workload", "my-workload"),
			},
			ExpectUpdates: []client.Object{
				workload.
					SpecDie(func(d *dieappsv1.DeploymentSpecDie) {
						// not something a binding would ever project, but good enough for a test
						d.Paused(true)
					}).DieReleaseUnstructured().(client.Object),
			},
		},
		"update workload forbidden": {
			Resource: serviceBinding,
			GivenObjects: []client.Object{
				workload,
			},
			GivenStashedValues: map[reconcilers.StashKey]interface{}{
				controllers.WorkloadsStashKey: []runtime.Object{
					workload.DieReleaseUnstructured(),
				},
				controllers.ProjectedWorkloadsStashKey: []runtime.Object{
					workload.
						SpecDie(func(d *dieappsv1.DeploymentSpecDie) {
							// not something a binding would ever project, but good enough for a test
							d.Paused(true)
						}).
						DieReleaseUnstructured(),
				},
			},
			WithReactors: []rtesting.ReactionFunc{
				rtesting.InduceFailure("update", "Deployment", rtesting.InduceFailureOpts{
					Error: apierrs.NewForbidden(schema.GroupResource{}, "", fmt.Errorf("test forbidden")),
				}),
			},
			ExpectResource: serviceBinding.
				StatusDie(func(d *dieservicebindingv1beta1.ServiceBindingStatusDie) {
					d.ConditionsDie(
						dieservicebindingv1beta1.ServiceBindingConditionReady.
							False().
							Reason("WorkloadForbidden").
							Message("the controller does not have permission to update the workloads"),
						dieservicebindingv1beta1.ServiceBindingConditionServiceAvailable.
							True().
							Reason("ResolvedBindingSecret"),
						dieservicebindingv1beta1.ServiceBindingConditionWorkloadProjected.
							False().
							Reason("WorkloadForbidden").
							Message("the controller does not have permission to update the workloads"),
					)
				}),
			ExpectEvents: []rtesting.Event{
				rtesting.NewEvent(serviceBinding, scheme, corev1.EventTypeWarning, "UpdateFailed", "Failed to update Deployment %q: forbidden: test forbidden", "my-workload"),
			},
			ExpectUpdates: []client.Object{
				workload.
					SpecDie(func(d *dieappsv1.DeploymentSpecDie) {
						// not something a binding would ever project, but good enough for a test
						d.Paused(true)
					}).DieReleaseUnstructured().(client.Object),
			},
		},
		"require same number of workloads and projected workloads": {
			Resource: serviceBinding,
			GivenObjects: []client.Object{
				workload,
			},
			GivenStashedValues: map[reconcilers.StashKey]interface{}{
				controllers.WorkloadsStashKey: []runtime.Object{
					workload.DieReleaseUnstructured(),
					workload.DieReleaseUnstructured(),
				},
				controllers.ProjectedWorkloadsStashKey: []runtime.Object{
					workload.
						SpecDie(func(d *dieappsv1.DeploymentSpecDie) {
							// not something a binding would ever project, but good enough for a test
							d.Paused(true)
						}).
						DieReleaseUnstructured(),
				},
			},
			ShouldPanic: true,
		},
		"panic if workload and projected workload are not the same uid": {
			Resource: serviceBinding,
			GivenStashedValues: map[reconcilers.StashKey]interface{}{
				controllers.WorkloadsStashKey: []runtime.Object{
					workload.DieReleaseUnstructured(),
				},
				controllers.ProjectedWorkloadsStashKey: []runtime.Object{
					workload.
						MetadataDie(func(d *diemetav1.ObjectMetaDie) {
							d.UID("")
						}).
						DieReleaseUnstructured(),
				},
			},
			ShouldPanic: true,
		},
		"panic if workload and projected workload are not the same resource version": {
			Resource: serviceBinding,
			GivenStashedValues: map[reconcilers.StashKey]interface{}{
				controllers.WorkloadsStashKey: []runtime.Object{
					workload.DieReleaseUnstructured(),
				},
				controllers.ProjectedWorkloadsStashKey: []runtime.Object{
					workload.
						MetadataDie(func(d *diemetav1.ObjectMetaDie) {
							d.ResourceVersion("1000")
						}).
						DieReleaseUnstructured(),
				},
			},
			ShouldPanic: true,
		},
	}

	rts.Run(t, scheme, func(t *testing.T, tc *rtesting.SubReconcilerTestCase, c reconcilers.Config) reconcilers.SubReconciler {
		return controllers.PatchWorkloads()
	})
}

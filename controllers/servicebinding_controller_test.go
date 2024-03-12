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
	"errors"
	"fmt"
	"testing"

	dieappsv1 "dies.dev/apis/apps/v1"
	diecorev1 "dies.dev/apis/core/v1"
	diemetav1 "dies.dev/apis/meta/v1"
	"github.com/vmware-labs/reconciler-runtime/reconcilers"
	rtesting "github.com/vmware-labs/reconciler-runtime/testing"
	"github.com/vmware-labs/reconciler-runtime/tracker"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/uuid"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	servicebindingv1 "github.com/servicebinding/runtime/apis/v1"
	"github.com/servicebinding/runtime/controllers"
	dieservicebindingv1 "github.com/servicebinding/runtime/dies/v1"
	"github.com/servicebinding/runtime/lifecycle"
)

func TestServiceBindingReconciler(t *testing.T) {
	namespace := "test-namespace"
	name := "my-binding"
	uid := types.UID("dde10100-d7b3-4cba-9430-51d60a8612a6")
	secretName := "my-secret"
	request := reconcilers.Request{NamespacedName: types.NamespacedName{Namespace: namespace, Name: name}}

	podSpecableMapping := `{"versions":[{"version":"*","annotations":".spec.template.metadata.annotations","containers":[{"path":".spec.template.spec.initContainers[*]","name":".name","env":".env","volumeMounts":".volumeMounts"},{"path":".spec.template.spec.containers[*]","name":".name","env":".env","volumeMounts":".volumeMounts"}],"volumes":".spec.template.spec.volumes"}]}`

	scheme := runtime.NewScheme()
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(servicebindingv1.AddToScheme(scheme))

	now := metav1.Now().Rfc3339Copy()

	serviceBinding := dieservicebindingv1.ServiceBindingBlank.
		MetadataDie(func(d *diemetav1.ObjectMetaDie) {
			d.Namespace(namespace)
			d.Name(name)
			d.UID(uid)
		}).
		SpecDie(func(d *dieservicebindingv1.ServiceBindingSpecDie) {
			d.ServiceDie(func(d *dieservicebindingv1.ServiceBindingServiceReferenceDie) {
				d.APIVersion("v1")
				d.Kind("Secret")
				d.Name(secretName)
			})
			d.WorkloadDie(func(d *dieservicebindingv1.ServiceBindingWorkloadReferenceDie) {
				d.APIVersion("apps/v1")
				d.Kind("Deployment")
				d.Name("my-workload")
			})
		})

	workloadMapping := dieservicebindingv1.ClusterWorkloadResourceMappingBlank.
		MetadataDie(func(d *diemetav1.ObjectMetaDie) {
			d.Name("deployments.apps")
		})

	workload := dieappsv1.DeploymentBlank.
		APIVersion("apps/v1").
		Kind("Deployment").
		MetadataDie(func(d *diemetav1.ObjectMetaDie) {
			d.Namespace(namespace)
			d.Name("my-workload")
			d.UID(uuid.NewUUID())
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
		MetadataDie(func(d *diemetav1.ObjectMetaDie) {
			d.AddAnnotation(fmt.Sprintf("projector.servicebinding.io/mapping-%s", uid), podSpecableMapping)
		}).
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
	unstructured.SetNestedMap(unprojectedWorkload.UnstructuredContent(), map[string]interface{}{}, "metadata", "annotations")
	unstructured.SetNestedMap(unprojectedWorkload.UnstructuredContent(), map[string]interface{}{}, "spec", "template", "metadata", "annotations")
	containers, _, _ := unstructured.NestedSlice(unprojectedWorkload.UnstructuredContent(), "spec", "template", "spec", "containers")
	unstructured.SetNestedSlice(containers[0].(map[string]interface{}), []interface{}{}, "volumeMounts")
	unstructured.SetNestedSlice(unprojectedWorkload.UnstructuredContent(), containers, "spec", "template", "spec", "containers")
	unstructured.SetNestedSlice(unprojectedWorkload.UnstructuredContent(), []interface{}{}, "spec", "template", "spec", "volumes")

	newWorkloadUID := uuid.NewUUID()

	rts := rtesting.ReconcilerTests{
		"in sync": {
			Request: request,
			StatusSubResourceTypes: []client.Object{
				&servicebindingv1.ServiceBinding{},
			},
			GivenObjects: []client.Object{
				serviceBinding.
					MetadataDie(func(d *diemetav1.ObjectMetaDie) {
						d.Finalizers("servicebinding.io/finalizer")
					}).
					StatusDie(func(d *dieservicebindingv1.ServiceBindingStatusDie) {
						d.ConditionsDie(
							dieservicebindingv1.ServiceBindingConditionReady.True().Reason("ServiceBound"),
							dieservicebindingv1.ServiceBindingConditionServiceAvailable.True().Reason("ResolvedBindingSecret"),
							dieservicebindingv1.ServiceBindingConditionWorkloadProjected.True().Reason("WorkloadProjected"),
						)
						d.BindingDie(func(d *dieservicebindingv1.ServiceBindingSecretReferenceDie) {
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
		"newly created, resolves secret": {
			Request: request,
			StatusSubResourceTypes: []client.Object{
				&servicebindingv1.ServiceBinding{},
			},
			GivenObjects: []client.Object{
				serviceBinding,
				workload,
			},
			ExpectEvents: []rtesting.Event{
				rtesting.NewEvent(serviceBinding, scheme, corev1.EventTypeNormal, "FinalizerPatched", "Patched finalizer %q", "servicebinding.io/finalizer"),
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
			ExpectStatusUpdates: []client.Object{
				serviceBinding.
					StatusDie(func(d *dieservicebindingv1.ServiceBindingStatusDie) {
						d.ConditionsDie(
							dieservicebindingv1.ServiceBindingConditionReady.Unknown().Reason("Initializing"),
							dieservicebindingv1.ServiceBindingConditionServiceAvailable.True().Reason("ResolvedBindingSecret"),
							dieservicebindingv1.ServiceBindingConditionWorkloadProjected.Unknown().Reason("Initializing"),
						)
						d.BindingDie(func(d *dieservicebindingv1.ServiceBindingSecretReferenceDie) {
							d.Name(secretName)
						})
					}),
			},
		},
		"has resolved secret, project into workload": {
			Request: request,
			StatusSubResourceTypes: []client.Object{
				&servicebindingv1.ServiceBinding{},
			},
			GivenObjects: []client.Object{
				serviceBinding.
					MetadataDie(func(d *diemetav1.ObjectMetaDie) {
						d.Finalizers("servicebinding.io/finalizer")
					}).
					StatusDie(func(d *dieservicebindingv1.ServiceBindingStatusDie) {
						d.BindingDie(func(d *dieservicebindingv1.ServiceBindingSecretReferenceDie) {
							d.Name(secretName)
						})
					}),
				workload,
			},
			ExpectTracks: []rtesting.TrackRequest{
				rtesting.NewTrackRequest(projectedWorkload, serviceBinding, scheme),
				rtesting.NewTrackRequest(workloadMapping, serviceBinding, scheme),
			},
			ExpectEvents: []rtesting.Event{
				rtesting.NewEvent(serviceBinding, scheme, corev1.EventTypeNormal, "Updated", "Updated Deployment %q", "my-workload"),
				rtesting.NewEvent(serviceBinding, scheme, corev1.EventTypeNormal, "StatusUpdated", "Updated status"),
			},
			ExpectUpdates: []client.Object{
				projectedWorkload.DieReleaseUnstructured(),
			},
			ExpectStatusUpdates: []client.Object{
				serviceBinding.
					StatusDie(func(d *dieservicebindingv1.ServiceBindingStatusDie) {
						d.ConditionsDie(
							dieservicebindingv1.ServiceBindingConditionReady.True().Reason("ServiceBound"),
							dieservicebindingv1.ServiceBindingConditionServiceAvailable.True().Reason("ResolvedBindingSecret"),
							dieservicebindingv1.ServiceBindingConditionWorkloadProjected.True().Reason("WorkloadProjected"),
						)
						d.BindingDie(func(d *dieservicebindingv1.ServiceBindingSecretReferenceDie) {
							d.Name(secretName)
						})
					}),
			},
		},
		"switch bound workload": {
			Request: request,
			StatusSubResourceTypes: []client.Object{
				&servicebindingv1.ServiceBinding{},
			},
			GivenObjects: []client.Object{
				serviceBinding.
					MetadataDie(func(d *diemetav1.ObjectMetaDie) {
						d.Finalizers("servicebinding.io/finalizer")
					}).
					SpecDie(func(d *dieservicebindingv1.ServiceBindingSpecDie) {
						d.WorkloadDie(func(d *dieservicebindingv1.ServiceBindingWorkloadReferenceDie) {
							d.Name("new-workload")
						})
					}).
					StatusDie(func(d *dieservicebindingv1.ServiceBindingStatusDie) {
						d.ConditionsDie(
							dieservicebindingv1.ServiceBindingConditionReady.True().Reason("ServiceBound"),
							dieservicebindingv1.ServiceBindingConditionServiceAvailable.True().Reason("ResolvedBindingSecret"),
							dieservicebindingv1.ServiceBindingConditionWorkloadProjected.True().Reason("WorkloadProjected"),
						)
						d.BindingDie(func(d *dieservicebindingv1.ServiceBindingSecretReferenceDie) {
							d.Name(secretName)
						})
					}),
				projectedWorkload,
				workload.
					MetadataDie(func(d *diemetav1.ObjectMetaDie) {
						d.Name("new-workload")
						d.UID(newWorkloadUID)
					}),
			},
			ExpectTracks: []rtesting.TrackRequest{
				rtesting.NewTrackRequest(workload.MetadataDie(func(d *diemetav1.ObjectMetaDie) { d.Name("new-workload") }), serviceBinding, scheme),
				rtesting.NewTrackRequest(workloadMapping, serviceBinding, scheme),
				rtesting.NewTrackRequest(workloadMapping, serviceBinding, scheme),
			},
			ExpectEvents: []rtesting.Event{
				rtesting.NewEvent(serviceBinding, scheme, corev1.EventTypeNormal, "Updated", "Updated Deployment %q", workload.GetName()),
				rtesting.NewEvent(serviceBinding, scheme, corev1.EventTypeNormal, "Updated", "Updated Deployment %q", "new-workload"),
			},
			ExpectUpdates: []client.Object{
				// unproject my-workload
				unprojectedWorkload,
				// project new-workload
				projectedWorkload.
					MetadataDie(func(d *diemetav1.ObjectMetaDie) {
						d.Name("new-workload")
						d.UID(newWorkloadUID)
					}).
					DieReleaseUnstructured(),
			},
		},
		"terminating": {
			Request: request,
			StatusSubResourceTypes: []client.Object{
				&servicebindingv1.ServiceBinding{},
			},
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
				unprojectedWorkload,
			},
		},
	}

	rts.Run(t, scheme, func(t *testing.T, tc *rtesting.ReconcilerTestCase, c reconcilers.Config) reconcile.Reconciler {
		restMapper := c.RESTMapper().(*meta.DefaultRESTMapper)
		restMapper.Add(schema.GroupVersionKind{Group: "apps", Version: "v1", Kind: "Deployment"}, meta.RESTScopeNamespace)
		return controllers.ServiceBindingReconciler(c, lifecycle.ServiceBindingHooks{})
	})
}

func TestResolveBindingSecret(t *testing.T) {
	namespace := "test-namespace"
	name := "my-binding"

	scheme := runtime.NewScheme()
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(servicebindingv1.AddToScheme(scheme))

	serviceBinding := dieservicebindingv1.ServiceBindingBlank.
		MetadataDie(func(d *diemetav1.ObjectMetaDie) {
			d.Namespace(namespace)
			d.Name(name)
		})

	secretName := "my-secret"
	directSecretRef := dieservicebindingv1.ServiceBindingServiceReferenceBlank.
		APIVersion("v1").
		Kind("Secret").
		Name(secretName)
	serviceRef := dieservicebindingv1.ServiceBindingServiceReferenceBlank.
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

	rts := rtesting.SubReconcilerTests[*servicebindingv1.ServiceBinding]{
		"in sync": {
			Resource: serviceBinding.
				SpecDie(func(d *dieservicebindingv1.ServiceBindingSpecDie) {
					d.Service(directSecretRef.DieRelease())
				}).
				StatusDie(func(d *dieservicebindingv1.ServiceBindingStatusDie) {
					d.BindingDie(func(d *dieservicebindingv1.ServiceBindingSecretReferenceDie) {
						d.Name(secretName)
					})
					d.ConditionsDie(
						dieservicebindingv1.ServiceBindingConditionServiceAvailable.
							True().Reason("ResolvedBindingSecret"),
					)
				}).
				DieReleasePtr(),
		},
		"resolve direct secret": {
			Resource: serviceBinding.
				SpecDie(func(d *dieservicebindingv1.ServiceBindingSpecDie) {
					d.Service(directSecretRef.DieRelease())
				}).
				DieReleasePtr(),
			ExpectResource: serviceBinding.
				SpecDie(func(d *dieservicebindingv1.ServiceBindingSpecDie) {
					d.Service(directSecretRef.DieRelease())
				}).
				StatusDie(func(d *dieservicebindingv1.ServiceBindingStatusDie) {
					d.BindingDie(func(d *dieservicebindingv1.ServiceBindingSecretReferenceDie) {
						d.Name(secretName)
					})
					d.ConditionsDie(
						dieservicebindingv1.ServiceBindingConditionServiceAvailable.
							True().Reason("ResolvedBindingSecret"),
					)
				}).
				DieReleasePtr(),
			ShouldErr: true,
			Verify: func(t *testing.T, result ctrl.Result, err error) {
				if !errors.Is(err, reconcilers.HaltSubReconcilers) {
					t.Errorf("expected err to be of type reconcilers.HaltSubReconcilers")
				}
			},
		},
		"service is a provisioned service": {
			Resource: serviceBinding.
				SpecDie(func(d *dieservicebindingv1.ServiceBindingSpecDie) {
					d.Service(serviceRef.DieRelease())
				}).
				DieReleasePtr(),
			GivenObjects: []client.Object{
				provisionedService,
			},
			ExpectResource: serviceBinding.
				SpecDie(func(d *dieservicebindingv1.ServiceBindingSpecDie) {
					d.Service(serviceRef.DieRelease())
				}).
				StatusDie(func(d *dieservicebindingv1.ServiceBindingStatusDie) {
					d.BindingDie(func(d *dieservicebindingv1.ServiceBindingSecretReferenceDie) {
						d.Name(secretName)
					})
					d.ConditionsDie(
						dieservicebindingv1.ServiceBindingConditionServiceAvailable.
							True().Reason("ResolvedBindingSecret"),
					)
				}).
				DieReleasePtr(),
			ExpectTracks: []rtesting.TrackRequest{
				rtesting.NewTrackRequest(provisionedService, serviceBinding, scheme),
			},
			ShouldErr: true,
			Verify: func(t *testing.T, result ctrl.Result, err error) {
				if !errors.Is(err, reconcilers.HaltSubReconcilers) {
					t.Errorf("expected err to be of type reconcilers.HaltSubReconcilers")
				}
			},
		},
		"service is not a provisioned service": {
			Resource: serviceBinding.
				SpecDie(func(d *dieservicebindingv1.ServiceBindingSpecDie) {
					d.Service(serviceRef.DieRelease())
				}).
				DieReleasePtr(),
			GivenObjects: []client.Object{
				notProvisionedService,
			},
			ExpectResource: serviceBinding.
				SpecDie(func(d *dieservicebindingv1.ServiceBindingSpecDie) {
					d.Service(serviceRef.DieRelease())
				}).
				StatusDie(func(d *dieservicebindingv1.ServiceBindingStatusDie) {
					d.ConditionsDie(
						dieservicebindingv1.ServiceBindingConditionReady.
							Reason("ServiceMissingBinding").
							Message("the service was found, but did not contain a binding secret"),
						dieservicebindingv1.ServiceBindingConditionServiceAvailable.
							Reason("ServiceMissingBinding").
							Message("the service was found, but did not contain a binding secret"),
					)
				}).
				DieReleasePtr(),
			ExpectTracks: []rtesting.TrackRequest{
				rtesting.NewTrackRequest(provisionedService, serviceBinding, scheme),
			},
		},
		"service not found": {
			Resource: serviceBinding.
				SpecDie(func(d *dieservicebindingv1.ServiceBindingSpecDie) {
					d.Service(serviceRef.DieRelease())
				}).
				DieReleasePtr(),
			WithReactors: []rtesting.ReactionFunc{
				rtesting.InduceFailure("get", "MyProvisionedService", rtesting.InduceFailureOpts{
					Error: apierrs.NewNotFound(schema.GroupResource{}, "my-service"),
				}),
			},
			ExpectResource: serviceBinding.
				SpecDie(func(d *dieservicebindingv1.ServiceBindingSpecDie) {
					d.Service(serviceRef.DieRelease())
				}).
				StatusDie(func(d *dieservicebindingv1.ServiceBindingStatusDie) {
					d.ConditionsDie(
						dieservicebindingv1.ServiceBindingConditionReady.
							Reason("ServiceNotFound").
							Message("the service was not found"),
						dieservicebindingv1.ServiceBindingConditionServiceAvailable.
							Reason("ServiceNotFound").
							Message("the service was not found"),
					)
				}).
				DieReleasePtr(),
			ExpectTracks: []rtesting.TrackRequest{
				rtesting.NewTrackRequest(provisionedService, serviceBinding, scheme),
			},
		},
		"service forbidden": {
			Resource: serviceBinding.
				SpecDie(func(d *dieservicebindingv1.ServiceBindingSpecDie) {
					d.Service(serviceRef.DieRelease())
				}).
				DieReleasePtr(),
			WithReactors: []rtesting.ReactionFunc{
				rtesting.InduceFailure("get", "MyProvisionedService", rtesting.InduceFailureOpts{
					Error: apierrs.NewForbidden(schema.GroupResource{}, "my-service", fmt.Errorf("test forbidden")),
				}),
			},
			ExpectResource: serviceBinding.
				SpecDie(func(d *dieservicebindingv1.ServiceBindingSpecDie) {
					d.Service(serviceRef.DieRelease())
				}).
				StatusDie(func(d *dieservicebindingv1.ServiceBindingStatusDie) {
					d.ConditionsDie(
						dieservicebindingv1.ServiceBindingConditionReady.
							False().
							Reason("ServiceForbidden").
							Message("the controller does not have permission to get the service"),
						dieservicebindingv1.ServiceBindingConditionServiceAvailable.
							False().
							Reason("ServiceForbidden").
							Message("the controller does not have permission to get the service"),
					)
				}).
				DieReleasePtr(),
			ExpectTracks: []rtesting.TrackRequest{
				rtesting.NewTrackRequest(provisionedService, serviceBinding, scheme),
			},
		},
		"service generic get error": {
			Resource: serviceBinding.
				SpecDie(func(d *dieservicebindingv1.ServiceBindingSpecDie) {
					d.Service(serviceRef.DieRelease())
				}).
				DieReleasePtr(),
			WithReactors: []rtesting.ReactionFunc{
				rtesting.InduceFailure("get", "MyProvisionedService"),
			},
			ShouldErr: true,
			ExpectTracks: []rtesting.TrackRequest{
				rtesting.NewTrackRequest(provisionedService, serviceBinding, scheme),
			},
		},
	}

	rts.Run(t, scheme, func(t *testing.T, tc *rtesting.SubReconcilerTestCase[*servicebindingv1.ServiceBinding], c reconcilers.Config) reconcilers.SubReconciler[*servicebindingv1.ServiceBinding] {
		return controllers.ResolveBindingSecret(lifecycle.ServiceBindingHooks{})
	})
}

func TestResolveWorkload(t *testing.T) {
	namespace := "test-namespace"
	name := "my-binding"

	scheme := runtime.NewScheme()
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(servicebindingv1.AddToScheme(scheme))

	serviceBinding := dieservicebindingv1.ServiceBindingBlank.
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

	rts := rtesting.SubReconcilerTests[*servicebindingv1.ServiceBinding]{
		"resolve named workload": {
			Resource: serviceBinding.
				SpecDie(func(d *dieservicebindingv1.ServiceBindingSpecDie) {
					d.WorkloadDie(func(d *dieservicebindingv1.ServiceBindingWorkloadReferenceDie) {
						d.APIVersion("apps/v1")
						d.Kind("Deployment")
						d.Name("my-workload-1")
					})
				}).
				DieReleasePtr(),
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
				SpecDie(func(d *dieservicebindingv1.ServiceBindingSpecDie) {
					d.WorkloadDie(func(d *dieservicebindingv1.ServiceBindingWorkloadReferenceDie) {
						d.APIVersion("apps/v1")
						d.Kind("Deployment")
						d.Name("my-workload-1")
					})
				}).
				DieReleasePtr(),
			ExpectResource: serviceBinding.
				SpecDie(func(d *dieservicebindingv1.ServiceBindingSpecDie) {
					d.WorkloadDie(func(d *dieservicebindingv1.ServiceBindingWorkloadReferenceDie) {
						d.APIVersion("apps/v1")
						d.Kind("Deployment")
						d.Name("my-workload-1")
					})
				}).
				StatusDie(func(d *dieservicebindingv1.ServiceBindingStatusDie) {
					d.ConditionsDie(
						dieservicebindingv1.ServiceBindingConditionReady.
							Reason("WorkloadNotFound").Message("the workload was not found"),
						dieservicebindingv1.ServiceBindingConditionWorkloadProjected.
							Reason("WorkloadNotFound").Message("the workload was not found"),
					)
				}).
				DieReleasePtr(),
			ExpectTracks: []rtesting.TrackRequest{
				rtesting.NewTrackRequest(workload1, serviceBinding, scheme),
			},
		},
		"resolve named workload forbidden": {
			Resource: serviceBinding.
				SpecDie(func(d *dieservicebindingv1.ServiceBindingSpecDie) {
					d.WorkloadDie(func(d *dieservicebindingv1.ServiceBindingWorkloadReferenceDie) {
						d.APIVersion("apps/v1")
						d.Kind("Deployment")
						d.Name("my-workload-1")
					})
				}).
				DieReleasePtr(),
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
			ExpectResource: serviceBinding.
				SpecDie(func(d *dieservicebindingv1.ServiceBindingSpecDie) {
					d.WorkloadDie(func(d *dieservicebindingv1.ServiceBindingWorkloadReferenceDie) {
						d.APIVersion("apps/v1")
						d.Kind("Deployment")
						d.Name("my-workload-1")
					})
				}).
				StatusDie(func(d *dieservicebindingv1.ServiceBindingStatusDie) {
					d.ConditionsDie(
						dieservicebindingv1.ServiceBindingConditionReady.
							False().
							Reason("WorkloadForbidden").
							Message("the controller does not have permission to get the workload"),
						dieservicebindingv1.ServiceBindingConditionWorkloadProjected.
							False().
							Reason("WorkloadForbidden").
							Message("the controller does not have permission to get the workload"),
					)
				}).
				DieReleasePtr(),
			ExpectTracks: []rtesting.TrackRequest{
				rtesting.NewTrackRequest(workload1, serviceBinding, scheme),
			},
		},
		"resolve selected workload": {
			Resource: serviceBinding.
				SpecDie(func(d *dieservicebindingv1.ServiceBindingSpecDie) {
					d.WorkloadDie(func(d *dieservicebindingv1.ServiceBindingWorkloadReferenceDie) {
						d.APIVersion("apps/v1")
						d.Kind("Deployment")
						d.SelectorDie(func(d *diemetav1.LabelSelectorDie) {
							d.AddMatchLabel("app", "my")
						})
					})
				}).
				DieReleasePtr(),
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
			ExpectTracks: []rtesting.TrackRequest{
				{
					Tracker: types.NamespacedName{Namespace: serviceBinding.GetNamespace(), Name: serviceBinding.GetName()},
					TrackedReference: tracker.Reference{
						APIGroup:  "apps",
						Kind:      "Deployment",
						Namespace: serviceBinding.GetNamespace(),
						Selector: labels.SelectorFromSet(labels.Set{
							"app": "my",
						}),
					},
				},
			},
		},
		"resolve selected workload not found": {
			GivenObjects: []client.Object{},
			Resource: serviceBinding.
				SpecDie(func(d *dieservicebindingv1.ServiceBindingSpecDie) {
					d.WorkloadDie(func(d *dieservicebindingv1.ServiceBindingWorkloadReferenceDie) {
						d.APIVersion("apps/v1")
						d.Kind("Deployment")
						d.SelectorDie(func(d *diemetav1.LabelSelectorDie) {
							d.AddMatchLabel("app", "my")
						})
					})
				}).
				DieReleasePtr(),
			ExpectStashedValues: map[reconcilers.StashKey]interface{}{
				controllers.WorkloadsStashKey: []runtime.Object{},
			},
			ExpectTracks: []rtesting.TrackRequest{
				{
					Tracker: types.NamespacedName{Namespace: serviceBinding.GetNamespace(), Name: serviceBinding.GetName()},
					TrackedReference: tracker.Reference{
						APIGroup:  "apps",
						Kind:      "Deployment",
						Namespace: serviceBinding.GetNamespace(),
						Selector: labels.SelectorFromSet(labels.Set{
							"app": "my",
						}),
					},
				},
			},
		},
		"resolve selected workload forbidden": {
			Resource: serviceBinding.
				SpecDie(func(d *dieservicebindingv1.ServiceBindingSpecDie) {
					d.WorkloadDie(func(d *dieservicebindingv1.ServiceBindingWorkloadReferenceDie) {
						d.APIVersion("apps/v1")
						d.Kind("Deployment")
						d.SelectorDie(func(d *diemetav1.LabelSelectorDie) {
							d.AddMatchLabel("app", "my")
						})
					})
				}).
				DieReleasePtr(),
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
			ExpectResource: serviceBinding.
				SpecDie(func(d *dieservicebindingv1.ServiceBindingSpecDie) {
					d.WorkloadDie(func(d *dieservicebindingv1.ServiceBindingWorkloadReferenceDie) {
						d.APIVersion("apps/v1")
						d.Kind("Deployment")
						d.SelectorDie(func(d *diemetav1.LabelSelectorDie) {
							d.AddMatchLabel("app", "my")
						})
					})
				}).
				StatusDie(func(d *dieservicebindingv1.ServiceBindingStatusDie) {
					d.ConditionsDie(
						dieservicebindingv1.ServiceBindingConditionReady.
							False().
							Reason("WorkloadForbidden").
							Message("the controller does not have permission to list the workloads"),
						dieservicebindingv1.ServiceBindingConditionWorkloadProjected.
							False().
							Reason("WorkloadForbidden").
							Message("the controller does not have permission to list the workloads"),
					)
				}).
				DieReleasePtr(),
			ExpectTracks: []rtesting.TrackRequest{
				{
					Tracker: types.NamespacedName{Namespace: serviceBinding.GetNamespace(), Name: serviceBinding.GetName()},
					TrackedReference: tracker.Reference{
						APIGroup:  "apps",
						Kind:      "Deployment",
						Namespace: serviceBinding.GetNamespace(),
						Selector: labels.SelectorFromSet(labels.Set{
							"app": "my",
						}),
					},
				},
			},
		},
	}

	rts.Run(t, scheme, func(t *testing.T, tc *rtesting.SubReconcilerTestCase[*servicebindingv1.ServiceBinding], c reconcilers.Config) reconcilers.SubReconciler[*servicebindingv1.ServiceBinding] {
		return controllers.ResolveWorkloads(lifecycle.ServiceBindingHooks{})
	})
}

func TestProjectBinding(t *testing.T) {
	namespace := "test-namespace"
	name := "my-binding"
	uid := types.UID("dde10100-d7b3-4cba-9430-51d60a8612a6")
	secretName := "my-secret"

	podSpecableMapping := `{"versions":[{"version":"*","annotations":".spec.template.metadata.annotations","containers":[{"path":".spec.template.spec.initContainers[*]","name":".name","env":".env","volumeMounts":".volumeMounts"},{"path":".spec.template.spec.containers[*]","name":".name","env":".env","volumeMounts":".volumeMounts"}],"volumes":".spec.template.spec.volumes"}]}`

	scheme := runtime.NewScheme()
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(servicebindingv1.AddToScheme(scheme))

	now := metav1.Now().Rfc3339Copy()

	serviceBinding := dieservicebindingv1.ServiceBindingBlank.
		MetadataDie(func(d *diemetav1.ObjectMetaDie) {
			d.Namespace(namespace)
			d.Name(name)
			d.UID(uid)
		}).
		SpecDie(func(d *dieservicebindingv1.ServiceBindingSpecDie) {
			d.Name(name)
			d.WorkloadDie(func(d *dieservicebindingv1.ServiceBindingWorkloadReferenceDie) {
				d.APIVersion("apps/v1")
				d.Kind("Deployment")
				d.Name("my-workload")
			})
		}).
		StatusDie(func(d *dieservicebindingv1.ServiceBindingStatusDie) {
			d.BindingDie(func(d *dieservicebindingv1.ServiceBindingSecretReferenceDie) {
				d.Name(secretName)
			})
		})

	workloadMapping := dieservicebindingv1.ClusterWorkloadResourceMappingBlank.
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
		MetadataDie(func(d *diemetav1.ObjectMetaDie) {
			d.AddAnnotation(fmt.Sprintf("projector.servicebinding.io/mapping-%s", uid), podSpecableMapping)
		}).
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
	unstructured.SetNestedMap(unprojectedWorkload.UnstructuredContent(), map[string]interface{}{}, "metadata", "annotations")
	unstructured.SetNestedMap(unprojectedWorkload.UnstructuredContent(), map[string]interface{}{}, "spec", "template", "metadata", "annotations")
	containers, _, _ := unstructured.NestedSlice(unprojectedWorkload.UnstructuredContent(), "spec", "template", "spec", "containers")
	unstructured.SetNestedSlice(containers[0].(map[string]interface{}), []interface{}{}, "volumeMounts")
	unstructured.SetNestedSlice(unprojectedWorkload.UnstructuredContent(), containers, "spec", "template", "spec", "containers")
	unstructured.SetNestedSlice(unprojectedWorkload.UnstructuredContent(), []interface{}{}, "spec", "template", "spec", "volumes")

	rts := rtesting.SubReconcilerTests[*servicebindingv1.ServiceBinding]{
		"project workload": {
			Resource: serviceBinding.
				SpecDie(func(d *dieservicebindingv1.ServiceBindingSpecDie) {
					d.WorkloadDie(func(d *dieservicebindingv1.ServiceBindingWorkloadReferenceDie) {
						d.APIVersion("apps/v1")
						d.Kind("Deployment")
						d.Name(workload.GetName())
					})
				}).
				DieReleasePtr(),
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
					d.Finalizers("servicebinding.io/finalizer")
				}).
				SpecDie(func(d *dieservicebindingv1.ServiceBindingSpecDie) {
					d.WorkloadDie(func(d *dieservicebindingv1.ServiceBindingWorkloadReferenceDie) {
						d.APIVersion("apps/v1")
						d.Kind("Deployment")
						d.Name(workload.GetName())
					})
				}).
				DieReleasePtr(),
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

	rts.Run(t, scheme, func(t *testing.T, tc *rtesting.SubReconcilerTestCase[*servicebindingv1.ServiceBinding], c reconcilers.Config) reconcilers.SubReconciler[*servicebindingv1.ServiceBinding] {
		restMapper := c.RESTMapper().(*meta.DefaultRESTMapper)
		restMapper.Add(schema.GroupVersionKind{Group: "apps", Version: "v1", Kind: "Deployment"}, meta.RESTScopeNamespace)
		return controllers.ProjectBinding(lifecycle.ServiceBindingHooks{})
	})
}

func TestPatchWorkloads(t *testing.T) {
	namespace := "test-namespace"
	name := "my-binding"
	uid := types.UID("dde10100-d7b3-4cba-9430-51d60a8612a6")

	now := metav1.Now().Rfc3339Copy()
	scheme := runtime.NewScheme()
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(servicebindingv1.AddToScheme(scheme))

	serviceBinding := dieservicebindingv1.ServiceBindingBlank.
		MetadataDie(func(d *diemetav1.ObjectMetaDie) {
			d.Namespace(namespace)
			d.Name(name)
		}).
		StatusDie(func(d *dieservicebindingv1.ServiceBindingStatusDie) {
			d.ConditionsDie(
				dieservicebindingv1.ServiceBindingConditionReady,
				dieservicebindingv1.ServiceBindingConditionServiceAvailable.True().Reason("ResolvedBindingSecret"),
				dieservicebindingv1.ServiceBindingConditionWorkloadProjected,
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

	rts := rtesting.SubReconcilerTests[*servicebindingv1.ServiceBinding]{
		"in sync": {
			Resource: serviceBinding.
				StatusDie(func(d *dieservicebindingv1.ServiceBindingStatusDie) {
					d.ConditionsDie(
						dieservicebindingv1.ServiceBindingConditionReady.True().Reason("ServiceBound"),
						dieservicebindingv1.ServiceBindingConditionServiceAvailable.True().Reason("ResolvedBindingSecret"),
						dieservicebindingv1.ServiceBindingConditionWorkloadProjected.True().Reason("WorkloadProjected"),
					)
				}).
				DieReleasePtr(),
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
			Resource: serviceBinding.DieReleasePtr(),
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
				StatusDie(func(d *dieservicebindingv1.ServiceBindingStatusDie) {
					d.ConditionsDie(
						dieservicebindingv1.ServiceBindingConditionReady.True().Reason("ServiceBound"),
						dieservicebindingv1.ServiceBindingConditionServiceAvailable.True().Reason("ResolvedBindingSecret"),
						dieservicebindingv1.ServiceBindingConditionWorkloadProjected.True().Reason("WorkloadProjected"),
					)
				}).
				DieReleasePtr(),
			ExpectEvents: []rtesting.Event{
				rtesting.NewEvent(serviceBinding, scheme, corev1.EventTypeNormal, "Updated", "Updated Deployment %q", "my-workload"),
			},
			ExpectUpdates: []client.Object{
				workload.
					SpecDie(func(d *dieappsv1.DeploymentSpecDie) {
						// not something a binding would ever project, but good enough for a test
						d.Paused(true)
					}).DieReleaseUnstructured(),
			},
		},
		"update workload ignoring not found errors": {
			Resource: serviceBinding.DieReleasePtr(),
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
				StatusDie(func(d *dieservicebindingv1.ServiceBindingStatusDie) {
					d.ConditionsDie(
						dieservicebindingv1.ServiceBindingConditionReady.True().Reason("ServiceBound"),
						dieservicebindingv1.ServiceBindingConditionServiceAvailable.True().Reason("ResolvedBindingSecret"),
						dieservicebindingv1.ServiceBindingConditionWorkloadProjected.True().Reason("WorkloadProjected"),
					)
				}).
				DieReleasePtr(),
			ExpectEvents: []rtesting.Event{
				rtesting.NewEvent(serviceBinding, scheme, corev1.EventTypeWarning, "UpdateFailed", "Failed to update Deployment %q: deployments.apps %q not found", "my-workload", "my-workload"),
			},
			ExpectUpdates: []client.Object{
				workload.
					SpecDie(func(d *dieappsv1.DeploymentSpecDie) {
						// not something a binding would ever project, but good enough for a test
						d.Paused(true)
					}).DieReleaseUnstructured(),
			},
		},
		"update workload forbidden": {
			Resource: serviceBinding.DieReleasePtr(),
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
				StatusDie(func(d *dieservicebindingv1.ServiceBindingStatusDie) {
					d.ConditionsDie(
						dieservicebindingv1.ServiceBindingConditionReady.
							False().
							Reason("WorkloadForbidden").
							Message("the controller does not have permission to update the workloads"),
						dieservicebindingv1.ServiceBindingConditionServiceAvailable.
							True().
							Reason("ResolvedBindingSecret"),
						dieservicebindingv1.ServiceBindingConditionWorkloadProjected.
							False().
							Reason("WorkloadForbidden").
							Message("the controller does not have permission to update the workloads"),
					)
				}).
				DieReleasePtr(),
			ExpectEvents: []rtesting.Event{
				rtesting.NewEvent(serviceBinding, scheme, corev1.EventTypeWarning, "UpdateFailed", "Failed to update Deployment %q: forbidden: test forbidden", "my-workload"),
			},
			ExpectUpdates: []client.Object{
				workload.
					SpecDie(func(d *dieappsv1.DeploymentSpecDie) {
						// not something a binding would ever project, but good enough for a test
						d.Paused(true)
					}).DieReleaseUnstructured(),
			},
		},
		"require same number of workloads and projected workloads": {
			Resource: serviceBinding.DieReleasePtr(),
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
			Resource: serviceBinding.DieReleasePtr(),
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
			Resource: serviceBinding.DieReleasePtr(),
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

	rts.Run(t, scheme, func(t *testing.T, tc *rtesting.SubReconcilerTestCase[*servicebindingv1.ServiceBinding], c reconcilers.Config) reconcilers.SubReconciler[*servicebindingv1.ServiceBinding] {
		return controllers.PatchWorkloads()
	})
}

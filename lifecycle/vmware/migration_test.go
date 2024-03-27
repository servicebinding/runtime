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

package vmware_test

import (
	"fmt"
	"testing"

	"gomodules.xyz/jsonpatch/v2"
	admissionv1 "k8s.io/api/admission/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/uuid"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	dieadmissionv1 "reconciler.io/dies/apis/admission/v1"
	dieappsv1 "reconciler.io/dies/apis/apps/v1"
	diecorev1 "reconciler.io/dies/apis/core/v1"
	diemetav1 "reconciler.io/dies/apis/meta/v1"
	"reconciler.io/runtime/reconcilers"
	rtesting "reconciler.io/runtime/testing"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	servicebindingv1 "github.com/servicebinding/runtime/apis/v1"
	"github.com/servicebinding/runtime/controllers"
	dieservicebindingv1 "github.com/servicebinding/runtime/dies/v1"
	"github.com/servicebinding/runtime/lifecycle"
	"github.com/servicebinding/runtime/lifecycle/vmware"
)

func TestMigrationHooks_Controller(t *testing.T) {
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

	vmwareServiceBindingProjection := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "internal.bindings.labs.vmware.com/v1alpha1",
			"kind":       "ServiceBindingProjection",
			"metadata": map[string]interface{}{
				"namespace": serviceBinding.GetNamespace(),
				"name":      serviceBinding.GetName(),
				"finalizers": []interface{}{
					"servicebindingprojections.internal.bindings.labs.vmware.com",
				},
				"labels": map[string]interface{}{
					"servicebinding.io/servicebinding": serviceBinding.GetName(),
				},
				"ownerReferences": []interface{}{
					map[string]interface{}{
						"apiVersion":         "servicebinding.io/v1alpha3",
						"blockOwnerDeletion": true,
						"controller":         true,
						"kind":               "ServiceBinding",
						"name":               serviceBinding.GetName(),
						"uid":                string(serviceBinding.GetUID()),
					},
				},
			},
			"spec": map[string]interface{}{
				"binding": map[string]interface{}{
					"name": secretName,
				},
				"name": serviceBinding.GetName(),
				"workload": map[string]interface{}{
					"apiVersion": "apps/v1",
					"kind":       "Deployment",
					"name":       workload.GetName(),
				},
			},
		},
	}

	vmwareWorkload := workload.
		MetadataDie(func(d *diemetav1.ObjectMetaDie) {
			d.AddAnnotation("internal.bindings.labs.vmware.com/projection-4b2c350fb984fc36b6cf39515a2efced0fcb5053", secretName)
		}).
		SpecDie(func(d *dieappsv1.DeploymentSpecDie) {
			d.TemplateDie(func(d *diecorev1.PodTemplateSpecDie) {
				d.SpecDie(func(d *diecorev1.PodSpecDie) {
					d.ContainerDie("my-container", func(d *diecorev1.ContainerDie) {
						d.EnvDie("SERVICE_BINDING_ROOT", func(d *diecorev1.EnvVarDie) {
							d.Value("/bindings")
						})
						d.VolumeMountDie("binding-4b2c350fb984fc36b6cf39515a2efced0fcb5053", func(d *diecorev1.VolumeMountDie) {
							d.MountPath(fmt.Sprintf("/bindings/%s", serviceBinding.GetName()))
							d.ReadOnly(true)
						})
					})
					d.VolumeDie("binding-4b2c350fb984fc36b6cf39515a2efced0fcb5053", func(d *diecorev1.VolumeDie) {
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
		"migrate vmware binding": {
			Request: request,
			StatusSubResourceTypes: []client.Object{
				&servicebindingv1.ServiceBinding{},
			},
			GivenObjects: []client.Object{
				serviceBinding.
					StatusDie(func(d *dieservicebindingv1.ServiceBindingStatusDie) {
						d.ConditionsDie(
							diemetav1.ConditionBlank.Type("Ready").True().Reason("Ready").LastTransitionTime(now),
							diemetav1.ConditionBlank.Type("ServiceAvailable").True().Reason("Available").LastTransitionTime(now),
							diemetav1.ConditionBlank.Type("ProjectionReady").True().Reason("Projected").LastTransitionTime(now),
						)
						d.BindingDie(func(d *dieservicebindingv1.ServiceBindingSecretReferenceDie) {
							d.Name(secretName)
						})
					}),
				vmwareServiceBindingProjection,
				vmwareWorkload,
			},
			ExpectTracks: []rtesting.TrackRequest{
				rtesting.NewTrackRequest(projectedWorkload, serviceBinding, scheme),
				rtesting.NewTrackRequest(workloadMapping, serviceBinding, scheme),
			},
			ExpectEvents: []rtesting.Event{
				rtesting.NewEvent(serviceBinding, scheme, corev1.EventTypeNormal, "FinalizerPatched", "Patched finalizer %q", "servicebinding.io/finalizer"),
				rtesting.NewEvent(vmwareServiceBindingProjection, scheme, corev1.EventTypeNormal, "FinalizerPatched", "Patched finalizer %q", "servicebindingprojections.internal.bindings.labs.vmware.com"),
				rtesting.NewEvent(serviceBinding, scheme, corev1.EventTypeNormal, "Updated", "Updated Deployment %q", workload.GetName()),
				rtesting.NewEvent(serviceBinding, scheme, corev1.EventTypeNormal, "StatusUpdated", "Updated status"),
			},
			ExpectPatches: []rtesting.PatchRef{
				{Group: "servicebinding.io", Kind: "ServiceBinding", Namespace: serviceBinding.GetNamespace(), Name: serviceBinding.GetName(), PatchType: types.MergePatchType, Patch: []byte(`{"metadata":{"finalizers":["servicebinding.io/finalizer"],"resourceVersion":"999"}}`)},
				{Group: "internal.bindings.labs.vmware.com", Kind: "ServiceBindingProjection", Namespace: vmwareServiceBindingProjection.GetNamespace(), Name: vmwareServiceBindingProjection.GetName(), PatchType: types.MergePatchType, Patch: []byte(`{"metadata":{"finalizers":[],"resourceVersion":"999"}}`)},
			},
			ExpectDeletes: []rtesting.DeleteRef{
				rtesting.NewDeleteRefFromObject(vmwareServiceBindingProjection, scheme),
			},
			ExpectUpdates: []client.Object{
				projectedWorkload.DieReleaseUnstructured(),
			},
			ExpectStatusUpdates: []client.Object{
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
			},
		},
		"migrate vmware binding with projected envvars and overridden type and provider": {
			Request: request,
			StatusSubResourceTypes: []client.Object{
				&servicebindingv1.ServiceBinding{},
			},
			GivenObjects: []client.Object{
				serviceBinding.
					SpecDie(func(d *dieservicebindingv1.ServiceBindingSpecDie) {
						d.Type("overridden-type")
						d.Provider("overridden-provider")
						d.EnvDie("BOUND_PASSWORD", func(d *dieservicebindingv1.EnvMappingDie) {
							d.Key("password")
						})
						d.EnvDie("BOUND_TYPE", func(d *dieservicebindingv1.EnvMappingDie) {
							d.Key("type")
						})
						d.EnvDie("BOUND_PROVIDER", func(d *dieservicebindingv1.EnvMappingDie) {
							d.Key("provider")
						})
					}).
					StatusDie(func(d *dieservicebindingv1.ServiceBindingStatusDie) {
						d.ConditionsDie(
							diemetav1.ConditionBlank.Type("Ready").True().Reason("Ready").LastTransitionTime(now),
							diemetav1.ConditionBlank.Type("ServiceAvailable").True().Reason("Available").LastTransitionTime(now),
							diemetav1.ConditionBlank.Type("ProjectionReady").True().Reason("Projected").LastTransitionTime(now),
						)
						d.BindingDie(func(d *dieservicebindingv1.ServiceBindingSecretReferenceDie) {
							d.Name(secretName)
						})
					}),
				vmwareServiceBindingProjection,
				vmwareWorkload.
					MetadataDie(func(d *diemetav1.ObjectMetaDie) {
						d.AddAnnotation("internal.bindings.labs.vmware.com/projection-4b2c350fb984fc36b6cf39515a2efced0fcb5053-type", "overridden-type")
						d.AddAnnotation("internal.bindings.labs.vmware.com/projection-4b2c350fb984fc36b6cf39515a2efced0fcb5053-provider", "overridden-provider")
					}).
					SpecDie(func(d *dieappsv1.DeploymentSpecDie) {
						d.TemplateDie(func(d *diecorev1.PodTemplateSpecDie) {
							d.SpecDie(func(d *diecorev1.PodSpecDie) {
								d.ContainerDie("my-container", func(d *diecorev1.ContainerDie) {
									d.EnvDie("BOUND_PASSWORD", func(d *diecorev1.EnvVarDie) {
										d.ValueFrom(&corev1.EnvVarSource{
											SecretKeyRef: &corev1.SecretKeySelector{
												Key: "password",
												LocalObjectReference: corev1.LocalObjectReference{
													Name: secretName,
												},
											},
										})
									})
									d.EnvDie("BOUND_TYPE", func(d *diecorev1.EnvVarDie) {
										d.ValueFrom(&corev1.EnvVarSource{
											FieldRef: &corev1.ObjectFieldSelector{
												APIVersion: "v1",
												FieldPath:  "metadata.annotations['internal.bindings.labs.vmware.com/projection-4b2c350fb984fc36b6cf39515a2efced0fcb5053-type']",
											},
										})
									})
									d.EnvDie("BOUND_PROVIDER", func(d *diecorev1.EnvVarDie) {
										d.ValueFrom(&corev1.EnvVarSource{
											FieldRef: &corev1.ObjectFieldSelector{
												APIVersion: "v1",
												FieldPath:  "metadata.annotations['internal.bindings.labs.vmware.com/projection-4b2c350fb984fc36b6cf39515a2efced0fcb5053-provider']",
											},
										})
									})
								})
								d.VolumeDie("binding-4b2c350fb984fc36b6cf39515a2efced0fcb5053", func(d *diecorev1.VolumeDie) {
									d.ProjectedDie(func(d *diecorev1.ProjectedVolumeSourceDie) {
										d.Sources(append(
											d.DieRelease().Sources,
											corev1.VolumeProjection{
												DownwardAPI: &corev1.DownwardAPIProjection{
													Items: []corev1.DownwardAPIVolumeFile{
														{
															FieldRef: &corev1.ObjectFieldSelector{
																APIVersion: "v1",
																FieldPath:  "metadata.annotations['internal.bindings.labs.vmware.com/projection-4b2c350fb984fc36b6cf39515a2efced0fcb5053-type']",
															},
															Path: "type",
														},
													},
												},
											},
											corev1.VolumeProjection{
												DownwardAPI: &corev1.DownwardAPIProjection{
													Items: []corev1.DownwardAPIVolumeFile{
														{
															FieldRef: &corev1.ObjectFieldSelector{
																APIVersion: "v1",
																FieldPath:  "metadata.annotations['internal.bindings.labs.vmware.com/projection-4b2c350fb984fc36b6cf39515a2efced0fcb5053-provider']",
															},
															Path: "provider",
														},
													},
												},
											},
										)...)
									})
								})
							})
						})
					}),
			},
			ExpectTracks: []rtesting.TrackRequest{
				rtesting.NewTrackRequest(projectedWorkload, serviceBinding, scheme),
				rtesting.NewTrackRequest(workloadMapping, serviceBinding, scheme),
			},
			ExpectEvents: []rtesting.Event{
				rtesting.NewEvent(serviceBinding, scheme, corev1.EventTypeNormal, "FinalizerPatched", "Patched finalizer %q", "servicebinding.io/finalizer"),
				rtesting.NewEvent(vmwareServiceBindingProjection, scheme, corev1.EventTypeNormal, "FinalizerPatched", "Patched finalizer %q", "servicebindingprojections.internal.bindings.labs.vmware.com"),
				rtesting.NewEvent(serviceBinding, scheme, corev1.EventTypeNormal, "Updated", "Updated Deployment %q", workload.GetName()),
				rtesting.NewEvent(serviceBinding, scheme, corev1.EventTypeNormal, "StatusUpdated", "Updated status"),
			},
			ExpectPatches: []rtesting.PatchRef{
				{Group: "servicebinding.io", Kind: "ServiceBinding", Namespace: serviceBinding.GetNamespace(), Name: serviceBinding.GetName(), PatchType: types.MergePatchType, Patch: []byte(`{"metadata":{"finalizers":["servicebinding.io/finalizer"],"resourceVersion":"999"}}`)},
				{Group: "internal.bindings.labs.vmware.com", Kind: "ServiceBindingProjection", Namespace: vmwareServiceBindingProjection.GetNamespace(), Name: vmwareServiceBindingProjection.GetName(), PatchType: types.MergePatchType, Patch: []byte(`{"metadata":{"finalizers":[],"resourceVersion":"999"}}`)},
			},
			ExpectDeletes: []rtesting.DeleteRef{
				rtesting.NewDeleteRefFromObject(vmwareServiceBindingProjection, scheme),
			},
			ExpectUpdates: []client.Object{
				projectedWorkload.
					SpecDie(func(d *dieappsv1.DeploymentSpecDie) {
						d.TemplateDie(func(d *diecorev1.PodTemplateSpecDie) {
							d.MetadataDie(func(d *diemetav1.ObjectMetaDie) {
								d.AddAnnotation("projector.servicebinding.io/type-dde10100-d7b3-4cba-9430-51d60a8612a6", "overridden-type")
								d.AddAnnotation("projector.servicebinding.io/provider-dde10100-d7b3-4cba-9430-51d60a8612a6", "overridden-provider")
							})
							d.SpecDie(func(d *diecorev1.PodSpecDie) {
								d.ContainerDie("my-container", func(d *diecorev1.ContainerDie) {
									d.EnvDie("BOUND_PASSWORD", func(d *diecorev1.EnvVarDie) {
										d.ValueFrom(&corev1.EnvVarSource{
											SecretKeyRef: &corev1.SecretKeySelector{
												Key: "password",
												LocalObjectReference: corev1.LocalObjectReference{
													Name: secretName,
												},
											},
										})
									})
									d.EnvDie("BOUND_PROVIDER", func(d *diecorev1.EnvVarDie) {
										d.ValueFrom(&corev1.EnvVarSource{
											FieldRef: &corev1.ObjectFieldSelector{
												FieldPath: "metadata.annotations['projector.servicebinding.io/provider-dde10100-d7b3-4cba-9430-51d60a8612a6']",
											},
										})
									})
									d.EnvDie("BOUND_TYPE", func(d *diecorev1.EnvVarDie) {
										d.ValueFrom(&corev1.EnvVarSource{
											FieldRef: &corev1.ObjectFieldSelector{
												FieldPath: "metadata.annotations['projector.servicebinding.io/type-dde10100-d7b3-4cba-9430-51d60a8612a6']",
											},
										})
									})
								})
								d.VolumeDie("servicebinding-dde10100-d7b3-4cba-9430-51d60a8612a6", func(d *diecorev1.VolumeDie) {
									d.ProjectedDie(func(d *diecorev1.ProjectedVolumeSourceDie) {
										d.Sources(append(
											d.DieRelease().Sources,
											corev1.VolumeProjection{
												DownwardAPI: &corev1.DownwardAPIProjection{
													Items: []corev1.DownwardAPIVolumeFile{
														{
															FieldRef: &corev1.ObjectFieldSelector{
																FieldPath: "metadata.annotations['projector.servicebinding.io/type-dde10100-d7b3-4cba-9430-51d60a8612a6']",
															},
															Path: "type",
														},
													},
												},
											},
											corev1.VolumeProjection{
												DownwardAPI: &corev1.DownwardAPIProjection{
													Items: []corev1.DownwardAPIVolumeFile{
														{
															FieldRef: &corev1.ObjectFieldSelector{
																FieldPath: "metadata.annotations['projector.servicebinding.io/provider-dde10100-d7b3-4cba-9430-51d60a8612a6']",
															},
															Path: "provider",
														},
													},
												},
											},
										)...)
									})
								})
							})
						})
					}).
					DieReleaseUnstructured(),
			},
			ExpectStatusUpdates: []client.Object{
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
			},
		},
	}

	rts.Run(t, scheme, func(t *testing.T, tc *rtesting.ReconcilerTestCase, c reconcilers.Config) reconcile.Reconciler {
		restMapper := c.RESTMapper().(*meta.DefaultRESTMapper)
		restMapper.Add(schema.GroupVersionKind{Group: "apps", Version: "v1", Kind: "Deployment"}, meta.RESTScopeNamespace)
		restMapper.Add(schema.GroupVersionKind{Group: "internal.bindings.labs.vmware.com", Version: "v1alpha1", Kind: "ServiceBindingProjection"}, meta.RESTScopeNamespace)
		hooks := vmware.InstallMigrationHooks(lifecycle.ServiceBindingHooks{})
		return controllers.ServiceBindingReconciler(c, hooks)
	})

}

func TestMigrationHooks_Webhook(t *testing.T) {
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

	vmwareServiceBindingProjection := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "internal.bindings.labs.vmware.com/v1alpha1",
			"kind":       "ServiceBindingProjection",
			"metadata": map[string]interface{}{
				"namespace": serviceBinding.GetNamespace(),
				"name":      serviceBinding.GetName(),
				"finalizers": []interface{}{
					"servicebindingprojections.internal.bindings.labs.vmware.com",
				},
				"labels": map[string]interface{}{
					"servicebinding.io/servicebinding": serviceBinding.GetName(),
				},
				"ownerReferences": []interface{}{
					map[string]interface{}{
						"apiVersion":         "servicebinding.io/v1alpha3",
						"blockOwnerDeletion": true,
						"controller":         true,
						"kind":               "ServiceBinding",
						"name":               serviceBinding.GetName(),
						"uid":                string(serviceBinding.GetUID()),
					},
				},
			},
			"spec": map[string]interface{}{
				"binding": map[string]interface{}{
					"name": secretName,
				},
				"name": serviceBinding.GetName(),
				"workload": map[string]interface{}{
					"apiVersion": "apps/v1",
					"kind":       "Deployment",
					"name":       workload.GetName(),
				},
			},
		},
	}

	vmwareWorkload := workload.
		MetadataDie(func(d *diemetav1.ObjectMetaDie) {
			d.AddAnnotation("internal.bindings.labs.vmware.com/projection-4b2c350fb984fc36b6cf39515a2efced0fcb5053", secretName)
		}).
		SpecDie(func(d *dieappsv1.DeploymentSpecDie) {
			d.TemplateDie(func(d *diecorev1.PodTemplateSpecDie) {
				d.SpecDie(func(d *diecorev1.PodSpecDie) {
					d.ContainerDie("my-container", func(d *diecorev1.ContainerDie) {
						d.EnvDie("SERVICE_BINDING_ROOT", func(d *diecorev1.EnvVarDie) {
							d.Value("/bindings")
						})
						d.VolumeMountDie("binding-4b2c350fb984fc36b6cf39515a2efced0fcb5053", func(d *diecorev1.VolumeMountDie) {
							d.MountPath(fmt.Sprintf("/bindings/%s", serviceBinding.GetName()))
							d.ReadOnly(true)
						})
					})
					d.VolumeDie("binding-4b2c350fb984fc36b6cf39515a2efced0fcb5053", func(d *diecorev1.VolumeDie) {
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

	request := dieadmissionv1.AdmissionRequestBlank.
		UID(uuid.NewUUID()).
		Operation(admissionv1.Create)
	response := dieadmissionv1.AdmissionResponseBlank.
		Allowed(true)

	addWorkloadRefIndex := func(cb *fake.ClientBuilder) *fake.ClientBuilder {
		return cb.WithIndex(&servicebindingv1.ServiceBinding{}, controllers.WorkloadRefIndexKey, controllers.WorkloadRefIndexFunc)
	}

	rts := rtesting.AdmissionWebhookTests{
		"in sync": {
			WithClientBuilder: addWorkloadRefIndex,
			Request: &admission.Request{
				AdmissionRequest: request.
					Object(projectedWorkload.DieReleaseRawExtension()).
					DieRelease(),
			},
			GivenObjects: []client.Object{
				serviceBinding.
					StatusDie(func(d *dieservicebindingv1.ServiceBindingStatusDie) {
						d.ConditionsDie(
							diemetav1.ConditionBlank.Type("Ready").True().Reason("Ready").LastTransitionTime(now),
							diemetav1.ConditionBlank.Type("ServiceAvailable").True().Reason("Available").LastTransitionTime(now),
							diemetav1.ConditionBlank.Type("ProjectionReady").True().Reason("Projected").LastTransitionTime(now),
						)
						d.BindingDie(func(d *dieservicebindingv1.ServiceBindingSecretReferenceDie) {
							d.Name(secretName)
						})
					}),
			},
			ExpectedResponse: admission.Response{
				AdmissionResponse: response.DieRelease(),
			},
		},
		"cleanup workload, ignore service binding cleanup": {
			WithClientBuilder: addWorkloadRefIndex,
			Request: &admission.Request{
				AdmissionRequest: request.
					Object(vmwareWorkload.DieReleaseRawExtension()).
					DieRelease(),
			},
			GivenObjects: []client.Object{
				serviceBinding.
					StatusDie(func(d *dieservicebindingv1.ServiceBindingStatusDie) {
						d.ConditionsDie(
							diemetav1.ConditionBlank.Type("Ready").True().Reason("Ready").LastTransitionTime(now),
							diemetav1.ConditionBlank.Type("ServiceAvailable").True().Reason("Available").LastTransitionTime(now),
							diemetav1.ConditionBlank.Type("ProjectionReady").True().Reason("Projected").LastTransitionTime(now),
						)
						d.BindingDie(func(d *dieservicebindingv1.ServiceBindingSecretReferenceDie) {
							d.Name(secretName)
						})
					}),
				// the vmware projection should be ignored by the webhook, it will be deleted by the controller
				vmwareServiceBindingProjection,
			},
			ExpectedResponse: admission.Response{
				AdmissionResponse: response.DieRelease(),
				Patches: []jsonpatch.Operation{
					{
						Operation: "add",
						Path:      fmt.Sprintf("/metadata/annotations/projector.servicebinding.io~1mapping-%s", string(serviceBinding.GetUID())),
						Value:     podSpecableMapping,
					},
					{
						Operation: "remove",
						Path:      "/metadata/annotations/internal.bindings.labs.vmware.com~1projection-4b2c350fb984fc36b6cf39515a2efced0fcb5053",
					},
					{
						Operation: "add",
						Path:      "/spec/template/metadata/annotations",
						Value: map[string]interface{}{
							fmt.Sprintf("projector.servicebinding.io/secret-%s", string(serviceBinding.GetUID())): secretName,
						},
					},
					{
						Operation: "replace",
						Path:      "/spec/template/spec/containers/0/volumeMounts/0/name",
						Value:     "servicebinding-dde10100-d7b3-4cba-9430-51d60a8612a6",
					},
					{
						Operation: "replace",
						Path:      "/spec/template/spec/volumes/0/name",
						Value:     "servicebinding-dde10100-d7b3-4cba-9430-51d60a8612a6",
					},
				},
			},
		},
	}

	rts.Run(t, scheme, func(t *testing.T, wtc *rtesting.AdmissionWebhookTestCase, c reconcilers.Config) *admission.Webhook {
		restMapper := c.RESTMapper().(*meta.DefaultRESTMapper)
		restMapper.Add(schema.GroupVersionKind{Group: "apps", Version: "v1", Kind: "Deployment"}, meta.RESTScopeNamespace)
		restMapper.Add(schema.GroupVersionKind{Group: "internal.bindings.labs.vmware.com", Version: "v1alpha1", Kind: "ServiceBindingProjection"}, meta.RESTScopeNamespace)
		hooks := vmware.InstallMigrationHooks(lifecycle.ServiceBindingHooks{})
		return controllers.AdmissionProjectorWebhook(c, hooks).Build()
	})

}

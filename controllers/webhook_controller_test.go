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
	"context"
	"fmt"
	"testing"

	dieadmissionv1 "dies.dev/apis/admission/v1"
	dieadmissionregistrationv1 "dies.dev/apis/admissionregistration/v1"
	dieappsv1 "dies.dev/apis/apps/v1"
	diecorev1 "dies.dev/apis/core/v1"
	diemetav1 "dies.dev/apis/meta/v1"
	"github.com/go-logr/logr"
	"github.com/google/go-cmp/cmp"
	"github.com/vmware-labs/reconciler-runtime/reconcilers"
	rtesting "github.com/vmware-labs/reconciler-runtime/testing"
	"gomodules.xyz/jsonpatch/v2"
	admissionv1 "k8s.io/api/admission/v1"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	authorizationv1 "k8s.io/api/authorization/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	servicebindingv1beta1 "github.com/servicebinding/runtime/apis/v1beta1"
	"github.com/servicebinding/runtime/controllers"
	dieservicebindingv1beta1 "github.com/servicebinding/runtime/dies/v1beta1"
	"github.com/servicebinding/runtime/rbac"
)

func TestAdmissionProjectorReconciler(t *testing.T) {
	name := "my-webhook"
	key := types.NamespacedName{Name: name}

	scheme := runtime.NewScheme()
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(servicebindingv1beta1.AddToScheme(scheme))

	webhook := dieadmissionregistrationv1.MutatingWebhookConfigurationBlank.
		MetadataDie(func(d *diemetav1.ObjectMetaDie) {
			d.Name(name)
		}).
		WebhookDie("projector.servicebinding.io", func(d *dieadmissionregistrationv1.MutatingWebhookDie) {
			d.ClientConfigDie(func(d *dieadmissionregistrationv1.WebhookClientConfigDie) {
				d.ServiceDie(func(d *dieadmissionregistrationv1.ServiceReferenceDie) {
					d.Namespace("my-system")
					d.Name("my-service")
				})
			})
			d.RulesDie(
				dieadmissionregistrationv1.RuleWithOperationsBlank.
					APIGroups("apps").
					APIVersions("*").
					Resources("deployments").
					Operations(
						admissionregistrationv1.Create,
						admissionregistrationv1.Update,
					),
			)
		})

	serviceBinding := dieservicebindingv1beta1.ServiceBindingBlank.
		MetadataDie(func(d *diemetav1.ObjectMetaDie) {
			d.Namespace("my-namespace")
			d.Name("my-binding")
		}).
		SpecDie(func(d *dieservicebindingv1beta1.ServiceBindingSpecDie) {
			d.ServiceDie(func(d *dieservicebindingv1beta1.ServiceBindingServiceReferenceDie) {
				d.APIVersion("example/v1")
				d.Kind("MyService")
				d.Name("my-service")
			})
			d.WorkloadDie(func(d *dieservicebindingv1beta1.ServiceBindingWorkloadReferenceDie) {
				d.APIVersion("apps/v1")
				d.Kind("Deployment")
				d.Name("my-workload")
			})
		})

	rts := rtesting.ReconcilerTests{
		"in sync": {
			Key: key,
			GivenObjects: []client.Object{
				webhook,
				serviceBinding,
			},
			WithReactors: []rtesting.ReactionFunc{
				allowSelfSubjectAccessReviewFor("apps", "deployments", "update"),
			},
			ExpectCreates: []client.Object{
				selfSubjectAccessReviewFor("apps", "deployments", "update"),
			},
		},
		"update": {
			Key: key,
			GivenObjects: []client.Object{
				webhook.
					WebhookDie("projector.servicebinding.io", func(d *dieadmissionregistrationv1.MutatingWebhookDie) {
						d.Rules()
					}),
				serviceBinding,
			},
			WithReactors: []rtesting.ReactionFunc{
				allowSelfSubjectAccessReviewFor("apps", "deployments", "update"),
			},
			ExpectEvents: []rtesting.Event{
				rtesting.NewEvent(webhook, scheme, corev1.EventTypeNormal, "Updated", "Updated MutatingWebhookConfiguration %q", name),
			},
			ExpectCreates: []client.Object{
				selfSubjectAccessReviewFor("apps", "deployments", "update"),
			},
			ExpectUpdates: []client.Object{
				webhook,
			},
		},
		"ignore other keys": {
			Key: types.NamespacedName{
				Name: "other-webhook",
			},
			GivenObjects: []client.Object{
				webhook.
					WebhookDie("projector.servicebinding.io", func(d *dieadmissionregistrationv1.MutatingWebhookDie) {
						d.Rules()
					}),
				serviceBinding,
			},
		},
		"ignore malformed webhook": {
			Key: key,
			GivenObjects: []client.Object{
				webhook.
					Webhooks(),
				serviceBinding,
			},
			ExpectCreates: []client.Object{
				selfSubjectAccessReviewFor("apps", "deployments", "update"),
			},
		},
	}

	rts.Run(t, scheme, func(t *testing.T, tc *rtesting.ReconcilerTestCase, c reconcilers.Config) reconcile.Reconciler {
		restMapper := c.RESTMapper().(*meta.DefaultRESTMapper)
		restMapper.Add(schema.GroupVersionKind{Group: "apps", Version: "v1", Kind: "Deployment"}, meta.RESTScopeNamespace)
		accessChecker := rbac.NewAccessChecker(c, 0).WithVerb("update")
		return controllers.AdmissionProjectorReconciler(c, name, accessChecker)
	})
}

func TestAdmissionProjectorWebhook(t *testing.T) {
	namespace := "test-namespace"
	name := "my-workload"
	secret := "my-binding"

	scheme := runtime.NewScheme()
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(servicebindingv1beta1.AddToScheme(scheme))

	requestUID := types.UID("9deefaa1-2c90-4f40-9c7b-3f5c1fd75dde")
	bindingUID := types.UID("89deaf20-7bab-4610-81db-6f8c3f7fa51d")

	workload := dieappsv1.DeploymentBlank.
		APIVersion("apps/v1").
		Kind("Deployment").
		MetadataDie(func(d *diemetav1.ObjectMetaDie) {
			d.Namespace(namespace)
			d.Name(name)
		}).
		SpecDie(func(d *dieappsv1.DeploymentSpecDie) {
			d.TemplateDie(func(d *diecorev1.PodTemplateSpecDie) {
				d.SpecDie(func(d *diecorev1.PodSpecDie) {
					d.ContainerDie("workload", func(d *diecorev1.ContainerDie) {
						d.Image("scratch")
					})
				})
			})
		})
	serviceBinding := dieservicebindingv1beta1.ServiceBindingBlank.
		MetadataDie(func(d *diemetav1.ObjectMetaDie) {
			d.Namespace(namespace)
			d.Name(name)
			d.UID(bindingUID)
		}).
		StatusDie(func(d *dieservicebindingv1beta1.ServiceBindingStatusDie) {
			d.BindingDie(func(d *dieservicebindingv1beta1.ServiceBindingSecretReferenceDie) {
				d.Name(secret)
			})
		})

	request := dieadmissionv1.AdmissionRequestBlank.
		UID(requestUID).
		Operation(admissionv1.Create)
	response := dieadmissionv1.AdmissionResponseBlank.
		Allowed(true)

	wts := rtesting.AdmissionWebhookTests{
		"no binding targeting workload": {
			GivenObjects: []client.Object{
				serviceBinding.
					MetadataDie(func(d *diemetav1.ObjectMetaDie) {
						d.Name(fmt.Sprintf("%s-named", name))
					}).
					SpecDie(func(d *dieservicebindingv1beta1.ServiceBindingSpecDie) {
						d.WorkloadDie(func(d *dieservicebindingv1beta1.ServiceBindingWorkloadReferenceDie) {
							d.Name("some-other-workload")
						})
					}),
				serviceBinding.
					MetadataDie(func(d *diemetav1.ObjectMetaDie) {
						d.Name(fmt.Sprintf("%s-selected", name))
					}).
					SpecDie(func(d *dieservicebindingv1beta1.ServiceBindingSpecDie) {
						d.WorkloadDie(func(d *dieservicebindingv1beta1.ServiceBindingWorkloadReferenceDie) {
							d.SelectorDie(func(d *diemetav1.LabelSelectorDie) {
								d.AddMatchLabel("some-other-workload", "true")
							})
						})
					}),
			},
			Request: &admission.Request{
				AdmissionRequest: request.
					Object(workload.DieReleaseRawExtension()).
					DieRelease(),
			},
			ExpectedResponse: admission.Response{
				AdmissionResponse: response.DieRelease(),
			},
		},
		"binding already projected": {
			GivenObjects: []client.Object{
				serviceBinding.SpecDie(func(d *dieservicebindingv1beta1.ServiceBindingSpecDie) {
					d.WorkloadDie(func(d *dieservicebindingv1beta1.ServiceBindingWorkloadReferenceDie) {
						d.APIVersion("apps/v1")
						d.Kind("Deployment")
						d.Name(name)
					})
				}),
			},
			Request: &admission.Request{
				AdmissionRequest: request.
					Object(
						workload.
							SpecDie(func(d *dieappsv1.DeploymentSpecDie) {
								d.TemplateDie(func(d *diecorev1.PodTemplateSpecDie) {
									d.MetadataDie(func(d *diemetav1.ObjectMetaDie) {
										d.AddAnnotation(fmt.Sprintf("projector.servicebinding.io/secret-%s", bindingUID), secret)
									})
									d.SpecDie(func(d *diecorev1.PodSpecDie) {
										d.ContainerDie("workload", func(d *diecorev1.ContainerDie) {
											d.EnvDie("SERVICE_BINDING_ROOT", func(d *diecorev1.EnvVarDie) {
												d.Value("/bindings")
											})
											d.VolumeMountDie(fmt.Sprintf("servicebinding-%s", bindingUID), func(d *diecorev1.VolumeMountDie) {
												d.MountPath(fmt.Sprintf("/bindings/%s", name))
												d.ReadOnly(true)
											})
										})
										d.VolumeDie(fmt.Sprintf("servicebinding-%s", bindingUID), func(d *diecorev1.VolumeDie) {
											d.ProjectedDie(func(d *diecorev1.ProjectedVolumeSourceDie) {
												d.SourcesDie(
													diecorev1.VolumeProjectionBlank.SecretDie(func(d *diecorev1.SecretProjectionDie) {
														d.LocalObjectReference(corev1.LocalObjectReference{
															Name: secret,
														})
													}),
												)
											})
										})
									})
								})
							}).
							DieReleaseRawExtension(),
					).
					DieRelease(),
			},
			ExpectedResponse: admission.Response{
				AdmissionResponse: response.DieRelease(),
			},
		},
		"binding projected by name": {
			GivenObjects: []client.Object{
				serviceBinding.SpecDie(func(d *dieservicebindingv1beta1.ServiceBindingSpecDie) {
					d.WorkloadDie(func(d *dieservicebindingv1beta1.ServiceBindingWorkloadReferenceDie) {
						d.APIVersion("apps/v1")
						d.Kind("Deployment")
						d.Name(name)
					})
				}),
			},
			Request: &admission.Request{
				AdmissionRequest: request.
					Object(workload.DieReleaseRawExtension()).
					DieRelease(),
			},
			ExpectedResponse: admission.Response{
				AdmissionResponse: response.DieRelease(),
				Patches: []jsonpatch.Operation{
					{
						Operation: "add",
						Path:      "/spec/template/metadata/annotations",
						Value: map[string]interface{}{
							fmt.Sprintf("projector.servicebinding.io/secret-%s", bindingUID): secret,
						},
					},
					{
						Operation: "add",
						Path:      "/spec/template/spec/containers/0/env",
						Value: []interface{}{
							map[string]interface{}{
								"name":  "SERVICE_BINDING_ROOT",
								"value": "/bindings",
							},
						},
					},
					{
						Operation: "add",
						Path:      "/spec/template/spec/containers/0/volumeMounts",
						Value: []interface{}{
							map[string]interface{}{
								"name":      fmt.Sprintf("servicebinding-%s", bindingUID),
								"mountPath": "/bindings/my-workload",
								"readOnly":  true,
							},
						},
					},
					{
						Operation: "add",
						Path:      "/spec/template/spec/volumes",
						Value: []interface{}{
							map[string]interface{}{
								"name": fmt.Sprintf("servicebinding-%s", bindingUID),
								"projected": map[string]interface{}{
									"sources": []interface{}{
										map[string]interface{}{
											"secret": map[string]interface{}{
												"name": secret,
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
		"binding projected by selector": {
			GivenObjects: []client.Object{
				serviceBinding.SpecDie(func(d *dieservicebindingv1beta1.ServiceBindingSpecDie) {
					d.WorkloadDie(func(d *dieservicebindingv1beta1.ServiceBindingWorkloadReferenceDie) {
						d.APIVersion("apps/v1")
						d.Kind("Deployment")
						d.Selector(&metav1.LabelSelector{})
					})
				}),
			},
			Request: &admission.Request{
				AdmissionRequest: request.
					Object(workload.DieReleaseRawExtension()).
					DieRelease(),
			},
			ExpectedResponse: admission.Response{
				AdmissionResponse: response.DieRelease(),
				Patches: []jsonpatch.Operation{
					{
						Operation: "add",
						Path:      "/spec/template/metadata/annotations",
						Value: map[string]interface{}{
							fmt.Sprintf("projector.servicebinding.io/secret-%s", bindingUID): secret,
						},
					},
					{
						Operation: "add",
						Path:      "/spec/template/spec/containers/0/env",
						Value: []interface{}{
							map[string]interface{}{
								"name":  "SERVICE_BINDING_ROOT",
								"value": "/bindings",
							},
						},
					},
					{
						Operation: "add",
						Path:      "/spec/template/spec/containers/0/volumeMounts",
						Value: []interface{}{
							map[string]interface{}{
								"name":      fmt.Sprintf("servicebinding-%s", bindingUID),
								"mountPath": "/bindings/my-workload",
								"readOnly":  true,
							},
						},
					},
					{
						Operation: "add",
						Path:      "/spec/template/spec/volumes",
						Value: []interface{}{
							map[string]interface{}{
								"name": fmt.Sprintf("servicebinding-%s", bindingUID),
								"projected": map[string]interface{}{
									"sources": []interface{}{
										map[string]interface{}{
											"secret": map[string]interface{}{
												"name": secret,
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
		"ingore terminating bindings": {
			GivenObjects: []client.Object{
				serviceBinding.
					MetadataDie(func(d *diemetav1.ObjectMetaDie) {
						now := metav1.Now()
						d.DeletionTimestamp(&now)
					}).
					SpecDie(func(d *dieservicebindingv1beta1.ServiceBindingSpecDie) {
						d.WorkloadDie(func(d *dieservicebindingv1beta1.ServiceBindingWorkloadReferenceDie) {
							d.APIVersion("apps/v1")
							d.Kind("Deployment")
							d.Name(name)
						})
					}),
			},
			Request: &admission.Request{
				AdmissionRequest: request.
					Object(workload.DieReleaseRawExtension()).
					DieRelease(),
			},
			ExpectedResponse: admission.Response{
				AdmissionResponse: response.DieRelease(),
			},
		},
		"error loading bindings": {
			WithReactors: []rtesting.ReactionFunc{
				rtesting.InduceFailure("list", "ServiceBindingList"),
			},
			Request: &admission.Request{
				AdmissionRequest: request.
					Object(workload.DieReleaseRawExtension()).
					DieRelease(),
			},
			ExpectedResponse: admission.Response{
				AdmissionResponse: response.
					Allowed(false).
					ResultDie(func(d *diemetav1.StatusDie) {
						d.Code(500)
						d.Message("inducing failure for list ServiceBindingList")
					}).
					DieRelease(),
			},
		},
	}
	wts.Run(t, scheme, func(t *testing.T, tc *rtesting.AdmissionWebhookTestCase, c reconcilers.Config) *admission.Webhook {
		restMapper := c.RESTMapper().(*meta.DefaultRESTMapper)
		restMapper.Add(schema.GroupVersionKind{Group: "apps", Version: "v1", Kind: "Deployment"}, meta.RESTScopeNamespace)
		return controllers.AdmissionProjectorWebhook(c).Build()
	})
}

func TestTriggerReconciler(t *testing.T) {
	name := "my-webhook"
	key := types.NamespacedName{Name: name}

	scheme := runtime.NewScheme()
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(servicebindingv1beta1.AddToScheme(scheme))

	webhook := dieadmissionregistrationv1.ValidatingWebhookConfigurationBlank.
		MetadataDie(func(d *diemetav1.ObjectMetaDie) {
			d.Name(name)
		}).
		WebhookDie("trigger.servicebinding.io", func(d *dieadmissionregistrationv1.ValidatingWebhookDie) {
			d.ClientConfigDie(func(d *dieadmissionregistrationv1.WebhookClientConfigDie) {
				d.ServiceDie(func(d *dieadmissionregistrationv1.ServiceReferenceDie) {
					d.Namespace("my-system")
					d.Name("my-service")
				})
			})
			d.RulesDie(
				dieadmissionregistrationv1.RuleWithOperationsBlank.
					APIGroups("apps").
					APIVersions("*").
					Resources("deployments").
					Operations(
						admissionregistrationv1.Create,
						admissionregistrationv1.Update,
						admissionregistrationv1.Delete,
					),
				dieadmissionregistrationv1.RuleWithOperationsBlank.
					APIGroups("example").
					APIVersions("*").
					Resources("myservices").
					Operations(
						admissionregistrationv1.Create,
						admissionregistrationv1.Update,
						admissionregistrationv1.Delete,
					),
			)
		})

	serviceBinding := dieservicebindingv1beta1.ServiceBindingBlank.
		MetadataDie(func(d *diemetav1.ObjectMetaDie) {
			d.Namespace("my-namespace")
			d.Name("my-binding")
		}).
		SpecDie(func(d *dieservicebindingv1beta1.ServiceBindingSpecDie) {
			d.ServiceDie(func(d *dieservicebindingv1beta1.ServiceBindingServiceReferenceDie) {
				d.APIVersion("example/v1")
				d.Kind("MyService")
				d.Name("my-service")
			})
			d.WorkloadDie(func(d *dieservicebindingv1beta1.ServiceBindingWorkloadReferenceDie) {
				d.APIVersion("apps/v1")
				d.Kind("Deployment")
				d.Name("my-workload")
			})
		})

	rts := rtesting.ReconcilerTests{
		"in sync": {
			Key: key,
			GivenObjects: []client.Object{
				webhook,
				serviceBinding,
			},
			WithReactors: []rtesting.ReactionFunc{
				allowSelfSubjectAccessReviewFor("apps", "deployments", "get"),
				allowSelfSubjectAccessReviewFor("example", "myservices", "get"),
			},
			ExpectCreates: []client.Object{
				selfSubjectAccessReviewFor("apps", "deployments", "get"),
				selfSubjectAccessReviewFor("example", "myservices", "get"),
			},
		},
		"update": {
			Key: key,
			GivenObjects: []client.Object{
				webhook.
					WebhookDie("trigger.servicebinding.io", func(d *dieadmissionregistrationv1.ValidatingWebhookDie) {
						d.Rules()
					}),
				serviceBinding,
			},
			WithReactors: []rtesting.ReactionFunc{
				allowSelfSubjectAccessReviewFor("apps", "deployments", "get"),
				allowSelfSubjectAccessReviewFor("example", "myservices", "get"),
			},
			ExpectEvents: []rtesting.Event{
				rtesting.NewEvent(webhook, scheme, corev1.EventTypeNormal, "Updated", "Updated ValidatingWebhookConfiguration %q", name),
			},
			ExpectCreates: []client.Object{
				selfSubjectAccessReviewFor("apps", "deployments", "get"),
				selfSubjectAccessReviewFor("example", "myservices", "get"),
			},
			ExpectUpdates: []client.Object{
				webhook,
			},
		},
		"ignore other keys": {
			Key: types.NamespacedName{
				Name: "other-webhook",
			},
			GivenObjects: []client.Object{
				webhook.
					WebhookDie("trigger.servicebinding.io", func(d *dieadmissionregistrationv1.ValidatingWebhookDie) {
						d.Rules()
					}),
				serviceBinding,
			},
		},
		"ignore malformed webhook": {
			Key: key,
			GivenObjects: []client.Object{
				webhook.
					Webhooks(),
				serviceBinding,
			},
			WithReactors: []rtesting.ReactionFunc{
				allowSelfSubjectAccessReviewFor("apps", "deployments", "get"),
				allowSelfSubjectAccessReviewFor("example", "myservices", "get"),
			},
			ExpectCreates: []client.Object{
				selfSubjectAccessReviewFor("apps", "deployments", "get"),
				selfSubjectAccessReviewFor("example", "myservices", "get"),
			},
		},
	}

	rts.Run(t, scheme, func(t *testing.T, tc *rtesting.ReconcilerTestCase, c reconcilers.Config) reconcile.Reconciler {
		restMapper := c.RESTMapper().(*meta.DefaultRESTMapper)
		restMapper.Add(schema.GroupVersionKind{Group: "apps", Version: "v1", Kind: "Deployment"}, meta.RESTScopeNamespace)
		restMapper.Add(schema.GroupVersionKind{Group: "example", Version: "v1", Kind: "MyService"}, meta.RESTScopeNamespace)
		accessChecker := rbac.NewAccessChecker(c, 0).WithVerb("get")
		return controllers.TriggerReconciler(c, name, accessChecker)
	})
}

func TestTriggerWebhook(t *testing.T) {
	namespace := "test-namespace"
	name := "my-workload"
	bindingName := "my-binding"

	scheme := runtime.NewScheme()
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(servicebindingv1beta1.AddToScheme(scheme))

	requestUID := types.UID("9deefaa1-2c90-4f40-9c7b-3f5c1fd75dde")

	serviceBinding := dieservicebindingv1beta1.ServiceBindingBlank.
		MetadataDie(func(d *diemetav1.ObjectMetaDie) {
			d.Namespace(namespace)
			d.Name(bindingName)
		})

	workload := dieappsv1.DeploymentBlank.
		APIVersion("apps/v1").
		Kind("Deployment").
		MetadataDie(func(d *diemetav1.ObjectMetaDie) {
			d.Namespace(namespace)
			d.Name(name)
		})

	request := dieadmissionv1.AdmissionRequestBlank.
		UID(requestUID).
		Operation(admissionv1.Create)
	response := dieadmissionv1.AdmissionResponseBlank.
		Allowed(true)

	wts := rtesting.AdmissionWebhookTests{
		"no op, nil workqueue": {
			Request: &admission.Request{
				AdmissionRequest: request.
					Object(workload.DieReleaseRawExtension()).
					DieRelease(),
			},
			ExpectedResponse: admission.Response{
				AdmissionResponse: response.DieRelease(),
			},
		},
		"nothing to enqueue": {
			Request: &admission.Request{
				AdmissionRequest: request.
					Object(workload.DieReleaseRawExtension()).
					DieRelease(),
			},
			ExpectedResponse: admission.Response{
				AdmissionResponse: response.DieRelease(),
			},
			Metadata: map[string]interface{}{
				"queue":            workqueue.New(),
				"expectedRequests": []reconcile.Request{},
			},
		},
		"enqueue tracked": {
			Request: &admission.Request{
				AdmissionRequest: request.
					Object(workload.DieReleaseRawExtension()).
					DieRelease(),
			},
			GivenTracks: []rtesting.TrackRequest{
				rtesting.NewTrackRequest(serviceBinding, workload, scheme),
			},
			ExpectedResponse: admission.Response{
				AdmissionResponse: response.DieRelease(),
			},
			Metadata: map[string]interface{}{
				"queue": workqueue.New(),
				"expectedRequests": []reconcile.Request{
					{NamespacedName: types.NamespacedName{Namespace: namespace, Name: bindingName}},
				},
			},
		},
	}
	wts.Run(t, scheme, func(t *testing.T, tc *rtesting.AdmissionWebhookTestCase, c reconcilers.Config) *admission.Webhook {
		if tc.Metadata == nil {
			tc.Metadata = map[string]interface{}{}
		}
		tc.CleanUp = func(t *testing.T, ctx context.Context, tc *rtesting.AdmissionWebhookTestCase) error {
			queue, ok := tc.Metadata["queue"].(workqueue.Interface)
			if !ok {
				return nil
			}
			actualRequests := []reconcile.Request{}
			for len(actualRequests) < queue.Len() {
				request, _ := queue.Get()
				actualRequests = append(actualRequests, request.(reconcile.Request))
			}
			expectedRequests := tc.Metadata["expectedRequests"].([]reconcile.Request)
			if diff := cmp.Diff(expectedRequests, actualRequests); diff != "" {
				t.Errorf("enqueued request (-expected, +actual): %s", diff)
			}
			return nil
		}

		queue, _ := tc.Metadata["queue"].(workqueue.Interface)
		ctrl := &mockController{
			Queue: queue,
		}
		return controllers.TriggerWebhook(c, ctrl).Build()
	})
}

func TestLoadServiceBindings(t *testing.T) {
	scheme := runtime.NewScheme()
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(servicebindingv1beta1.AddToScheme(scheme))

	webhook := dieadmissionregistrationv1.ValidatingWebhookConfigurationBlank

	serviceBinding := dieservicebindingv1beta1.ServiceBindingBlank.
		MetadataDie(func(d *diemetav1.ObjectMetaDie) {
			d.Namespace("my-namespace")
			d.Name("my-binding")
		}).
		SpecDie(func(d *dieservicebindingv1beta1.ServiceBindingSpecDie) {
			d.ServiceDie(func(d *dieservicebindingv1beta1.ServiceBindingServiceReferenceDie) {
				d.APIVersion("example/v1")
				d.Kind("MyService")
				d.Name("my-service")
			})
			d.WorkloadDie(func(d *dieservicebindingv1beta1.ServiceBindingWorkloadReferenceDie) {
				d.APIVersion("apps/v1")
				d.Kind("Deployment")
				d.Name("my-workload")
			})
		})

	rts := rtesting.SubReconcilerTests{
		"list all servicebindings": {
			Resource: webhook,
			GivenObjects: []client.Object{
				serviceBinding,
			},
			ExpectStashedValues: map[reconcilers.StashKey]interface{}{
				controllers.ServiceBindingsStashKey: []servicebindingv1beta1.ServiceBinding{
					serviceBinding.DieRelease(),
				},
			},
		},
		"error listing all servicebindings": {
			Resource: webhook,
			GivenObjects: []client.Object{
				serviceBinding,
			},
			WithReactors: []rtesting.ReactionFunc{
				rtesting.InduceFailure("list", "ServiceBindingList"),
			},
			ShouldErr: true,
		},
	}

	rts.Run(t, scheme, func(t *testing.T, tc *rtesting.SubReconcilerTestCase, c reconcilers.Config) reconcilers.SubReconciler {
		req := reconcile.Request{NamespacedName: types.NamespacedName{Name: "my-webhook"}}
		return controllers.LoadServiceBindings(req)
	})
}

func TestInterceptGVKs(t *testing.T) {
	scheme := runtime.NewScheme()
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(servicebindingv1beta1.AddToScheme(scheme))

	webhook := dieadmissionregistrationv1.ValidatingWebhookConfigurationBlank

	serviceBinding := dieservicebindingv1beta1.ServiceBindingBlank.
		MetadataDie(func(d *diemetav1.ObjectMetaDie) {
			d.Namespace("my-namespace")
			d.Name("my-binding")
		}).
		SpecDie(func(d *dieservicebindingv1beta1.ServiceBindingSpecDie) {
			d.ServiceDie(func(d *dieservicebindingv1beta1.ServiceBindingServiceReferenceDie) {
				d.APIVersion("example/v1")
				d.Kind("MyService")
				d.Name("my-service")
			})
			d.WorkloadDie(func(d *dieservicebindingv1beta1.ServiceBindingWorkloadReferenceDie) {
				d.APIVersion("apps/v1")
				d.Kind("Deployment")
				d.Name("my-workload")
			})
		})

	rts := rtesting.SubReconcilerTests{
		"collect workload gvks": {
			Resource: webhook,
			GivenStashedValues: map[reconcilers.StashKey]interface{}{
				controllers.ServiceBindingsStashKey: []servicebindingv1beta1.ServiceBinding{
					serviceBinding.DieRelease(),
				},
			},
			ExpectStashedValues: map[reconcilers.StashKey]interface{}{
				controllers.ObservedGVKsStashKey: []schema.GroupVersionKind{
					{Group: "apps", Version: "v1", Kind: "Deployment"},
				},
			},
		},
		"append workload gvks": {
			Resource: webhook,
			GivenStashedValues: map[reconcilers.StashKey]interface{}{
				controllers.ServiceBindingsStashKey: []servicebindingv1beta1.ServiceBinding{
					serviceBinding.DieRelease(),
				},
				controllers.ObservedGVKsStashKey: []schema.GroupVersionKind{
					{Group: "example", Version: "v1", Kind: "MyService"},
				},
			},
			ExpectStashedValues: map[reconcilers.StashKey]interface{}{
				controllers.ObservedGVKsStashKey: []schema.GroupVersionKind{
					{Group: "example", Version: "v1", Kind: "MyService"},
					{Group: "apps", Version: "v1", Kind: "Deployment"},
				},
			},
		},
	}

	rts.Run(t, scheme, func(t *testing.T, tc *rtesting.SubReconcilerTestCase, c reconcilers.Config) reconcilers.SubReconciler {
		return controllers.InterceptGVKs()
	})
}

func TestTriggerGVKs(t *testing.T) {
	scheme := runtime.NewScheme()
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(servicebindingv1beta1.AddToScheme(scheme))

	webhook := dieadmissionregistrationv1.ValidatingWebhookConfigurationBlank

	serviceBinding := dieservicebindingv1beta1.ServiceBindingBlank.
		MetadataDie(func(d *diemetav1.ObjectMetaDie) {
			d.Namespace("my-namespace")
			d.Name("my-binding")
		}).
		SpecDie(func(d *dieservicebindingv1beta1.ServiceBindingSpecDie) {
			d.ServiceDie(func(d *dieservicebindingv1beta1.ServiceBindingServiceReferenceDie) {
				d.APIVersion("example/v1")
				d.Kind("MyService")
				d.Name("my-service")
			})
			d.WorkloadDie(func(d *dieservicebindingv1beta1.ServiceBindingWorkloadReferenceDie) {
				d.APIVersion("apps/v1")
				d.Kind("Deployment")
				d.Name("my-workload")
			})
		})

	rts := rtesting.SubReconcilerTests{
		"collect service gvks": {
			Resource: webhook,
			GivenStashedValues: map[reconcilers.StashKey]interface{}{
				controllers.ServiceBindingsStashKey: []servicebindingv1beta1.ServiceBinding{
					serviceBinding.DieRelease(),
				},
			},
			ExpectStashedValues: map[reconcilers.StashKey]interface{}{
				controllers.ObservedGVKsStashKey: []schema.GroupVersionKind{
					{Group: "example", Version: "v1", Kind: "MyService"},
				},
			},
		},
		"append service gvks": {
			Resource: webhook,
			GivenStashedValues: map[reconcilers.StashKey]interface{}{
				controllers.ServiceBindingsStashKey: []servicebindingv1beta1.ServiceBinding{
					serviceBinding.DieRelease(),
				},
				controllers.ObservedGVKsStashKey: []schema.GroupVersionKind{
					{Group: "apps", Version: "v1", Kind: "Deployment"},
				},
			},
			ExpectStashedValues: map[reconcilers.StashKey]interface{}{
				controllers.ObservedGVKsStashKey: []schema.GroupVersionKind{
					{Group: "apps", Version: "v1", Kind: "Deployment"},
					{Group: "example", Version: "v1", Kind: "MyService"},
				},
			},
		},
		"ignore direct binding": {
			Resource: webhook,
			GivenStashedValues: map[reconcilers.StashKey]interface{}{
				controllers.ServiceBindingsStashKey: []servicebindingv1beta1.ServiceBinding{
					serviceBinding.
						SpecDie(func(d *dieservicebindingv1beta1.ServiceBindingSpecDie) {
							d.ServiceDie(func(d *dieservicebindingv1beta1.ServiceBindingServiceReferenceDie) {
								d.APIVersion("v1")
								d.Kind("Secret")
							})
						}).
						DieRelease(),
				},
			},
			ExpectStashedValues: map[reconcilers.StashKey]interface{}{
				controllers.ObservedGVKsStashKey: []schema.GroupVersionKind{},
			},
		},
	}

	rts.Run(t, scheme, func(t *testing.T, tc *rtesting.SubReconcilerTestCase, c reconcilers.Config) reconcilers.SubReconciler {
		return controllers.TriggerGVKs()
	})
}

func TestWebhookRules(t *testing.T) {
	scheme := runtime.NewScheme()
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(servicebindingv1beta1.AddToScheme(scheme))

	webhook := dieadmissionregistrationv1.ValidatingWebhookConfigurationBlank

	operations := []admissionregistrationv1.OperationType{
		admissionregistrationv1.Connect,
	}

	rts := rtesting.SubReconcilerTests{
		"empty": {
			Resource: webhook,
			GivenStashedValues: map[reconcilers.StashKey]interface{}{
				controllers.ObservedGVKsStashKey: []schema.GroupVersionKind{},
			},
			ExpectStashedValues: map[reconcilers.StashKey]interface{}{
				controllers.WebhookRulesStashKey: []admissionregistrationv1.RuleWithOperations{},
			},
		},
		"convert": {
			Resource: webhook,
			GivenStashedValues: map[reconcilers.StashKey]interface{}{
				controllers.ObservedGVKsStashKey: []schema.GroupVersionKind{
					{Group: "apps", Version: "v1", Kind: "Deployment"},
				},
			},
			WithReactors: []rtesting.ReactionFunc{
				allowSelfSubjectAccessReviewFor("apps", "deployments", "get"),
			},
			ExpectStashedValues: map[reconcilers.StashKey]interface{}{
				controllers.WebhookRulesStashKey: []admissionregistrationv1.RuleWithOperations{
					{
						Operations: operations,
						Rule: admissionregistrationv1.Rule{
							APIGroups:   []string{"apps"},
							APIVersions: []string{"*"},
							Resources:   []string{"deployments"},
						},
					},
				},
			},
			ExpectCreates: []client.Object{
				selfSubjectAccessReviewFor("apps", "deployments", "get"),
			},
		},
		"dedup versions": {
			Resource: webhook,
			GivenStashedValues: map[reconcilers.StashKey]interface{}{
				controllers.ObservedGVKsStashKey: []schema.GroupVersionKind{
					{Group: "apps", Version: "v1", Kind: "Deployment"},
					{Group: "apps", Version: "v1beta1", Kind: "Deployment"},
				},
			},
			WithReactors: []rtesting.ReactionFunc{
				allowSelfSubjectAccessReviewFor("apps", "deployments", "get"),
			},
			ExpectStashedValues: map[reconcilers.StashKey]interface{}{
				controllers.WebhookRulesStashKey: []admissionregistrationv1.RuleWithOperations{
					{
						Operations: operations,
						Rule: admissionregistrationv1.Rule{
							APIGroups:   []string{"apps"},
							APIVersions: []string{"*"},
							Resources:   []string{"deployments"},
						},
					},
				},
			},
			ExpectCreates: []client.Object{
				selfSubjectAccessReviewFor("apps", "deployments", "get"),
			},
		},
		"merge resources of same group": {
			Resource: webhook,
			GivenStashedValues: map[reconcilers.StashKey]interface{}{
				controllers.ObservedGVKsStashKey: []schema.GroupVersionKind{
					{Group: "apps", Version: "v1", Kind: "StatefulSet"},
					{Group: "apps", Version: "v1", Kind: "Deployment"},
				},
			},
			WithReactors: []rtesting.ReactionFunc{
				allowSelfSubjectAccessReviewFor("apps", "deployments", "get"),
				allowSelfSubjectAccessReviewFor("apps", "statefulsets", "get"),
			},
			ExpectStashedValues: map[reconcilers.StashKey]interface{}{
				controllers.WebhookRulesStashKey: []admissionregistrationv1.RuleWithOperations{
					{
						Operations: operations,
						Rule: admissionregistrationv1.Rule{
							APIGroups:   []string{"apps"},
							APIVersions: []string{"*"},
							Resources:   []string{"deployments", "statefulsets"},
						},
					},
				},
			},
			ExpectCreates: []client.Object{
				selfSubjectAccessReviewFor("apps", "deployments", "get"),
				selfSubjectAccessReviewFor("apps", "statefulsets", "get"),
			},
		},
		"preserve resources of different group": {
			Resource: webhook,
			GivenStashedValues: map[reconcilers.StashKey]interface{}{
				controllers.ObservedGVKsStashKey: []schema.GroupVersionKind{
					{Group: "batch", Version: "v1", Kind: "Job"},
					{Group: "apps", Version: "v1", Kind: "Deployment"},
				},
			},
			WithReactors: []rtesting.ReactionFunc{
				allowSelfSubjectAccessReviewFor("apps", "deployments", "get"),
				allowSelfSubjectAccessReviewFor("batch", "jobs", "get"),
			},
			ExpectStashedValues: map[reconcilers.StashKey]interface{}{
				controllers.WebhookRulesStashKey: []admissionregistrationv1.RuleWithOperations{
					{
						Operations: operations,
						Rule: admissionregistrationv1.Rule{
							APIGroups:   []string{"apps"},
							APIVersions: []string{"*"},
							Resources:   []string{"deployments"},
						},
					},
					{
						Operations: operations,
						Rule: admissionregistrationv1.Rule{
							APIGroups:   []string{"batch"},
							APIVersions: []string{"*"},
							Resources:   []string{"jobs"},
						},
					},
				},
			},
			ExpectCreates: []client.Object{
				selfSubjectAccessReviewFor("apps", "deployments", "get"),
				selfSubjectAccessReviewFor("batch", "jobs", "get"),
			},
		},
		"error on unknown resource": {
			Resource: webhook,
			GivenStashedValues: map[reconcilers.StashKey]interface{}{
				controllers.ObservedGVKsStashKey: []schema.GroupVersionKind{
					{Group: "foo", Version: "v1", Kind: "Bar"},
				},
			},
			ShouldErr: true,
		},
		"drop denied resources": {
			Resource: webhook,
			GivenStashedValues: map[reconcilers.StashKey]interface{}{
				controllers.ObservedGVKsStashKey: []schema.GroupVersionKind{
					{Group: "apps", Version: "v1", Kind: "Deployment"},
				},
			},
			ExpectStashedValues: map[reconcilers.StashKey]interface{}{
				controllers.WebhookRulesStashKey: []admissionregistrationv1.RuleWithOperations{},
			},
			ExpectCreates: []client.Object{
				selfSubjectAccessReviewFor("apps", "deployments", "get"),
			},
		},
		"treat SelfSubjectAccessReview errors as denied": {
			Resource: webhook,
			GivenStashedValues: map[reconcilers.StashKey]interface{}{
				controllers.ObservedGVKsStashKey: []schema.GroupVersionKind{
					{Group: "apps", Version: "v1", Kind: "Deployment"},
				},
			},
			WithReactors: []rtesting.ReactionFunc{
				rtesting.InduceFailure("create", "SelfSubjectAccessReview"),
			},
			ExpectStashedValues: map[reconcilers.StashKey]interface{}{
				controllers.WebhookRulesStashKey: []admissionregistrationv1.RuleWithOperations{},
			},
			ExpectCreates: []client.Object{
				selfSubjectAccessReviewFor("apps", "deployments", "get"),
			},
		},
	}

	rts.Run(t, scheme, func(t *testing.T, tc *rtesting.SubReconcilerTestCase, c reconcilers.Config) reconcilers.SubReconciler {
		restMapper := c.RESTMapper().(*meta.DefaultRESTMapper)
		restMapper.Add(schema.GroupVersionKind{Group: "apps", Version: "v1", Kind: "Deployment"}, meta.RESTScopeNamespace)
		restMapper.Add(schema.GroupVersionKind{Group: "apps", Version: "v1beta1", Kind: "Deployment"}, meta.RESTScopeNamespace)
		restMapper.Add(schema.GroupVersionKind{Group: "apps", Version: "v1", Kind: "StatefulSet"}, meta.RESTScopeNamespace)
		restMapper.Add(schema.GroupVersionKind{Group: "batch", Version: "v1", Kind: "Job"}, meta.RESTScopeNamespace)
		accessChecker := rbac.NewAccessChecker(c, 0).WithVerb("get")
		return controllers.WebhookRules(operations, accessChecker)
	})
}

func selfSubjectAccessReviewFor(group, resource, verb string) *authorizationv1.SelfSubjectAccessReview {
	return &authorizationv1.SelfSubjectAccessReview{
		Spec: authorizationv1.SelfSubjectAccessReviewSpec{
			ResourceAttributes: &authorizationv1.ResourceAttributes{
				Group:    group,
				Resource: resource,
				Verb:     verb,
			},
		},
	}
}

func allowSelfSubjectAccessReviewFor(group, resource, verb string) rtesting.ReactionFunc {
	return func(action rtesting.Action) (handled bool, ret runtime.Object, err error) {
		r := action.GetResource()
		if r.Group != "authorization.k8s.io" || r.Resource != "SelfSubjectAccessReview" || r.Version != "v1" || action.GetVerb() != "create" {
			// ignore, not creating a SelfSubjectAccessReview
			return false, nil, nil
		}
		ssar := action.(rtesting.CreateAction).GetObject().(*authorizationv1.SelfSubjectAccessReview)
		if ra := ssar.Spec.ResourceAttributes; ra != nil {
			if ra.Group == group && ra.Resource == resource && ra.Verb == verb {
				ssar.Status.Allowed = true
				return true, ssar, nil
			}
		}
		return false, nil, nil
	}
}

var _ controller.Controller = (*mockController)(nil)

type mockController struct {
	Queue workqueue.Interface
}

func (c *mockController) Reconcile(context.Context, reconcile.Request) (reconcile.Result, error) {
	return reconcile.Result{}, nil
}

func (c *mockController) Watch(src source.Source, eventhandler handler.EventHandler, predicates ...predicate.Predicate) error {
	return nil
}

func (c *mockController) Start(ctx context.Context) error {
	return nil
}

func (c *mockController) GetLogger() logr.Logger {
	return logr.Discard()
}

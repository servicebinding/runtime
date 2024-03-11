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

package lifecycle_test

import (
	"context"
	"fmt"
	"testing"

	dieappsv1 "dies.dev/apis/apps/v1"
	diecorev1 "dies.dev/apis/core/v1"
	diemetav1 "dies.dev/apis/meta/v1"
	"github.com/stretchr/testify/mock"
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
	"k8s.io/utils/pointer"
	"reconciler.io/runtime/reconcilers"
	rtesting "reconciler.io/runtime/testing"
	"reconciler.io/runtime/tracker"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	servicebindingv1 "github.com/servicebinding/runtime/apis/v1"
	"github.com/servicebinding/runtime/controllers"
	dieservicebindingv1 "github.com/servicebinding/runtime/dies/v1"
	"github.com/servicebinding/runtime/lifecycle"
	"github.com/servicebinding/runtime/projector"
	"github.com/servicebinding/runtime/resolver"
)

func TestServiceBindingHooks(t *testing.T) {
	namespace := "test-namespace"
	name := "my-binding"
	secretName := "my-secret"

	scheme := runtime.NewScheme()
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(servicebindingv1.AddToScheme(scheme))

	serviceBinding := dieservicebindingv1.ServiceBindingBlank.
		MetadataDie(func(d *diemetav1.ObjectMetaDie) {
			d.Namespace(namespace)
			d.Name(name)
			d.Finalizers("servicebinding.io/finalizer")
			d.UID(uuid.NewUUID())
		}).
		SpecDie(func(d *dieservicebindingv1.ServiceBindingSpecDie) {
			d.ServiceDie(func(d *dieservicebindingv1.ServiceBindingServiceReferenceDie) {
				d.APIVersion("v1")
				d.Kind("Secret")
				d.Name(secretName)
			})
		}).
		StatusDie(func(d *dieservicebindingv1.ServiceBindingStatusDie) {
			d.BindingDie(func(d *dieservicebindingv1.ServiceBindingSecretReferenceDie) {
				d.Name(secretName)
			})
			d.ConditionsDie(
				dieservicebindingv1.ServiceBindingConditionReady.True().Reason("ServiceBound"),
				dieservicebindingv1.ServiceBindingConditionServiceAvailable.True().Reason("ResolvedBindingSecret"),
				dieservicebindingv1.ServiceBindingConditionWorkloadProjected.True().Reason("ResolvedBindingSecret"),
			)
		})
	serviceBindingByName := serviceBinding.
		SpecDie(func(d *dieservicebindingv1.ServiceBindingSpecDie) {
			d.WorkloadDie(func(d *dieservicebindingv1.ServiceBindingWorkloadReferenceDie) {
				d.APIVersion("apps/v1")
				d.Kind("Deployment")
				d.Name(name)
			})
		})
	serviceBindingBySelector := serviceBinding.
		SpecDie(func(d *dieservicebindingv1.ServiceBindingSpecDie) {
			d.WorkloadDie(func(d *dieservicebindingv1.ServiceBindingWorkloadReferenceDie) {
				d.APIVersion("apps/v1")
				d.Kind("Deployment")
				d.SelectorDie(func(d *diemetav1.LabelSelectorDie) {
					d.AddMatchLabel("test.servicebinding.io", "workload")
				})
			})
		})

	workload := dieappsv1.DeploymentBlank.
		APIVersion("apps/v1").
		Kind("Deployment").
		MetadataDie(func(d *diemetav1.ObjectMetaDie) {
			d.Namespace(namespace)
			d.Name(name)
			d.UID(uuid.NewUUID())
		}).
		SpecDie(func(d *dieappsv1.DeploymentSpecDie) {
			d.TemplateDie(func(d *diecorev1.PodTemplateSpecDie) {
				d.SpecDie(func(d *diecorev1.PodSpecDie) {
					d.ContainerDie("workload", func(d *diecorev1.ContainerDie) {
						d.EnvDie("SERVICE_BINDING_ROOT", func(d *diecorev1.EnvVarDie) {
							d.Value("/bindings")
						})
					})
				})
			})
		})

	anyContext := mock.MatchedBy(func(arg interface{}) bool {
		_, ok := arg.(context.Context)
		return ok
	})
	matchObj := func(expected client.Object) interface{} {
		return mock.MatchedBy(func(actual client.Object) bool {
			return expected.GetObjectKind().GroupVersionKind() == actual.GetObjectKind().GroupVersionKind() &&
				expected.GetNamespace() == actual.GetNamespace() &&
				expected.GetName() == actual.GetName() &&
				expected.GetUID() == actual.GetUID()
		})
	}

	t.Run("Controller", func(t *testing.T) {
		workload1 := workload.
			MetadataDie(func(d *diemetav1.ObjectMetaDie) {
				d.Name(fmt.Sprintf("%s-1", workload.GetName()))
				d.UID(uuid.NewUUID())
				d.AddLabel("test.servicebinding.io", "workload")
			})
		workload2 := workload.
			MetadataDie(func(d *diemetav1.ObjectMetaDie) {
				d.Name(fmt.Sprintf("%s-2", workload.GetName()))
				d.UID(uuid.NewUUID())
				d.AddLabel("test.servicebinding.io", "workload")
			})

		rts := rtesting.SubReconcilerTests[*servicebindingv1.ServiceBinding]{
			"controller binding by name": {
				Metadata: map[string]interface{}{
					"HooksExpectations": func(m *mock.Mock) {
						m.On("ServiceBindingPreProjection", 1, anyContext, matchObj(serviceBinding.DieReleasePtr())).Return(nil).Once()
						m.On("WorkloadPreProjection", 2, anyContext, matchObj(workload.DieReleaseUnstructured())).Return(nil).Once()
						m.On("Projector.Project", 3, anyContext, matchObj(serviceBinding.DieReleasePtr()), matchObj(workload.DieReleaseUnstructured())).Return(nil).Once()
						m.On("WorkloadPostProjection", 4, anyContext, matchObj(workload.DieReleaseUnstructured())).Return(nil).Once()
						m.On("ServiceBindingPostProjection", 5, anyContext, matchObj(serviceBinding.DieReleasePtr())).Return(nil).Once()
					},
				},
				CleanUp: func(t *testing.T, ctx context.Context, tc *rtesting.SubReconcilerTestCase[*servicebindingv1.ServiceBinding]) error {
					m := tc.Metadata["HooksMock"].(*mock.Mock)
					m.AssertExpectations(t)
					return nil
				},
				Resource: serviceBindingByName.DieReleasePtr(),
				GivenObjects: []client.Object{
					workload.DieReleasePtr(),
				},
				ExpectTracks: []rtesting.TrackRequest{
					rtesting.NewTrackRequest(workload, serviceBinding, scheme),
				},
			},
			"controller binding by selector": {
				Metadata: map[string]interface{}{
					"HooksExpectations": func(m *mock.Mock) {
						m.On("ServiceBindingPreProjection", 1, anyContext, matchObj(serviceBinding.DieReleasePtr())).Return(nil).Once()
						m.On("WorkloadPreProjection", 2, anyContext, matchObj(workload1.DieReleaseUnstructured())).Return(nil).Once()
						m.On("Projector.Project", 3, anyContext, matchObj(serviceBinding.DieReleasePtr()), matchObj(workload1.DieReleaseUnstructured())).Return(nil).Once()
						m.On("WorkloadPostProjection", 4, anyContext, matchObj(workload1.DieReleaseUnstructured())).Return(nil).Once()
						m.On("WorkloadPreProjection", 5, anyContext, matchObj(workload2.DieReleaseUnstructured())).Return(nil).Once()
						m.On("Projector.Project", 6, anyContext, matchObj(serviceBinding.DieReleasePtr()), matchObj(workload2.DieReleaseUnstructured())).Return(nil).Once()
						m.On("WorkloadPostProjection", 7, anyContext, matchObj(workload2.DieReleaseUnstructured())).Return(nil).Once()
						m.On("ServiceBindingPostProjection", 8, anyContext, matchObj(serviceBinding.DieReleasePtr())).Return(nil).Once()
					},
				},
				CleanUp: func(t *testing.T, ctx context.Context, tc *rtesting.SubReconcilerTestCase[*servicebindingv1.ServiceBinding]) error {
					m := tc.Metadata["HooksMock"].(*mock.Mock)
					m.AssertExpectations(t)
					return nil
				},
				Resource: serviceBindingBySelector.DieReleasePtr(),
				GivenObjects: []client.Object{
					workload1.DieReleasePtr(),
					workload2.DieReleasePtr(),
				},
				ExpectTracks: []rtesting.TrackRequest{
					{
						Tracker: types.NamespacedName{Namespace: namespace, Name: name},
						TrackedReference: tracker.Reference{
							APIGroup:  "apps",
							Kind:      "Deployment",
							Namespace: namespace,
							Selector: labels.SelectorFromSet(labels.Set{
								"test.servicebinding.io": "workload",
							}),
						},
					},
				},
			},
		}

		rts.Run(t, scheme, func(t *testing.T, rtc *rtesting.SubReconcilerTestCase[*servicebindingv1.ServiceBinding], c reconcilers.Config) reconcilers.SubReconciler[*servicebindingv1.ServiceBinding] {
			restMapper := c.RESTMapper().(*meta.DefaultRESTMapper)
			restMapper.Add(schema.GroupVersionKind{Group: "apps", Version: "v1", Kind: "Deployment"}, meta.RESTScopeNamespace)
			hooks, m := makeHooks()
			rtc.Metadata["HooksMock"] = m
			rtc.Metadata["HooksExpectations"].(func(*mock.Mock))(m)
			return controllers.ServiceBindingReconciler(c, hooks).Reconciler
		})
	})

	t.Run("Webhook", func(t *testing.T) {
		serviceBinding1 := serviceBindingByName.
			MetadataDie(func(d *diemetav1.ObjectMetaDie) {
				d.Name(fmt.Sprintf("%s-1", serviceBinding.GetName()))
				d.UID(uuid.NewUUID())
			})
		serviceBinding2 := serviceBindingByName.
			MetadataDie(func(d *diemetav1.ObjectMetaDie) {
				d.Name(fmt.Sprintf("%s-2", serviceBinding.GetName()))
				d.UID(uuid.NewUUID())
			})

		addWorkloadRefIndex := func(cb *fake.ClientBuilder) *fake.ClientBuilder {
			return cb.WithIndex(&servicebindingv1.ServiceBinding{}, controllers.WorkloadRefIndexKey, controllers.WorkloadRefIndexFunc)
		}

		rts := rtesting.SubReconcilerTests[*unstructured.Unstructured]{
			"webhook": {
				Metadata: map[string]interface{}{
					"HooksExpectations": func(m *mock.Mock) {
						m.On("WorkloadPreProjection", 1, anyContext, matchObj(workload.DieReleaseUnstructured())).Return(nil).Once()
						m.On("ServiceBindingPreProjection", 2, anyContext, matchObj(serviceBinding1.DieReleasePtr())).Return(nil).Once()
						m.On("Projector.Project", 3, anyContext, matchObj(serviceBinding1.DieReleasePtr()), matchObj(workload.DieReleaseUnstructured())).Return(nil).Once()
						m.On("ServiceBindingPostProjection", 4, anyContext, matchObj(serviceBinding1.DieReleasePtr())).Return(nil).Once()
						m.On("ServiceBindingPreProjection", 5, anyContext, matchObj(serviceBinding2.DieReleasePtr())).Return(nil).Once()
						m.On("Projector.Project", 6, anyContext, matchObj(serviceBinding2.DieReleasePtr()), matchObj(workload.DieReleaseUnstructured())).Return(nil).Once()
						m.On("ServiceBindingPostProjection", 7, anyContext, matchObj(serviceBinding2.DieReleasePtr())).Return(nil).Once()
						m.On("WorkloadPostProjection", 8, anyContext, matchObj(workload.DieReleaseUnstructured())).Return(nil).Once()
					},
				},
				CleanUp: func(t *testing.T, ctx context.Context, tc *rtesting.SubReconcilerTestCase[*unstructured.Unstructured]) error {
					m := tc.Metadata["HooksMock"].(*mock.Mock)
					m.AssertExpectations(t)
					return nil
				},
				WithClientBuilder: addWorkloadRefIndex,
				Resource:          workload.DieReleaseUnstructured(),
				GivenObjects: []client.Object{
					serviceBinding1.DieReleasePtr(),
					serviceBinding2.DieReleasePtr(),
				},
			},
		}

		rts.Run(t, scheme, func(t *testing.T, rtc *rtesting.SubReconcilerTestCase[*unstructured.Unstructured], c reconcilers.Config) reconcilers.SubReconciler[*unstructured.Unstructured] {
			restMapper := c.RESTMapper().(*meta.DefaultRESTMapper)
			restMapper.Add(schema.GroupVersionKind{Group: "apps", Version: "v1", Kind: "Deployment"}, meta.RESTScopeNamespace)
			hooks, m := makeHooks()
			rtc.Metadata["HooksMock"] = m
			rtc.Metadata["HooksExpectations"].(func(*mock.Mock))(m)
			return controllers.AdmissionProjectorWebhook(c, hooks).Reconciler
		})
	})
}

type mockProjector struct {
	m *mock.Mock
	i *int
}

func (p *mockProjector) Project(ctx context.Context, binding *servicebindingv1.ServiceBinding, workload runtime.Object) error {
	*p.i = *p.i + 1
	return p.m.MethodCalled("Projector.Project", *p.i, ctx, binding, workload).Error(0)
}

func (p *mockProjector) Unproject(ctx context.Context, binding *servicebindingv1.ServiceBinding, workload runtime.Object) error {
	*p.i = *p.i + 1
	return p.m.MethodCalled("Projector.Unproject", *p.i, ctx, binding, workload).Error(0)
}

func (p *mockProjector) IsProjected(ctx context.Context, binding *servicebindingv1.ServiceBinding, workload runtime.Object) bool {
	annotations := workload.(metav1.Object).GetAnnotations()
	if len(annotations) == 0 {
		return false
	}
	_, ok := annotations[fmt.Sprintf("%s%s", projector.MappingAnnotationPrefix, workload.(metav1.Object).GetUID())]
	return ok
}

func makeHooks() (lifecycle.ServiceBindingHooks, *mock.Mock) {
	m := &mock.Mock{}
	i := pointer.Int(0)
	hooks := lifecycle.ServiceBindingHooks{
		ResolverFactory: func(c client.Client) resolver.Resolver {
			return resolver.New(c)
		},
		ProjectorFactory: func(ms projector.MappingSource) projector.ServiceBindingProjector {
			return &mockProjector{m: m, i: i}
		},
		ServiceBindingPreProjection: func(ctx context.Context, binding *servicebindingv1.ServiceBinding) error {
			*i = *i + 1
			return m.MethodCalled("ServiceBindingPreProjection", *i, ctx, binding).Error(0)
		},
		ServiceBindingPostProjection: func(ctx context.Context, binding *servicebindingv1.ServiceBinding) error {
			*i = *i + 1
			return m.MethodCalled("ServiceBindingPostProjection", *i, ctx, binding).Error(0)
		},
		WorkloadPreProjection: func(ctx context.Context, workload runtime.Object) error {
			*i = *i + 1
			return m.MethodCalled("WorkloadPreProjection", *i, ctx, workload).Error(0)
		},
		WorkloadPostProjection: func(ctx context.Context, workload runtime.Object) error {
			*i = *i + 1
			return m.MethodCalled("WorkloadPostProjection", *i, ctx, workload).Error(0)
		},
	}

	return hooks, m
}

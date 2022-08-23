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
	"reflect"

	"github.com/go-logr/logr"
	"github.com/vmware-labs/reconciler-runtime/reconcilers"
	"github.com/vmware-labs/reconciler-runtime/tracker"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/util/workqueue"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	servicebindingv1beta1 "github.com/servicebinding/runtime/apis/v1beta1"
	"github.com/servicebinding/runtime/projector"
	"github.com/servicebinding/runtime/rbac"
	"github.com/servicebinding/runtime/resolver"
)

//+kubebuilder:rbac:groups=admissionregistration.k8s.io,resources=mutatingwebhookconfigurations,verbs=get;list;watch;create;update;patch
//+kubebuilder:rbac:groups=core,resources=events,verbs=get;list;watch;create;update;patch;delete

// AdmissionProjector reconciles a MutatingWebhookConfiguration object
func AdmissionProjectorReconciler(c reconcilers.Config, name string, accessChecker rbac.AccessChecker) *reconcilers.AggregateReconciler {
	req := reconcile.Request{
		NamespacedName: types.NamespacedName{
			Name: name,
		},
	}

	return &reconcilers.AggregateReconciler{
		Name:    "AdmissionProjector",
		Type:    &admissionregistrationv1.MutatingWebhookConfiguration{},
		Request: req,
		Reconciler: reconcilers.Sequence{
			LoadServiceBindings(req),
			InterceptGVKs(),
			WebhookRules([]admissionregistrationv1.OperationType{admissionregistrationv1.Create, admissionregistrationv1.Update}, accessChecker),
		},
		DesiredResource: func(ctx context.Context, resource *admissionregistrationv1.MutatingWebhookConfiguration) (client.Object, error) {
			if resource == nil || len(resource.Webhooks) != 1 {
				// the webhook config isn't in a form that we expect, ignore it
				return resource, nil
			}
			rules := RetrieveWebhookRules(ctx)
			resource.Webhooks[0].Rules = rules
			return resource, nil
		},
		MergeBeforeUpdate: func(current, desired *admissionregistrationv1.MutatingWebhookConfiguration) {
			if current == nil || len(current.Webhooks) != 1 || desired == nil || len(desired.Webhooks) != 1 {
				// the webhook config isn't in a form that we expect, ignore it
				return
			}
			current.Webhooks[0].Rules = desired.Webhooks[0].Rules
		},
		Sanitize: func(resource *admissionregistrationv1.MutatingWebhookConfiguration) []admissionregistrationv1.RuleWithOperations {
			if resource == nil || len(resource.Webhooks) == 0 {
				return nil
			}
			return resource.Webhooks[0].Rules
		},

		Setup: func(ctx context.Context, mgr controllerruntime.Manager, bldr *builder.Builder) error {
			if err := mgr.GetFieldIndexer().IndexField(ctx, &servicebindingv1beta1.ServiceBinding{}, workloadRefIndexKey, func(obj client.Object) []string {
				serviceBinding := obj.(*servicebindingv1beta1.ServiceBinding)
				gvk := schema.FromAPIVersionAndKind(serviceBinding.Spec.Workload.APIVersion, serviceBinding.Spec.Workload.Kind)
				return []string{workloadRefIndexValue(gvk.Group, gvk.Kind)}
			}); err != nil {
				return err
			}
			return nil
		},
		Config: c,
	}
}

func AdmissionProjectorWebhook(c reconcilers.Config) *reconcilers.AdmissionWebhookAdapter {
	return &reconcilers.AdmissionWebhookAdapter{
		Name: "AdmissionProjectorWebhook",
		Type: &unstructured.Unstructured{},
		Reconciler: &reconcilers.SyncReconciler{
			Sync: func(ctx context.Context, workload *unstructured.Unstructured) error {
				c := reconcilers.RetrieveConfigOrDie(ctx)

				// find matching service bindings
				serviceBindings := &servicebindingv1beta1.ServiceBindingList{}
				gvk := schema.FromAPIVersionAndKind(workload.GetAPIVersion(), workload.GetKind())
				if err := c.List(ctx, serviceBindings, client.InNamespace(workload.GetNamespace()), client.MatchingFields{workloadRefIndexKey: workloadRefIndexValue(gvk.Group, gvk.Kind)}); err != nil {
					return err
				}

				// check that bindings are for this workload
				activeServiceBindings := []servicebindingv1beta1.ServiceBinding{}
				for _, sb := range serviceBindings.Items {
					if !sb.DeletionTimestamp.IsZero() {
						continue
					}
					ref := sb.Spec.Workload
					if ref.Name == workload.GetName() {
						activeServiceBindings = append(activeServiceBindings, sb)
						continue
					}
					if ref.Selector != nil {
						selector, err := metav1.LabelSelectorAsSelector(ref.Selector)
						if err != nil {
							continue
						}
						if selector.Matches(labels.Set(workload.GetLabels())) {
							activeServiceBindings = append(activeServiceBindings, sb)
							continue
						}
					}
				}

				// project active bindings into workload
				projector := projector.New(resolver.New(c))
				for i := range activeServiceBindings {
					sb := activeServiceBindings[i].DeepCopy()
					sb.Default()
					if err := projector.Project(ctx, sb, workload); err != nil {
						return err
					}
				}

				return nil
			},
		},
		Config: c,
	}
}

//+kubebuilder:rbac:groups=admissionregistration.k8s.io,resources=validatingwebhookconfigurations,verbs=get;list;watch;create;update;patch
//+kubebuilder:rbac:groups=core,resources=events,verbs=get;list;watch;create;update;patch;delete

// TriggerReconciler reconciles a ValidatingWebhookConfiguration object
func TriggerReconciler(c reconcilers.Config, name string, accessChecker rbac.AccessChecker) *reconcilers.AggregateReconciler {
	req := reconcile.Request{
		NamespacedName: types.NamespacedName{
			Name: name,
		},
	}

	return &reconcilers.AggregateReconciler{
		Name:    "Trigger",
		Type:    &admissionregistrationv1.ValidatingWebhookConfiguration{},
		Request: req,
		Reconciler: reconcilers.Sequence{
			LoadServiceBindings(req),
			TriggerGVKs(),
			InterceptGVKs(),
			WebhookRules([]admissionregistrationv1.OperationType{admissionregistrationv1.Create, admissionregistrationv1.Update, admissionregistrationv1.Delete}, accessChecker),
		},
		DesiredResource: func(ctx context.Context, resource *admissionregistrationv1.ValidatingWebhookConfiguration) (client.Object, error) {
			if resource == nil || len(resource.Webhooks) != 1 {
				// the webhook config isn't in a form that we expect, ignore it
				return resource, nil
			}
			rules := RetrieveWebhookRules(ctx)
			resource.Webhooks[0].Rules = rules
			return resource, nil
		},
		MergeBeforeUpdate: func(current, desired *admissionregistrationv1.ValidatingWebhookConfiguration) {
			if current == nil || len(current.Webhooks) != 1 || desired == nil || len(desired.Webhooks) != 1 {
				// the webhook config isn't in a form that we expect, ignore it
				return
			}
			current.Webhooks[0].Rules = desired.Webhooks[0].Rules
		},
		Sanitize: func(resource *admissionregistrationv1.ValidatingWebhookConfiguration) []admissionregistrationv1.RuleWithOperations {
			if resource == nil || len(resource.Webhooks) == 0 {
				return nil
			}
			return resource.Webhooks[0].Rules
		},

		Config: c,
	}
}

func TriggerWebhook(c reconcilers.Config, serviceBindingController controller.Controller) *reconcilers.AdmissionWebhookAdapter {
	return &reconcilers.AdmissionWebhookAdapter{
		Name: "AdmissionProjectorWebhook",
		Type: &unstructured.Unstructured{},
		Reconciler: &reconcilers.SyncReconciler{
			Sync: func(ctx context.Context, trigger *unstructured.Unstructured) error {
				log := logr.FromContextOrDiscard(ctx)
				c := reconcilers.RetrieveConfigOrDie(ctx)
				req := reconcilers.RetrieveAdmissionRequest(ctx)

				// TODO find a better way to get at the queue, this is fragile and may break in any controller-runtime update
				queueValue := reflect.ValueOf(serviceBindingController).Elem().FieldByName("Queue")
				if queueValue.IsNil() {
					// queue is not populated yet
					return nil
				}
				queue := queueValue.Interface().(workqueue.Interface)

				trackKey := tracker.NewKey(
					schema.FromAPIVersionAndKind(trigger.GetAPIVersion(), trigger.GetKind()),
					types.NamespacedName{
						Namespace: trigger.GetNamespace(),
						Name:      trigger.GetName(),
					},
				)
				for _, nsn := range c.Tracker.Lookup(ctx, trackKey) {
					rr := reconcile.Request{NamespacedName: nsn}
					log.V(2).Info("enqueue tracked request", "request", rr, "for", trackKey, "dryRun", req.DryRun)
					if req.DryRun != nil && *req.DryRun {
						// ignore dry run requests
						continue
					}
					queue.Add(rr)
				}

				return nil
			},
		},
		Config: c,
	}
}

func LoadServiceBindings(req reconcile.Request) reconcilers.SubReconciler {
	return &reconcilers.SyncReconciler{
		Name: "LoadServiceBindings",
		Sync: func(ctx context.Context, _ client.Object) error {
			c := reconcilers.RetrieveConfigOrDie(ctx)

			serviceBindings := &servicebindingv1beta1.ServiceBindingList{}
			if err := c.List(ctx, serviceBindings); err != nil {
				return err
			}

			StashServiceBindings(ctx, serviceBindings.Items)

			return nil
		},
		Setup: func(ctx context.Context, mgr controllerruntime.Manager, bldr *builder.Builder) error {
			bldr.Watches(&source.Kind{Type: &servicebindingv1beta1.ServiceBinding{}}, handler.EnqueueRequestsFromMapFunc(
				func(o client.Object) []reconcile.Request {
					return []reconcile.Request{req}
				},
			))
			return nil
		},
	}
}

func InterceptGVKs() reconcilers.SubReconciler {
	return &reconcilers.SyncReconciler{
		Name: "InterceptGVKs",
		Sync: func(ctx context.Context, _ client.Object) error {
			serviceBindings := RetrieveServiceBindings(ctx)
			gvks := RetrieveObservedGKVs(ctx)

			for i := range serviceBindings {
				workload := serviceBindings[i].Spec.Workload
				gvk := schema.FromAPIVersionAndKind(workload.APIVersion, workload.Kind)
				gvks = append(gvks, gvk)
			}

			StashObservedGVKs(ctx, gvks)

			return nil
		},
	}
}

func TriggerGVKs() reconcilers.SubReconciler {
	return &reconcilers.SyncReconciler{
		Name: "TriggerGVKs",
		Sync: func(ctx context.Context, _ client.Object) error {
			serviceBindings := RetrieveServiceBindings(ctx)
			gvks := RetrieveObservedGKVs(ctx)

			for i := range serviceBindings {
				service := serviceBindings[i].Spec.Service
				gvk := schema.FromAPIVersionAndKind(service.APIVersion, service.Kind)
				if gvk.Kind == "Secret" && (gvk.Group == "" || gvk.Group == "core") {
					// ignore direct bindings
					continue
				}
				gvks = append(gvks, gvk)
			}

			StashObservedGVKs(ctx, gvks)

			return nil
		},
	}
}

func WebhookRules(operations []admissionregistrationv1.OperationType, accessChecker rbac.AccessChecker) reconcilers.SubReconciler {
	return &reconcilers.SyncReconciler{
		Name: "WebhookRules",
		Sync: func(ctx context.Context, _ client.Object) error {
			log := logr.FromContextOrDiscard(ctx)
			c := reconcilers.RetrieveConfigOrDie(ctx)

			// dedup gvks as gvrs
			gvks := RetrieveObservedGKVs(ctx)
			groupResources := map[string]map[string]interface{}{}
			for _, gvk := range gvks {
				rm, err := c.RESTMapper().RESTMapping(gvk.GroupKind(), gvk.Version)
				if err != nil {
					return err
				}
				gvr := rm.Resource
				if _, ok := groupResources[gvr.Group]; !ok {
					groupResources[gvr.Group] = map[string]interface{}{}
				}
				groupResources[gvr.Group][gvr.Resource] = true
			}

			// normalize rules to a canonical form
			rules := []admissionregistrationv1.RuleWithOperations{}
			groups := sets.NewString()
			for group := range groupResources {
				groups.Insert(group)
			}
			for _, group := range groups.List() {
				resources := sets.NewString()
				for resource := range groupResources[group] {
					resources.Insert(resource)
				}

				// check that we have permission to interact with these resources. Admission webhooks bypass RBAC
				for _, resource := range resources.List() {
					if !accessChecker.CanI(ctx, group, resource) {
						log.Info("ignoring resource, access denied", "group", group, "resource", resource)
						resources.Delete(resource)
					}
				}

				if resources.Len() == 0 {
					continue
				}

				rules = append(rules, admissionregistrationv1.RuleWithOperations{
					Operations: operations,
					Rule: admissionregistrationv1.Rule{
						APIGroups:   []string{group},
						APIVersions: []string{"*"},
						Resources:   resources.List(),
					},
				})
			}

			StashWebhookRules(ctx, rules)

			return nil
		},
	}
}

const ServiceBindingsStashKey reconcilers.StashKey = "servicebinding.io:servicebindings"

func StashServiceBindings(ctx context.Context, serviceBindings []servicebindingv1beta1.ServiceBinding) {
	reconcilers.StashValue(ctx, ServiceBindingsStashKey, serviceBindings)
}

func RetrieveServiceBindings(ctx context.Context) []servicebindingv1beta1.ServiceBinding {
	value := reconcilers.RetrieveValue(ctx, ServiceBindingsStashKey)
	if serviceBindings, ok := value.([]servicebindingv1beta1.ServiceBinding); ok {
		return serviceBindings
	}
	return nil
}

const ObservedGVKsStashKey reconcilers.StashKey = "servicebinding.io:observedgvks"

func StashObservedGVKs(ctx context.Context, gvks []schema.GroupVersionKind) {
	reconcilers.StashValue(ctx, ObservedGVKsStashKey, gvks)
}

func RetrieveObservedGKVs(ctx context.Context) []schema.GroupVersionKind {
	value := reconcilers.RetrieveValue(ctx, ObservedGVKsStashKey)
	if refs, ok := value.([]schema.GroupVersionKind); ok {
		return refs
	}
	return nil
}

const WebhookRulesStashKey reconcilers.StashKey = "servicebinding.io:webhookrules"

func StashWebhookRules(ctx context.Context, rules []admissionregistrationv1.RuleWithOperations) {
	reconcilers.StashValue(ctx, WebhookRulesStashKey, rules)
}

func RetrieveWebhookRules(ctx context.Context) []admissionregistrationv1.RuleWithOperations {
	value := reconcilers.RetrieveValue(ctx, WebhookRulesStashKey)
	if rules, ok := value.([]admissionregistrationv1.RuleWithOperations); ok {
		return rules
	}
	return nil
}

const workloadRefIndexKey = ".metadata.workloadRef"

func workloadRefIndexValue(group, kind string) string {
	return schema.GroupKind{Group: group, Kind: kind}.String()
}

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

package controllers

import (
	"context"
	"encoding/json"
	"time"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/source"

	servicebindingv1beta1 "github.com/servicebinding/service-binding-controller/apis/v1beta1"
	"github.com/servicebinding/service-binding-controller/projector"
	"github.com/servicebinding/service-binding-controller/resolver"
)

// ServiceBindingReconciler reconciles a ServiceBinding object
type ServiceBindingReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=servicebinding.io,resources=servicebindings,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=servicebinding.io,resources=servicebindings/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=servicebinding.io,resources=servicebindings/finalizers,verbs=update
//+kubebuilder:rbac:groups=servicebinding.io,resources=clusterworkloadresourcemappings,verbs=get;list;watch
//+kubebuilder:rbac:groups=apps,resources=deployments;daemonsets;replicasets;statefulsets,verbs=get;list;update;patch

// tag::reconcile-func[]

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the ServiceBinding object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.8.3/pkg/reconcile
func (r *ServiceBindingReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	servicebinding := &servicebindingv1beta1.ServiceBinding{}

	if err := r.Get(ctx, req.NamespacedName, servicebinding); err != nil {
		if errors.IsNotFound(err) {
			// We got a request for reconciliation, but no object exists.  Do
			// nothing and don't requeue.
			log.Info("Service Binding not found; ignoring.", "name", req.NamespacedName, "err", err)
			return ctrl.Result{}, nil
		}
		log.Error(err, "Unable to retrieve service binding", "name", req.NamespacedName, "err", err)
		return ctrl.Result{}, nil
	}
	// log.Info("Retrieved Service Binding", "service binding", servicebinding)

	resolver := resolver.New(r.Client)
	workloadData := servicebinding.Spec.Workload
	workloadRef := v1.ObjectReference{
		APIVersion: workloadData.APIVersion,
		Kind:       workloadData.Kind,
		Name:       workloadData.Name,
		Namespace:  servicebinding.Namespace,
	}

	log.Info("Attempting to retrieve workload", "workloadRef", workloadRef)
	workload, err := resolver.LookupWorkload(ctx, workloadRef)
	if err != nil {
		updateStatus(servicebinding, err)
		r.Status().Update(ctx, servicebinding, &client.UpdateOptions{})

		log.Error(err,
			"Unable to retrieve workload",
			"workload", workloadRef,
			"service binding", servicebinding)
		return ctrl.Result{Requeue: true}, nil
	}
	// log.Info("Retrieved workload", "workload", workload, "workloadRef", workloadRef)

	// workloadUnstructured, err := runtime.DefaultUnstructuredConverter.ToUnstructured(workload)
	// if err != nil {
	// 	updateStatus(servicebinding, err)
	// 	log.Error(err, "Unable to fetch binding information", "workload", workload)
	// 	return ctrl.Result{Requeue: true}, nil
	// }
	// secret, found, err := unstructured.NestedString(workloadUnstructured, "status", "binding", "name")
	// if !found || err != nil {
	// 	updateStatus(servicebinding, err)
	// 	log.Error(err, "Unable to fetch binding information", "workload", workload)
	// 	return ctrl.Result{Requeue: true}, nil
	// }
	// servicebinding.Status.Binding.Name = secret

	projector := projector.New(resolver)
	if servicebinding.DeletionTimestamp.IsZero() {
		err = projector.Project(ctx, servicebinding, workload)
		if err != nil {
			log.Error(
				err,
				"Failed to project bindings into workload",
				"workload", workload,
				"service binding", servicebinding,
				"err", err)
		}
	} else {
		err = projector.Unproject(ctx, servicebinding, workload)
		if err != nil {
			log.Error(
				err,
				"Failed to unproject bindings into workload",
				"workload", workload,
				"service binding", servicebinding,
				"err", err)
		}
	}
	requeue := err != nil
	projectorErr := err

	data, err := json.Marshal(workload)
	if err != nil {
		log.Error(err, "Error marshalling workload", "workload", workload, "err", err)
	}
	log.Info("Projected service bindings", "service binding", servicebinding, "workload", data)

	err = r.Update(ctx, workload, &client.UpdateOptions{})
	requeue = requeue || (err != nil)
	if err != nil {
		log.Error(err, "Unable to update workload", "workload", workload, "err", err)
	} else {
		err = projectorErr
	}

	updateStatus(servicebinding, err)
	err = r.Status().Update(ctx, servicebinding, &client.UpdateOptions{})
	if err != nil {
		log.Error(err, "Unable to update status of servicebinding", "service binding", servicebinding, "err", err)
		requeue = true
	}

	if requeue {
		log.Error(
			err,
			"Failed to resolve service binding",
			"service binding", servicebinding,
			"err", err,
		)
	} else {
		log.Info(
			"Resolved service binding",
			"service binding", servicebinding,
		)
	}
	return ctrl.Result{Requeue: requeue}, nil
}

// end::reconcile-func[]

// SetupWithManager sets up the controller with the Manager.
func (r *ServiceBindingReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&servicebindingv1beta1.ServiceBinding{}).
		Watches(&source.Kind{Type: &servicebindingv1beta1.ClusterWorkloadResourceMapping{}}, handler.Funcs{}).
		Complete(r)
}

func updateStatus(binding *servicebindingv1beta1.ServiceBinding, err error) {
	condition := metav1.Condition{
		Type:               servicebindingv1beta1.ServiceBindingConditionReady,
		Reason:             "Projected",
		LastTransitionTime: metav1.Time{Time: time.Now()},
	}
	if err != nil {
		condition.Status = metav1.ConditionFalse
		condition.Message = err.Error()
	} else {
		condition.Status = metav1.ConditionTrue
	}

	binding.Status.Conditions = []metav1.Condition{condition}
	binding.Status.ObservedGeneration = binding.Generation
}

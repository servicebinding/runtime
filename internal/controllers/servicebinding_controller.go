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

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/source"

	servicebindingv1beta1 "github.com/servicebinding/service-binding-controller/apis/v1beta1"
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
		log.Error(err, "Unable to retrieve service binding", "name", req.NamespacedName)
		return ctrl.Result{}, nil
	}

	requeue, err := ResolveServiceBinding(ctx, servicebinding, r.Client)
	updateStatus(servicebinding, err)
	if err != nil {
		return ctrl.Result{Requeue: requeue}, err
	}

	log.Info("Writing service binding", "service binding", servicebinding)
	err = r.Status().Update(ctx, servicebinding)
	if err != nil {
		log.Error(err, "Unable to update status of servicebinding", "service binding", servicebinding)
		requeue = true
	}

	log.Info(
		"Resolved service binding",
		"service binding", servicebinding,
		"requeue", requeue,
		"err", err,
	)
	return ctrl.Result{Requeue: requeue}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ServiceBindingReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&servicebindingv1beta1.ServiceBinding{}).
		Watches(&source.Kind{Type: &servicebindingv1beta1.ClusterWorkloadResourceMapping{}}, handler.Funcs{}).
		Complete(r)
}

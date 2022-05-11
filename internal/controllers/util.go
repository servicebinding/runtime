package controllers

import (
	"context"
	"time"

	"github.com/servicebinding/service-binding-controller/apis/v1beta1"
	"github.com/servicebinding/service-binding-controller/projector"
	"github.com/servicebinding/service-binding-controller/resolver"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	serviceBindingFinalizer = "finalizer.servicebinding.servicebinding.io"
)

// looks up all resources and performs a binding;
// returns any errors that occurred and if a retry needs to happen
func ResolveServiceBinding(ctx context.Context, binding *v1beta1.ServiceBinding, clientInterface client.Client) (bool, error) {
	log := log.FromContext(ctx, "serviceBinding", binding)

	resolver := resolver.New(clientInterface)

	workloadData := binding.Spec.Workload
	workloadRef := v1.ObjectReference{
		APIVersion: workloadData.APIVersion,
		Kind:       workloadData.Kind,
		Name:       workloadData.Name,
		Namespace:  binding.Namespace,
	}

	log.Info("Attempting to retrieve workload", "workloadRef", workloadRef)
	workload, err := resolver.LookupWorkload(ctx, workloadRef)
	if err != nil {
		log.Error(err,
			"Unable to retrieve workload",
			"workload", workloadRef)
		return true, err
	}

	serviceRef := v1.ObjectReference{
		APIVersion: binding.Spec.Service.APIVersion,
		Kind:       binding.Spec.Service.Kind,
		Name:       binding.Spec.Service.Name,
		Namespace:  binding.Namespace,
	}
	secret, err := resolver.LookupBindingSecret(ctx, serviceRef)
	if err != nil {
		log.Error(err, "Unable to retrieve binding information from service", "service", serviceRef)
	}
	binding.Status.Binding = &v1beta1.ServiceBindingSecretReference{Name: secret}

	projector := projector.New(resolver)
	if binding.DeletionTimestamp.IsZero() {
		err = projector.Project(ctx, binding, workload)
		if err != nil {
			log.Error(
				err,
				"Failed to project bindings into workload",
				"workload", workload,
				"service binding", binding,
				"err", err)
		}
		trySetFinalizer(binding)
	} else {
		err = projector.Unproject(ctx, binding, workload)
		if err != nil {
			log.Error(
				err,
				"Failed to unproject bindings into workload",
				"workload", workload,
				"service binding", binding,
				"err", err)
			return true, err
		}
		tryUnsetFinalizer(binding)
	}

	err = clientInterface.Update(ctx, workload)
	if err != nil {
		log.Error(err, "Unable to update workload", "workload", workload)
		return true, err
	}

	return false, nil
}

func trySetFinalizer(binding *v1beta1.ServiceBinding) {
	if binding == nil {
		return
	}

	finalizers := binding.GetFinalizers()
	for _, f := range finalizers {
		if f == serviceBindingFinalizer {
			return
		}
	}
	binding.SetFinalizers(append(finalizers, serviceBindingFinalizer))
}

func tryUnsetFinalizer(binding *v1beta1.ServiceBinding) {
	if binding == nil {
		return
	}
	finalizers := binding.GetFinalizers()
	for i, f := range finalizers {
		if f == serviceBindingFinalizer {
			binding.SetFinalizers(append(finalizers[:i], finalizers[i+1:]...))
			return
		}
	}
}

func updateStatus(binding *v1beta1.ServiceBinding, err error) {
	condition := metav1.Condition{
		Type:               v1beta1.ServiceBindingConditionReady,
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

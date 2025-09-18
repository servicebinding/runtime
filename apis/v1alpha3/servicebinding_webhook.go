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

package v1alpha3

import (
	"context"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	servicebindingv1 "github.com/servicebinding/runtime/apis/v1"
)

func (r *ServiceBinding) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		WithDefaulter(r).
		WithValidator(r).
		Complete()
}

var _ webhook.CustomDefaulter = &ServiceBinding{}

// Default implements webhook.CustomDefaulter so a webhook will be registered for the type
func (r *ServiceBinding) Default(ctx context.Context, obj runtime.Object) error {
	r = obj.(*ServiceBinding)
	r1 := &servicebindingv1.ServiceBinding{}
	if err := r.ConvertTo(r1); err != nil {
		return err
	}
	if err := (&servicebindingv1.ServiceBinding{}).Default(ctx, r1); err != nil {
		return err
	}
	if err := r.ConvertFrom(r1); err != nil {
		return err
	}
	return nil
}

//+kubebuilder:webhook:path=/validate-servicebinding-io-v1alpha3-servicebinding,mutating=false,failurePolicy=fail,sideEffects=None,groups=servicebinding.io,resources=servicebindings,verbs=create;update,versions=v1alpha3,name=v1alpha3.servicebindings.servicebinding.io,admissionReviewVersions={v1,v1beta1}

var _ webhook.CustomValidator = &ServiceBinding{}

// ValidateCreate implements webhook.CustomValidator so a webhook will be registered for the type
func (r *ServiceBinding) ValidateCreate(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	r = obj.(*ServiceBinding)

	r1 := &servicebindingv1.ServiceBinding{}
	if err := r.ConvertTo(r1); err != nil {
		return nil, err
	}
	return (&servicebindingv1.ServiceBinding{}).ValidateCreate(ctx, r1)
}

// ValidateUpdate implements webhook.CustomValidator so a webhook will be registered for the type
func (r *ServiceBinding) ValidateUpdate(ctx context.Context, old, obj runtime.Object) (admission.Warnings, error) {
	r = obj.(*ServiceBinding)

	r1 := &servicebindingv1.ServiceBinding{}
	if err := r.ConvertTo(r1); err != nil {
		return nil, err
	}
	return (&servicebindingv1.ServiceBinding{}).ValidateUpdate(ctx, old, r1)
}

// ValidateDelete implements webhook.CustomValidator so a webhook will be registered for the type
func (r *ServiceBinding) ValidateDelete(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	r = obj.(*ServiceBinding)

	r1 := &servicebindingv1.ServiceBinding{}
	if err := r.ConvertTo(r1); err != nil {
		return nil, err
	}
	return (&servicebindingv1.ServiceBinding{}).ValidateDelete(ctx, r1)
}

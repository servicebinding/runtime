/*
Copyright 2021 the original author or authors.

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

package v1

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/conversion"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

func (r *ServiceBinding) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

var _ webhook.CustomDefaulter = &ServiceBinding{}

// Default implements webhook.CustomDefaulter so a webhook will be registered for the type
func (r *ServiceBinding) Default(ctx context.Context, obj runtime.Object) error {
	r = obj.(*ServiceBinding)

	if r.Spec.Name == "" {
		r.Spec.Name = r.Name
	}

	return nil
}

//+kubebuilder:webhook:path=/validate-servicebinding-io-v1-servicebinding,mutating=false,failurePolicy=fail,sideEffects=None,groups=servicebinding.io,resources=servicebindings,verbs=create;update,versions=v1,name=v1.servicebindings.servicebinding.io,admissionReviewVersions={v1,v1beta1}

var _ webhook.CustomValidator = &ServiceBinding{}

// ValidateCreate implements webhook.CustomValidator so a webhook will be registered for the type
func (r *ServiceBinding) ValidateCreate(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	r = obj.(*ServiceBinding)

	(&ServiceBinding{}).Default(ctx, r)
	return nil, r.validate().ToAggregate()
}

// ValidateUpdate implements webhook.CustomValidator so a webhook will be registered for the type
func (r *ServiceBinding) ValidateUpdate(ctx context.Context, old, obj runtime.Object) (admission.Warnings, error) {
	r = obj.(*ServiceBinding)

	(&ServiceBinding{}).Default(ctx, r)
	errs := field.ErrorList{}

	// check immutable fields
	var ro *ServiceBinding
	if o, ok := old.(*ServiceBinding); ok {
		ro = o
	} else if o, ok := old.(conversion.Convertible); ok {
		ro = &ServiceBinding{}
		if err := o.ConvertTo(ro); err != nil {
			return nil, err
		}
	} else {
		errs = append(errs,
			field.InternalError(nil, fmt.Errorf("old object must be of type v1.ServiceBinding")),
		)
	}
	if len(errs) == 0 {
		if r.Spec.Workload.APIVersion != ro.Spec.Workload.APIVersion {
			errs = append(errs,
				field.Forbidden(field.NewPath("spec", "workload", "apiVersion"), "Workload apiVersion is immutable. Delete and recreate the ServiceBinding to update."),
			)
		}
		if r.Spec.Workload.Kind != ro.Spec.Workload.Kind {
			errs = append(errs,
				field.Forbidden(field.NewPath("spec", "workload", "kind"), "Workload kind is immutable. Delete and recreate the ServiceBinding to update."),
			)
		}
	}

	// validate new object
	errs = append(errs, r.validate()...)

	return nil, errs.ToAggregate()
}

// ValidateDelete implements webhook.CustomValidator so a webhook will be registered for the type
func (r *ServiceBinding) ValidateDelete(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	return nil, nil
}

func (r *ServiceBinding) validate() field.ErrorList {
	errs := field.ErrorList{}

	errs = append(errs, r.Spec.validate(field.NewPath("spec"))...)

	return errs
}

func (r *ServiceBindingSpec) validate(fldPath *field.Path) field.ErrorList {
	errs := field.ErrorList{}

	if r.Name == "" {
		errs = append(errs, field.Required(fldPath.Child("name"), ""))
	}
	errs = append(errs, r.Service.validate(fldPath.Child("service"))...)
	errs = append(errs, r.Workload.validate(fldPath.Child("workload"))...)
	for i := range r.Env {
		errs = append(errs, r.Env[i].validate(fldPath.Child("env").Index(i))...)
	}

	return errs
}

func (r *ServiceBindingServiceReference) validate(fldPath *field.Path) field.ErrorList {
	errs := field.ErrorList{}

	if r.APIVersion == "" {
		errs = append(errs, field.Required(fldPath.Child("apiVersion"), ""))
	}
	if r.Kind == "" {
		errs = append(errs, field.Required(fldPath.Child("kind"), ""))
	}
	if r.Name == "" {
		errs = append(errs, field.Required(fldPath.Child("name"), ""))
	}

	return errs
}

func (r *ServiceBindingWorkloadReference) validate(fldPath *field.Path) field.ErrorList {
	errs := field.ErrorList{}

	if r.APIVersion == "" {
		errs = append(errs, field.Required(fldPath.Child("apiVersion"), ""))
	}
	if r.Kind == "" {
		errs = append(errs, field.Required(fldPath.Child("kind"), ""))
	}
	if r.Name == "" && r.Selector == nil {
		errs = append(errs, field.Required(fldPath.Child("[name, selector]"), "expected exactly one, got neither"))
	}
	if r.Name != "" && r.Selector != nil {
		errs = append(errs, field.Required(fldPath.Child("[name, selector]"), "expected exactly one, got both"))
	}
	if r.Selector != nil {
		if _, err := metav1.LabelSelectorAsSelector(r.Selector); err != nil {
			errs = append(errs, field.Invalid(fldPath.Child("selector"), r.Selector, err.Error()))
		}
	}

	return errs
}

func (r *EnvMapping) validate(fldPath *field.Path) field.ErrorList {
	errs := field.ErrorList{}

	if r.Name == "" {
		errs = append(errs, field.Required(fldPath.Child("name"), ""))
	}
	if r.Key == "" {
		errs = append(errs, field.Required(fldPath.Child("key"), ""))
	}

	return errs
}

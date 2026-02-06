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

	"github.com/go-logr/logr"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

func (r *ServiceBinding) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr, r).
		WithDefaulter(r).
		WithValidator(r).
		Complete()
}

var _ admission.Defaulter[*ServiceBinding] = &ServiceBinding{}

// Default implements webhook.CustomDefaulter so a webhook will be registered for the type
func (*ServiceBinding) Default(ctx context.Context, obj *ServiceBinding) error {
	if obj.Spec.Name == "" {
		obj.Spec.Name = obj.Name
	}

	return nil
}

//+kubebuilder:webhook:path=/validate-servicebinding-io-v1-servicebinding,mutating=false,failurePolicy=fail,sideEffects=None,groups=servicebinding.io,resources=servicebindings,verbs=create;update,versions=v1,name=v1.servicebindings.servicebinding.io,admissionReviewVersions={v1,v1beta1}

var _ admission.Validator[*ServiceBinding] = &ServiceBinding{}

// ValidateCreate implements webhook.CustomValidator so a webhook will be registered for the type
func (*ServiceBinding) ValidateCreate(ctx context.Context, obj *ServiceBinding) (admission.Warnings, error) {
	log := logr.FromContextOrDiscard(ctx)
	log.V(1).Info("Validating Create")

	(&ServiceBinding{}).Default(ctx, obj)
	return nil, obj.validate().ToAggregate()
}

// ValidateUpdate implements webhook.CustomValidator so a webhook will be registered for the type
func (*ServiceBinding) ValidateUpdate(ctx context.Context, old, obj *ServiceBinding) (admission.Warnings, error) {
	log := logr.FromContextOrDiscard(ctx)
	log.V(1).Info("Validating Update")

	(&ServiceBinding{}).Default(ctx, obj)
	errs := field.ErrorList{}

	// check immutable fields
	if obj.Spec.Workload.APIVersion != old.Spec.Workload.APIVersion {
		errs = append(errs,
			field.Forbidden(field.NewPath("spec", "workload", "apiVersion"), "Workload apiVersion is immutable. Delete and recreate the ServiceBinding to update."),
		)
	}
	if obj.Spec.Workload.Kind != old.Spec.Workload.Kind {
		errs = append(errs,
			field.Forbidden(field.NewPath("spec", "workload", "kind"), "Workload kind is immutable. Delete and recreate the ServiceBinding to update."),
		)
	}

	// validate new object
	errs = append(errs, obj.validate()...)

	return nil, errs.ToAggregate()
}

// ValidateDelete implements webhook.CustomValidator so a webhook will be registered for the type
func (*ServiceBinding) ValidateDelete(ctx context.Context, obj *ServiceBinding) (admission.Warnings, error) {
	log := logr.FromContextOrDiscard(ctx)
	log.V(1).Info("Validating Delete")

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

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

package v1alpha3

import (
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

func (r *ServiceBinding) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

var _ webhook.Defaulter = &ServiceBinding{}

// Default implements webhook.Defaulter so a webhook will be registered for the type
func (r *ServiceBinding) Default() {
	if r.Spec.Name == "" {
		r.Spec.Name = r.Name
	}
}

//+kubebuilder:webhook:path=/validate-servicebinding-io-v1alpha3-servicebinding,mutating=false,failurePolicy=fail,sideEffects=None,groups=servicebinding.io,resources=servicebindings,verbs=create;update,versions=v1alpha3,name=vservicebinding.kb.io,admissionReviewVersions={v1,v1beta1}

var _ webhook.Validator = &ServiceBinding{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *ServiceBinding) ValidateCreate() error {
	return r.validate().ToAggregate()
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *ServiceBinding) ValidateUpdate(old runtime.Object) error {
	// TODO(user): check for immutable fields, if any
	return r.validate().ToAggregate()
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *ServiceBinding) ValidateDelete() error {
	return nil
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

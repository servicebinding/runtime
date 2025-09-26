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

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/client-go/util/jsonpath"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

func (r *ClusterWorkloadResourceMapping) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		WithDefaulter(r).
		WithValidator(r).
		Complete()
}

var _ webhook.CustomDefaulter = &ClusterWorkloadResourceMapping{}

// Default implements webhook.CustomDefaulter so a webhook will be registered for the type
func (r *ClusterWorkloadResourceMapping) Default(ctx context.Context, obj runtime.Object) error {
	r = obj.(*ClusterWorkloadResourceMapping)

	for i := range r.Spec.Versions {
		r.Spec.Versions[i].Default()
	}

	return nil
}

// Default applies values that are appropriate for a PodSpecable resource
func (r *ClusterWorkloadResourceMappingTemplate) Default() {
	if r.Annotations == "" {
		r.Annotations = ".spec.template.metadata.annotations"
	}
	if len(r.Containers) == 0 {
		r.Containers = []ClusterWorkloadResourceMappingContainer{
			{
				Path: ".spec.template.spec.initContainers[*]",
				Name: ".name",
			},
			{
				Path: ".spec.template.spec.containers[*]",
				Name: ".name",
			},
		}
	}
	for i := range r.Containers {
		c := &r.Containers[i]
		if c.Env == "" {
			c.Env = ".env"
		}
		if c.VolumeMounts == "" {
			c.VolumeMounts = ".volumeMounts"
		}
	}
	if r.Volumes == "" {
		r.Volumes = ".spec.template.spec.volumes"
	}
}

//+kubebuilder:webhook:path=/validate-servicebinding-io-v1-clusterworkloadresourcemapping,mutating=false,failurePolicy=fail,sideEffects=None,groups=servicebinding.io,resources=clusterworkloadresourcemappings,verbs=create;update,versions=v1,name=v1.clusterworkloadresourcemappings.servicebinding.io,admissionReviewVersions={v1,v1beta1}

var _ webhook.CustomValidator = &ClusterWorkloadResourceMapping{}

// ValidateCreate implements webhook.CustomValidator so a webhook will be registered for the type
func (r *ClusterWorkloadResourceMapping) ValidateCreate(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	log := logr.FromContextOrDiscard(ctx)
	log.V(1).Info("Validating Create")

	r = obj.(*ClusterWorkloadResourceMapping)

	(&ClusterWorkloadResourceMapping{}).Default(ctx, r)
	return nil, r.validate().ToAggregate()
}

// ValidateUpdate implements webhook.CustomValidator so a webhook will be registered for the type
func (r *ClusterWorkloadResourceMapping) ValidateUpdate(ctx context.Context, old, obj runtime.Object) (admission.Warnings, error) {
	log := logr.FromContextOrDiscard(ctx)
	log.V(1).Info("Validating Update")

	r = obj.(*ClusterWorkloadResourceMapping)

	(&ClusterWorkloadResourceMapping{}).Default(ctx, r)
	// TODO(user): check for immutable fields, if any
	return nil, r.validate().ToAggregate()
}

// ValidateDelete implements webhook.CustomValidator so a webhook will be registered for the type
func (r *ClusterWorkloadResourceMapping) ValidateDelete(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	log := logr.FromContextOrDiscard(ctx)
	log.V(1).Info("Validating Delete")

	return nil, nil
}

func (r *ClusterWorkloadResourceMapping) validate() field.ErrorList {
	errs := field.ErrorList{}

	versions := map[string]int{}
	for i := range r.Spec.Versions {
		// check for duplicate versions
		if p, ok := versions[r.Spec.Versions[i].Version]; ok {
			errs = append(errs, field.Duplicate(field.NewPath("spec", "versions", fmt.Sprintf("[%d, %d]", p, i), "version"), r.Spec.Versions[i].Version))
		}
		versions[r.Spec.Versions[i].Version] = i
		errs = append(errs, r.Spec.Versions[i].validate(field.NewPath("spec", "versions").Index(i))...)
	}

	return errs
}

func (r *ClusterWorkloadResourceMappingTemplate) validate(fldPath *field.Path) field.ErrorList {
	errs := field.ErrorList{}

	if r.Version == "" {
		errs = append(errs, field.Required(fldPath.Child("version"), ""))
	}
	errs = append(errs, validateRestrictedJsonPath(r.Annotations, fldPath.Child("annotations"))...)
	errs = append(errs, validateRestrictedJsonPath(r.Volumes, fldPath.Child("volumes"))...)
	for i := range r.Containers {
		errs = append(errs, r.Containers[i].validate(fldPath.Child("containers").Index(i))...)
	}

	return errs
}

func (r *ClusterWorkloadResourceMappingContainer) validate(fldPath *field.Path) field.ErrorList {
	errs := field.ErrorList{}

	errs = append(errs, validateJsonPath(r.Path, fldPath.Child("path"))...)
	if r.Name != "" {
		// name is optional
		errs = append(errs, validateRestrictedJsonPath(r.Name, fldPath.Child("name"))...)
	}
	errs = append(errs, validateRestrictedJsonPath(r.Env, fldPath.Child("env"))...)
	errs = append(errs, validateRestrictedJsonPath(r.VolumeMounts, fldPath.Child("volumeMounts"))...)

	return errs
}

func validateJsonPath(expression string, fldPath *field.Path) field.ErrorList {
	errs := field.ErrorList{}

	if p, err := jsonpath.Parse("", fmt.Sprintf("{%s}", expression)); err != nil {
		errs = append(errs, field.Invalid(fldPath, expression, err.Error()))
	} else {
		if len(p.Root.Nodes) != 1 {
			errs = append(errs, field.Invalid(fldPath, expression, "too many root nodes"))
		}
	}

	return errs
}

func validateRestrictedJsonPath(expression string, fldPath *field.Path) field.ErrorList {
	errs := field.ErrorList{}

	if p, err := jsonpath.Parse("", fmt.Sprintf("{%s}", expression)); err != nil {
		errs = append(errs, field.Invalid(fldPath, expression, err.Error()))
	} else {
		if len(p.Root.Nodes) != 1 {
			errs = append(errs, field.Invalid(fldPath, expression, "too many root nodes"))
		}
		// only allow jsonpath.NodeField nodes
		nodes := p.Root.Nodes
		for i := 0; i < len(nodes); i++ {
			switch n := nodes[i].(type) {
			case *jsonpath.ListNode:
				nodes = append(nodes, n.Nodes...)
			case *jsonpath.FieldNode:
				continue
			default:
				errs = append(errs, field.Invalid(fldPath, expression, fmt.Sprintf("unsupported node: %s", n)))
			}
		}
	}

	return errs
}

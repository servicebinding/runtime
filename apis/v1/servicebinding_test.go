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
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

func TestServiceBindingDefault(t *testing.T) {
	tests := []struct {
		name     string
		seed     *ServiceBinding
		expected *ServiceBinding
	}{
		{
			name: "default name",
			seed: &ServiceBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name: "my-binding",
				},
			},
			expected: &ServiceBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name: "my-binding",
				},
				Spec: ServiceBindingSpec{
					Name: "my-binding",
				},
			},
		},
		{
			name: "preserve name",
			seed: &ServiceBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name: "my-binding",
				},
				Spec: ServiceBindingSpec{
					Name: "preserved-name",
				},
			},
			expected: &ServiceBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name: "my-binding",
				},
				Spec: ServiceBindingSpec{
					Name: "preserved-name",
				},
			},
		},
	}

	for _, c := range tests {
		t.Run(c.name, func(t *testing.T) {
			actual := c.seed.DeepCopy()
			(&ServiceBinding{}).Default(t.Context(), actual)
			if diff := cmp.Diff(c.expected, actual); diff != "" {
				t.Errorf("(-expected, +actual): %s", diff)
			}
		})
	}
}

// serviceBindingNamed returns a ServiceBinding that is valid other than .spec.name, which is set to
// the provided value. Used by the .spec.name validation cases below.
func serviceBindingNamed(name string) *ServiceBinding {
	return &ServiceBinding{
		Spec: ServiceBindingSpec{
			Name: name,
			Service: ServiceBindingServiceReference{
				APIVersion: "v1",
				Kind:       "Secret",
				Name:       "my-service",
			},
			Workload: ServiceBindingWorkloadReference{
				APIVersion: "apps/v1",
				Kind:       "Deployment",
				Name:       "my-workload",
			},
		},
	}
}

func TestServiceBindingValidate(t *testing.T) {
	tests := []struct {
		name     string
		seed     *ServiceBinding
		expected field.ErrorList
	}{
		{
			name: "empty is not valid",
			seed: &ServiceBinding{},
			expected: field.ErrorList{
				field.Required(field.NewPath("spec", "name"), ""),
				field.Required(field.NewPath("spec", "service", "apiVersion"), ""),
				field.Required(field.NewPath("spec", "service", "kind"), ""),
				field.Required(field.NewPath("spec", "service", "name"), ""),
				field.Required(field.NewPath("spec", "workload", "apiVersion"), ""),
				field.Required(field.NewPath("spec", "workload", "kind"), ""),
				field.Required(field.NewPath("spec", "workload", "[name, selector]"), "expected exactly one, got neither"),
			},
		},
		{
			name: "workload valid",
			seed: &ServiceBinding{
				Spec: ServiceBindingSpec{
					Name: "my-binding",
					Service: ServiceBindingServiceReference{
						APIVersion: "v1",
						Kind:       "Secret",
						Name:       "my-service",
					},
					Workload: ServiceBindingWorkloadReference{
						APIVersion: "apps/v1",
						Kind:       "Deloyment",
						Name:       "my-workload",
					},
				},
			},
			expected: field.ErrorList{},
		},
		{
			name: "workload valid selector",
			seed: &ServiceBinding{
				Spec: ServiceBindingSpec{
					Name: "my-binding",
					Service: ServiceBindingServiceReference{
						APIVersion: "v1",
						Kind:       "Secret",
						Name:       "my-service",
					},
					Workload: ServiceBindingWorkloadReference{
						APIVersion: "apps/v1",
						Kind:       "Deloyment",
						Selector:   &metav1.LabelSelector{},
					},
				},
			},
			expected: field.ErrorList{},
		},
		{
			name: "workload invalid selector",
			seed: &ServiceBinding{
				Spec: ServiceBindingSpec{
					Name: "my-binding",
					Service: ServiceBindingServiceReference{
						APIVersion: "v1",
						Kind:       "Secret",
						Name:       "my-service",
					},
					Workload: ServiceBindingWorkloadReference{
						APIVersion: "apps/v1",
						Kind:       "Deloyment",
						Selector: &metav1.LabelSelector{
							MatchExpressions: []metav1.LabelSelectorRequirement{{
								Key:      "foo",
								Operator: "NotAnOperator",
								Values:   []string{"bar"},
							}},
						},
					},
				},
			},
			expected: field.ErrorList{
				field.Invalid(field.NewPath("spec", "workload", "selector"), &metav1.LabelSelector{
					MatchExpressions: []metav1.LabelSelectorRequirement{{
						Key:      "foo",
						Operator: "NotAnOperator",
						Values:   []string{"bar"},
					}},
				}, `"NotAnOperator" is not a valid label selector operator`),
			},
		},
		{
			name: "workload invalid overspeced",
			seed: &ServiceBinding{
				Spec: ServiceBindingSpec{
					Name: "my-binding",
					Service: ServiceBindingServiceReference{
						APIVersion: "v1",
						Kind:       "Secret",
						Name:       "my-service",
					},
					Workload: ServiceBindingWorkloadReference{
						APIVersion: "apps/v1",
						Kind:       "Deloyment",
						Name:       "my-workload",
						Selector:   &metav1.LabelSelector{},
					},
				},
			},
			expected: field.ErrorList{
				field.Required(field.NewPath("spec", "workload", "[name, selector]"), "expected exactly one, got both"),
			},
		},
		{
			name: "workload valid env",
			seed: &ServiceBinding{
				Spec: ServiceBindingSpec{
					Name: "my-binding",
					Service: ServiceBindingServiceReference{
						APIVersion: "v1",
						Kind:       "Secret",
						Name:       "my-service",
					},
					Workload: ServiceBindingWorkloadReference{
						APIVersion: "apps/v1",
						Kind:       "Deloyment",
						Name:       "my-workload",
					},
					Env: []EnvMapping{
						{
							Name: "VAR_NAME",
							Key:  "secret-key",
						},
					},
				},
			},
			expected: field.ErrorList{},
		},
		{
			name: "workload invalid env",
			seed: &ServiceBinding{
				Spec: ServiceBindingSpec{
					Name: "my-binding",
					Service: ServiceBindingServiceReference{
						APIVersion: "v1",
						Kind:       "Secret",
						Name:       "my-service",
					},
					Workload: ServiceBindingWorkloadReference{
						APIVersion: "apps/v1",
						Kind:       "Deloyment",
						Name:       "my-workload",
					},
					Env: []EnvMapping{
						{
							Name: "VAR_NAME",
							Key:  "secret-key",
						},
						{
							// missing fields
						},
					},
				},
			},
			expected: field.ErrorList{
				field.Required(field.NewPath("spec", "env[1]", "name"), ""),
				field.Required(field.NewPath("spec", "env[1]", "key"), ""),
			},
		},

		// .spec.name is projected into the workload as a volume mount directory at
		// $SERVICE_BINDING_ROOT/<name>. The spec requires binding names to match
		// [a-z0-9\-\.]{1,253}; "." and ".." additionally escape the binding root.

		{
			name:     "name valid",
			seed:     serviceBindingNamed("my-binding"),
			expected: field.ErrorList{},
		},
		{
			name:     "name valid leading hyphen",
			seed:     serviceBindingNamed("-foo"),
			expected: field.ErrorList{},
		},
		{
			name:     "name valid trailing hyphen",
			seed:     serviceBindingNamed("foo-"),
			expected: field.ErrorList{},
		},
		{
			name:     "name valid consecutive dots",
			seed:     serviceBindingNamed("foo..bar"),
			expected: field.ErrorList{},
		},
		{
			name:     "name valid trailing dot",
			seed:     serviceBindingNamed("foo."),
			expected: field.ErrorList{},
		},
		{
			name:     "name valid max length",
			seed:     serviceBindingNamed(strings.Repeat("a", 253)),
			expected: field.ErrorList{},
		},
		{
			name: "name invalid parent directory",
			seed: serviceBindingNamed(".."),
			expected: field.ErrorList{
				field.Invalid(field.NewPath("spec", "name"), "..", `must not be "." or ".."`),
			},
		},
		{
			name: "name invalid current directory",
			seed: serviceBindingNamed("."),
			expected: field.ErrorList{
				field.Invalid(field.NewPath("spec", "name"), ".", `must not be "." or ".."`),
			},
		},
		{
			name: "name invalid path traversal",
			seed: serviceBindingNamed("../../etc"),
			expected: field.ErrorList{
				field.Invalid(field.NewPath("spec", "name"), "../../etc", bindingNameErrMsg),
			},
		},
		{
			name: "name invalid uppercase",
			seed: serviceBindingNamed("Foo"),
			expected: field.ErrorList{
				field.Invalid(field.NewPath("spec", "name"), "Foo", bindingNameErrMsg),
			},
		},
		{
			name: "name invalid underscore",
			seed: serviceBindingNamed("foo_bar"),
			expected: field.ErrorList{
				field.Invalid(field.NewPath("spec", "name"), "foo_bar", bindingNameErrMsg),
			},
		},
		{
			name: "name invalid too long",
			seed: serviceBindingNamed(strings.Repeat("a", 254)),
			expected: field.ErrorList{
				field.Invalid(field.NewPath("spec", "name"), strings.Repeat("a", 254), bindingNameErrMsg),
			},
		},
	}

	for _, c := range tests {
		t.Run(c.name, func(t *testing.T) {
			if diff := cmp.Diff(c.expected, c.seed.validate(nil)); diff != "" {
				t.Errorf("validate (-expected, +actual): %s", diff)
			}

			expectedErr := c.expected.ToAggregate()

			_, actualCreateErr := (&ServiceBinding{}).ValidateCreate(t.Context(), c.seed.DeepCopy())
			if diff := cmp.Diff(expectedErr, actualCreateErr); diff != "" {
				t.Errorf("ValidateCreate (-expected, +actual): %s", diff)
			}

			// the old object carries a different .spec.name so that name validation is not
			// ratcheted, i.e. these cases assert the rules applied to a newly introduced value.
			// Ratcheting itself is covered by TestServiceBindingValidate_RatchetName.
			old := c.seed.DeepCopy()
			old.Spec.Name = "previous-name"

			_, actualUpdateErr := (&ServiceBinding{}).ValidateUpdate(t.Context(), old, c.seed.DeepCopy())
			if diff := cmp.Diff(expectedErr, actualUpdateErr); diff != "" {
				t.Errorf("ValidateUpdate (-expected, +actual): %s", diff)
			}

			_, actualDeleteErr := (&ServiceBinding{}).ValidateDelete(t.Context(), c.seed.DeepCopy())
			if diff := cmp.Diff(nil, actualDeleteErr); diff != "" {
				t.Errorf("ValidateDelete (-expected, +actual): %s", diff)
			}
		})
	}
}

// A ServiceBinding that omits .spec.name is valid: the webhook defaults the field from
// .metadata.name before validating it. Kubernetes constrains .metadata.name to a DNS-1123
// subdomain, which the binding name pattern always admits, so a defaulted name cannot escape the
// binding root. This is why validating .spec.name at admission is sufficient.
func TestServiceBindingValidate_DefaultedName(t *testing.T) {
	tests := []struct {
		name     string
		metaName string
	}{
		{name: "simple", metaName: "my-binding"},
		{name: "dotted", metaName: "a.b.c"},
		{name: "max length", metaName: strings.Repeat("a", 253)},
	}

	for _, c := range tests {
		t.Run(c.name, func(t *testing.T) {
			seed := serviceBindingNamed("")
			seed.ObjectMeta = metav1.ObjectMeta{Name: c.metaName}

			obj := seed.DeepCopy()
			if _, err := (&ServiceBinding{}).ValidateCreate(t.Context(), obj); err != nil {
				t.Errorf("ValidateCreate: unexpected error: %s", err)
			}
			if obj.Spec.Name != c.metaName {
				t.Errorf("expected .spec.name defaulted to %q, got %q", c.metaName, obj.Spec.Name)
			}

			if _, err := (&ServiceBinding{}).ValidateUpdate(t.Context(), seed.DeepCopy(), seed.DeepCopy()); err != nil {
				t.Errorf("ValidateUpdate: unexpected error: %s", err)
			}
		})
	}
}

// .spec.name validation is ratcheted on update: a value that predates this validation does not by
// itself block writes to the object. Deleting a ServiceBinding requires clearing its finalizer,
// which is an UPDATE, so rejecting an unchanged name would leave objects created before this
// validation stuck terminating. Introducing or changing to an invalid name is still rejected.
func TestServiceBindingValidate_RatchetName(t *testing.T) {
	tests := []struct {
		name      string
		oldName   string
		newName   string
		expectErr bool
	}{
		{name: "unchanged invalid name is allowed", oldName: "Legacy_Name", newName: "Legacy_Name", expectErr: false},
		{name: "unchanged traversal name is allowed, so it can be deleted", oldName: "..", newName: "..", expectErr: false},
		{name: "changed to another invalid name is rejected", oldName: "Legacy_Name", newName: "Other_Bad", expectErr: true},
		{name: "changed to a valid name is allowed", oldName: "Legacy_Name", newName: "legacy-name", expectErr: false},
		{name: "newly introduced traversal is rejected", oldName: "good-name", newName: "../../etc", expectErr: true},
		{name: "valid name unchanged is allowed", oldName: "good-name", newName: "good-name", expectErr: false},
	}

	for _, c := range tests {
		t.Run(c.name, func(t *testing.T) {
			old := serviceBindingNamed(c.oldName)
			obj := serviceBindingNamed(c.newName)

			_, err := (&ServiceBinding{}).ValidateUpdate(t.Context(), old, obj)
			if c.expectErr && err == nil {
				t.Errorf("ValidateUpdate: expected an error, got none")
			}
			if !c.expectErr && err != nil {
				t.Errorf("ValidateUpdate: unexpected error: %s", err)
			}

			// creating the same object outright is always validated, never ratcheted
			if _, err := (&ServiceBinding{}).ValidateCreate(t.Context(), serviceBindingNamed(c.newName)); err == nil && c.newName != "legacy-name" && c.newName != "good-name" {
				t.Errorf("ValidateCreate(%q): expected an error, got none", c.newName)
			}
		})
	}
}

func TestServiceBindingValidate_Immutable(t *testing.T) {
	tests := []struct {
		name     string
		seed     *ServiceBinding
		old      *ServiceBinding
		expected field.ErrorList
	}{
		{
			name: "allow update workload name",
			seed: &ServiceBinding{
				Spec: ServiceBindingSpec{
					Name: "my-binding",
					Service: ServiceBindingServiceReference{
						APIVersion: "v1",
						Kind:       "Secret",
						Name:       "my-service",
					},
					Workload: ServiceBindingWorkloadReference{
						APIVersion: "apps/v1",
						Kind:       "Deloyment",
						Name:       "new-workload",
					},
				},
			},
			old: &ServiceBinding{
				Spec: ServiceBindingSpec{
					Name: "my-binding",
					Service: ServiceBindingServiceReference{
						APIVersion: "v1",
						Kind:       "Secret",
						Name:       "my-service",
					},
					Workload: ServiceBindingWorkloadReference{
						APIVersion: "apps/v1",
						Kind:       "Deloyment",
						Name:       "old-workload",
					},
				},
			},
			expected: field.ErrorList{},
		},
		{
			name: "reject update workload apiVersion",
			seed: &ServiceBinding{
				Spec: ServiceBindingSpec{
					Name: "my-binding",
					Service: ServiceBindingServiceReference{
						APIVersion: "v1",
						Kind:       "Secret",
						Name:       "my-service",
					},
					Workload: ServiceBindingWorkloadReference{
						APIVersion: "apps/v1",
						Kind:       "Deloyment",
						Name:       "my-workload",
					},
				},
			},
			old: &ServiceBinding{
				Spec: ServiceBindingSpec{
					Name: "my-binding",
					Service: ServiceBindingServiceReference{
						APIVersion: "v1",
						Kind:       "Secret",
						Name:       "my-service",
					},
					Workload: ServiceBindingWorkloadReference{
						APIVersion: "extensions/v1beta1",
						Kind:       "Deloyment",
						Name:       "my-workload",
					},
				},
			},
			expected: field.ErrorList{
				{
					Type:     field.ErrorTypeForbidden,
					Field:    "spec.workload.apiVersion",
					Detail:   "Workload apiVersion is immutable. Delete and recreate the ServiceBinding to update.",
					BadValue: "",
				},
			},
		},
		{
			name: "reject update workload kind",
			seed: &ServiceBinding{
				Spec: ServiceBindingSpec{
					Name: "my-binding",
					Service: ServiceBindingServiceReference{
						APIVersion: "v1",
						Kind:       "Secret",
						Name:       "my-service",
					},
					Workload: ServiceBindingWorkloadReference{
						APIVersion: "apps/v1",
						Kind:       "Deloyment",
						Name:       "my-workload",
					},
				},
			},
			old: &ServiceBinding{
				Spec: ServiceBindingSpec{
					Name: "my-binding",
					Service: ServiceBindingServiceReference{
						APIVersion: "v1",
						Kind:       "Secret",
						Name:       "my-service",
					},
					Workload: ServiceBindingWorkloadReference{
						APIVersion: "apps/v1",
						Kind:       "StatefulSet",
						Name:       "my-workload",
					},
				},
			},
			expected: field.ErrorList{
				{
					Type:     field.ErrorTypeForbidden,
					Field:    "spec.workload.kind",
					Detail:   "Workload kind is immutable. Delete and recreate the ServiceBinding to update.",
					BadValue: "",
				},
			},
		},
	}

	for _, c := range tests {
		t.Run(c.name, func(t *testing.T) {
			expectedErr := c.expected.ToAggregate()

			_, actualUpdateErr := (&ServiceBinding{}).ValidateUpdate(t.Context(), c.old, c.seed)
			if diff := cmp.Diff(expectedErr, actualUpdateErr); diff != "" {
				t.Errorf("ValidateCreate (-expected, +actual): %s", diff)
			}
		})
	}
}

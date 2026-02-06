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
	}

	for _, c := range tests {
		t.Run(c.name, func(t *testing.T) {
			if diff := cmp.Diff(c.expected, c.seed.validate()); diff != "" {
				t.Errorf("validate (-expected, +actual): %s", diff)
			}

			expectedErr := c.expected.ToAggregate()

			_, actualCreateErr := (&ServiceBinding{}).ValidateCreate(t.Context(), c.seed.DeepCopy())
			if diff := cmp.Diff(expectedErr, actualCreateErr); diff != "" {
				t.Errorf("ValidateCreate (-expected, +actual): %s", diff)
			}

			_, actualUpdateErr := (&ServiceBinding{}).ValidateUpdate(t.Context(), c.seed.DeepCopy(), c.seed.DeepCopy())
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

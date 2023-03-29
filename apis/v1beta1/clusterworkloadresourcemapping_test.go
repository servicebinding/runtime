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

package v1beta1

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

func TestClusterWorkloadResourceMappingDefault(t *testing.T) {
	tests := []struct {
		name     string
		seed     *ClusterWorkloadResourceMapping
		expected *ClusterWorkloadResourceMapping
	}{
		{
			name: "podspecable defaults",
			seed: &ClusterWorkloadResourceMapping{
				Spec: ClusterWorkloadResourceMappingSpec{
					Versions: []ClusterWorkloadResourceMappingTemplate{
						{
							Version: "*",
						},
					},
				},
			},
			expected: &ClusterWorkloadResourceMapping{
				Spec: ClusterWorkloadResourceMappingSpec{
					Versions: []ClusterWorkloadResourceMappingTemplate{
						{
							Version:     "*",
							Annotations: ".spec.template.metadata.annotations",
							Containers: []ClusterWorkloadResourceMappingContainer{
								{
									Path:         ".spec.template.spec.initContainers[*]",
									Name:         ".name",
									Env:          ".env",
									VolumeMounts: ".volumeMounts",
								},
								{
									Path:         ".spec.template.spec.containers[*]",
									Name:         ".name",
									Env:          ".env",
									VolumeMounts: ".volumeMounts",
								},
							},
							Volumes: ".spec.template.spec.volumes",
						},
					},
				},
			},
		},
		{
			name: "cronjob defaults",
			seed: &ClusterWorkloadResourceMapping{
				Spec: ClusterWorkloadResourceMappingSpec{
					Versions: []ClusterWorkloadResourceMappingTemplate{
						{
							Version:     "*",
							Annotations: ".spec.jobTemplate.spec.template.metadata.annotations",
							Containers: []ClusterWorkloadResourceMappingContainer{
								{
									Path: ".spec.jobTemplate.spec.template.spec.initContainers[*]",
									Name: ".name",
								},
								{
									Path: ".spec.jobTemplate.spec.template.spec.containers[*]",
									Name: ".name",
								},
							},
							Volumes: ".spec.jobTemplate.spec.template.spec.volumes",
						},
					},
				},
			},
			expected: &ClusterWorkloadResourceMapping{
				Spec: ClusterWorkloadResourceMappingSpec{
					Versions: []ClusterWorkloadResourceMappingTemplate{
						{
							Version:     "*",
							Annotations: ".spec.jobTemplate.spec.template.metadata.annotations",
							Containers: []ClusterWorkloadResourceMappingContainer{
								{
									Path:         ".spec.jobTemplate.spec.template.spec.initContainers[*]",
									Name:         ".name",
									Env:          ".env",
									VolumeMounts: ".volumeMounts",
								},
								{
									Path:         ".spec.jobTemplate.spec.template.spec.containers[*]",
									Name:         ".name",
									Env:          ".env",
									VolumeMounts: ".volumeMounts",
								},
							},
							Volumes: ".spec.jobTemplate.spec.template.spec.volumes",
						},
					},
				},
			},
		},
		{
			name: "runtimecomponent defaults",
			seed: &ClusterWorkloadResourceMapping{
				Spec: ClusterWorkloadResourceMappingSpec{
					Versions: []ClusterWorkloadResourceMappingTemplate{
						{
							Version:     "v1beta1",
							Annotations: ".metadata.annotations",
							Containers: []ClusterWorkloadResourceMappingContainer{
								{
									Path: ".spec",
								},
								{
									Path: ".spec.initContainers[*]",
									Name: ".name",
								},
								{
									Path: ".spec.sidecarContainers[*]",
									Name: ".name",
								},
							},
							Volumes: ".spec.volumes",
						},
					},
				},
			},
			expected: &ClusterWorkloadResourceMapping{
				Spec: ClusterWorkloadResourceMappingSpec{
					Versions: []ClusterWorkloadResourceMappingTemplate{
						{
							Version:     "v1beta1",
							Annotations: ".metadata.annotations",
							Containers: []ClusterWorkloadResourceMappingContainer{
								{
									Path:         ".spec",
									Env:          ".env",
									VolumeMounts: ".volumeMounts",
								},
								{
									Path:         ".spec.initContainers[*]",
									Name:         ".name",
									Env:          ".env",
									VolumeMounts: ".volumeMounts",
								},
								{
									Path:         ".spec.sidecarContainers[*]",
									Name:         ".name",
									Env:          ".env",
									VolumeMounts: ".volumeMounts",
								},
							},
							Volumes: ".spec.volumes",
						},
					},
				},
			},
		},
		{
			name: "fully speced",
			seed: &ClusterWorkloadResourceMapping{
				Spec: ClusterWorkloadResourceMappingSpec{
					Versions: []ClusterWorkloadResourceMappingTemplate{
						{
							Version:     "*",
							Annotations: ".annotations",
							Containers: []ClusterWorkloadResourceMappingContainer{
								{
									Path:         ".containers[*]",
									Env:          ".env",
									VolumeMounts: ".volumeMounts",
								},
							},
							Volumes: ".volumes",
						},
					},
				},
			},
			expected: &ClusterWorkloadResourceMapping{
				Spec: ClusterWorkloadResourceMappingSpec{
					Versions: []ClusterWorkloadResourceMappingTemplate{
						{
							Version:     "*",
							Annotations: ".annotations",
							Containers: []ClusterWorkloadResourceMappingContainer{
								{
									Path:         ".containers[*]",
									Env:          ".env",
									VolumeMounts: ".volumeMounts",
								},
							},
							Volumes: ".volumes",
						},
					},
				},
			},
		},
	}

	for _, c := range tests {
		t.Run(c.name, func(t *testing.T) {
			actual := c.seed.DeepCopy()
			actual.Default()
			if diff := cmp.Diff(c.expected, actual); diff != "" {
				t.Errorf("(-expected, +actual): %s", diff)
			}
		})
	}
}
func TestClusterWorkloadResourceMappingValidate(t *testing.T) {
	tests := []struct {
		name     string
		seed     *ClusterWorkloadResourceMapping
		expected field.ErrorList
	}{
		{
			name:     "empty is valid",
			seed:     &ClusterWorkloadResourceMapping{},
			expected: field.ErrorList{},
		},
		{
			name: "wildcard version is valid",
			seed: &ClusterWorkloadResourceMapping{
				Spec: ClusterWorkloadResourceMappingSpec{
					Versions: []ClusterWorkloadResourceMappingTemplate{
						{
							Version: "*",
						},
					},
				},
			},
			expected: field.ErrorList{},
		},
		{
			name: "duplicate version is invalid",
			seed: &ClusterWorkloadResourceMapping{
				Spec: ClusterWorkloadResourceMappingSpec{
					Versions: []ClusterWorkloadResourceMappingTemplate{
						{
							Version: "*",
						},
						{
							Version: "*",
						},
					},
				},
			},
			expected: field.ErrorList{
				field.Duplicate(field.NewPath("spec.versions.[0, 1].version"), "*"),
			},
		},
		{
			name: "missing version is invalid",
			seed: &ClusterWorkloadResourceMapping{
				Spec: ClusterWorkloadResourceMappingSpec{
					Versions: []ClusterWorkloadResourceMappingTemplate{
						{
							Version: "",
						},
					},
				},
			},
			expected: field.ErrorList{
				field.Required(field.NewPath("spec.versions[0].version"), ""),
			},
		},
		{
			name: "allow container path to use unrestricted synax",
			seed: &ClusterWorkloadResourceMapping{
				Spec: ClusterWorkloadResourceMappingSpec{
					Versions: []ClusterWorkloadResourceMappingTemplate{
						{
							Version: "*",
							Containers: []ClusterWorkloadResourceMappingContainer{
								{
									Path: "..",
								},
							},
						},
					},
				},
			},
			expected: field.ErrorList{},
		},
		{
			name: "invalid container path",
			seed: &ClusterWorkloadResourceMapping{
				Spec: ClusterWorkloadResourceMappingSpec{
					Versions: []ClusterWorkloadResourceMappingTemplate{
						{
							Version: "*",
							Containers: []ClusterWorkloadResourceMappingContainer{
								{
									Path: "}{",
								},
							},
						},
					},
				},
			},
			expected: field.ErrorList{
				field.Invalid(field.NewPath("spec.versions[0].containers[0].path"), "}{", "too many root nodes"),
			},
		},
		{
			name: "invalid container name",
			seed: &ClusterWorkloadResourceMapping{
				Spec: ClusterWorkloadResourceMappingSpec{
					Versions: []ClusterWorkloadResourceMappingTemplate{
						{
							Version: "*",
							Containers: []ClusterWorkloadResourceMappingContainer{
								{
									Name: "..",
								},
							},
						},
					},
				},
			},
			expected: field.ErrorList{
				field.Invalid(field.NewPath("spec.versions[0].containers[0].name"), "..", "unsupported node: NodeRecursive"),
			},
		},
		{
			name: "invalid container env",
			seed: &ClusterWorkloadResourceMapping{
				Spec: ClusterWorkloadResourceMappingSpec{
					Versions: []ClusterWorkloadResourceMappingTemplate{
						{
							Version: "*",
							Containers: []ClusterWorkloadResourceMappingContainer{
								{
									Env: "..",
								},
							},
						},
					},
				},
			},
			expected: field.ErrorList{
				field.Invalid(field.NewPath("spec.versions[0].containers[0].env"), "..", "unsupported node: NodeRecursive"),
			},
		},
		{
			name: "invalid container volumeMounts",
			seed: &ClusterWorkloadResourceMapping{
				Spec: ClusterWorkloadResourceMappingSpec{
					Versions: []ClusterWorkloadResourceMappingTemplate{
						{
							Version: "*",
							Containers: []ClusterWorkloadResourceMappingContainer{
								{
									VolumeMounts: "..",
								},
							},
						},
					},
				},
			},
			expected: field.ErrorList{
				field.Invalid(field.NewPath("spec.versions[0].containers[0].volumeMounts"), "..", "unsupported node: NodeRecursive"),
			},
		},
		{
			name: "invalid annotations",
			seed: &ClusterWorkloadResourceMapping{
				Spec: ClusterWorkloadResourceMappingSpec{
					Versions: []ClusterWorkloadResourceMappingTemplate{
						{
							Version:     "*",
							Annotations: "..",
						},
					},
				},
			},
			expected: field.ErrorList{
				field.Invalid(field.NewPath("spec.versions[0].annotations"), "..", "unsupported node: NodeRecursive"),
			},
		},
		{
			name: "invalid volumes",
			seed: &ClusterWorkloadResourceMapping{
				Spec: ClusterWorkloadResourceMappingSpec{
					Versions: []ClusterWorkloadResourceMappingTemplate{
						{
							Version: "*",
							Volumes: "..",
						},
					},
				},
			},
			expected: field.ErrorList{
				field.Invalid(field.NewPath("spec.versions[0].volumes"), "..", "unsupported node: NodeRecursive"),
			},
		},
	}

	for _, c := range tests {
		t.Run(c.name, func(t *testing.T) {
			if diff := cmp.Diff(c.expected, c.seed.validate()); diff != "" {
				t.Errorf("validate (-expected, +actual): %s", diff)
			}

			expectedErr := c.expected.ToAggregate()

			_, actualCreateErr := c.seed.ValidateCreate()
			if diff := cmp.Diff(expectedErr, actualCreateErr); diff != "" {
				t.Errorf("ValidateCreate (-expected, +actual): %s", diff)
			}

			_, actualUpdateErr := c.seed.ValidateUpdate(c.seed.DeepCopy())
			if diff := cmp.Diff(expectedErr, actualUpdateErr); diff != "" {
				t.Errorf("ValidateCreate (-expected, +actual): %s", diff)
			}

			_, actualDeleteErr := c.seed.ValidateDelete()
			if diff := cmp.Diff(nil, actualDeleteErr); diff != "" {
				t.Errorf("ValidateDelete (-expected, +actual): %s", diff)
			}
		})
	}
}

func TestValidateJsonPath(t *testing.T) {
	fldPath := field.NewPath("test")
	tests := []struct {
		name       string
		expression string
		expected   field.ErrorList
	}{
		{
			name:       "valid",
			expression: "..",
			expected:   field.ErrorList{},
		},
		{
			name:       "multiple root nodes",
			expression: "}{",
			expected: field.ErrorList{
				field.Invalid(fldPath, "}{", "too many root nodes"),
			},
		},
		{
			name:       "invalid expression",
			expression: "[",
			expected: field.ErrorList{
				field.Invalid(fldPath, "[", "unterminated array"),
			},
		},
	}

	for _, c := range tests {
		t.Run(c.name, func(t *testing.T) {
			actual := validateJsonPath(c.expression, fldPath)
			if diff := cmp.Diff(c.expected, actual); diff != "" {
				t.Errorf("(-expected, +actual): %s", diff)
			}
		})
	}
}

func TestValidateRestrictedJsonPath(t *testing.T) {
	fldPath := field.NewPath("test")
	tests := []struct {
		name       string
		expression string
		expected   field.ErrorList
	}{
		{
			name:       "valid",
			expression: ".foo['bar']",
			expected:   field.ErrorList{},
		},
		{
			name:       "multiple root nodes",
			expression: "}{",
			expected: field.ErrorList{
				field.Invalid(fldPath, "}{", "too many root nodes"),
			},
		},
		{
			name:       "invalid expression",
			expression: "[",
			expected: field.ErrorList{
				field.Invalid(fldPath, "[", "unterminated array"),
			},
		},
		{
			name:       "text",
			expression: "'foo'",
			expected: field.ErrorList{
				field.Invalid(fldPath, "'foo'", "unsupported node: NodeText: foo"),
			},
		},
		{
			name:       "bool",
			expression: "true",
			expected: field.ErrorList{
				field.Invalid(fldPath, "true", "unsupported node: NodeBool: true"),
			},
		},
		{
			name:       "int",
			expression: "3",
			expected: field.ErrorList{
				field.Invalid(fldPath, "3", "unsupported node: NodeInt: 3"),
			},
		},
		{
			name:       "float",
			expression: "3.141592",
			expected: field.ErrorList{
				field.Invalid(fldPath, "3.141592", "unsupported node: NodeFloat: 3.141592"),
			},
		},
		{
			name:       "array",
			expression: "[0]",
			expected: field.ErrorList{
				field.Invalid(fldPath, "[0]", "unsupported node: NodeArray: [{0 true false} {1 true true} {0 false false}]"),
			},
		},
		{
			name:       "filter",
			expression: "[?(@.foo)]",
			expected: field.ErrorList{
				field.Invalid(fldPath, "[?(@.foo)]", "unsupported node: NodeFilter: NodeList exists NodeList"),
			},
		},
		{
			name:       "recursive",
			expression: "..",
			expected: field.ErrorList{
				field.Invalid(fldPath, "..", "unsupported node: NodeRecursive"),
			},
		},
		{
			name:       "wildcard",
			expression: ".*",
			expected: field.ErrorList{
				field.Invalid(fldPath, ".*", "unsupported node: NodeWildcard"),
			},
		},
		{
			name:       "union",
			expression: "[,]",
			expected: field.ErrorList{
				field.Invalid(fldPath, "[,]", "unsupported node: NodeUnion"),
			},
		},
		{
			name:       "identifier",
			expression: "foo",
			expected: field.ErrorList{
				field.Invalid(fldPath, "foo", "unsupported node: NodeIdentifier: foo"),
			},
		},
	}

	for _, c := range tests {
		t.Run(c.name, func(t *testing.T) {
			actual := validateRestrictedJsonPath(c.expression, fldPath)
			if diff := cmp.Diff(c.expected, actual); diff != "" {
				t.Errorf("(-expected, +actual): %s", diff)
			}
		})
	}
}

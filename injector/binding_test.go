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

package injector

import (
	"context"
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
	servicebindingv1alpha3 "github.com/servicebinding/service-binding-controller/apis/v1alpha3"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestBinding(t *testing.T) {
	tests := []struct {
		name        string
		mapping     MappingSource
		binding     *servicebindingv1alpha3.ServiceBinding
		workload    runtime.Object
		expected    runtime.Object
		expectedErr bool
	}{
		{
			name:    "podspecable",
			mapping: NewStubMapping(&servicebindingv1alpha3.ClusterWorkloadResourceMappingTemplate{}, nil),
			binding: &servicebindingv1alpha3.ServiceBinding{},
			workload: &appsv1.Deployment{
				Spec: appsv1.DeploymentSpec{
					Template: corev1.PodTemplateSpec{
						Spec: corev1.PodSpec{
							InitContainers: []corev1.Container{
								{
									Name: "init-hello",
								},
								{
									Name: "init-hello-2",
								},
							},
							Containers: []corev1.Container{
								{
									Name: "hello",
									Env: []corev1.EnvVar{
										{
											Name:  "SERVICE_BINDING_ROOT",
											Value: "/custom/path",
										},
									},
								},
								{
									Name: "hello-2",
								},
							},
						},
					},
				},
			},
			expected: &appsv1.Deployment{
				Spec: appsv1.DeploymentSpec{
					Template: corev1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Annotations: map[string]string{},
						},
						Spec: corev1.PodSpec{
							InitContainers: []corev1.Container{
								{
									Name: "init-hello",
									Env: []corev1.EnvVar{
										{
											Name:  "SERVICE_BINDING_ROOT",
											Value: "/bindings",
										},
									},
									VolumeMounts: []corev1.VolumeMount{},
								},
								{
									Name: "init-hello-2",
									Env: []corev1.EnvVar{
										{
											Name:  "SERVICE_BINDING_ROOT",
											Value: "/bindings",
										},
									},
									VolumeMounts: []corev1.VolumeMount{},
								},
							},
							Containers: []corev1.Container{
								{
									Name: "hello",
									Env: []corev1.EnvVar{
										{
											Name:  "SERVICE_BINDING_ROOT",
											Value: "/custom/path",
										},
									},
									VolumeMounts: []corev1.VolumeMount{},
								},
								{
									Name: "hello-2",
									Env: []corev1.EnvVar{
										{
											Name:  "SERVICE_BINDING_ROOT",
											Value: "/bindings",
										},
									},
									VolumeMounts: []corev1.VolumeMount{},
								},
							},
							Volumes: []corev1.Volume{},
						},
					},
				},
			},
		},
		{
			name: "almost podspecable",
			mapping: NewStubMapping(&servicebindingv1alpha3.ClusterWorkloadResourceMappingTemplate{
				Annotations: ".spec.jobTemplate.spec.template.metadata.annotations",
				Containers: []servicebindingv1alpha3.ClusterWorkloadResourceMappingContainer{
					{
						Path: ".spec.jobTemplate.spec.template.spec.containers[*]",
						Name: ".name",
					},
					{
						Path: ".spec.jobTemplate.spec.template.spec.initContainers[*]",
						Name: ".name",
					},
				},
				Volumes: ".spec.jobTemplate.spec.template.spec.volumes",
			}, nil),
			binding: &servicebindingv1alpha3.ServiceBinding{},
			workload: &batchv1.CronJob{
				Spec: batchv1.CronJobSpec{
					JobTemplate: batchv1.JobTemplateSpec{
						Spec: batchv1.JobSpec{
							Template: corev1.PodTemplateSpec{
								Spec: corev1.PodSpec{
									InitContainers: []corev1.Container{
										{
											Name: "init-hello",
										},
										{
											Name: "init-hello-2",
										},
									},
									Containers: []corev1.Container{
										{
											Name: "hello",
											Env: []corev1.EnvVar{
												{
													Name:  "SERVICE_BINDING_ROOT",
													Value: "/custom/path",
												},
											},
										},
										{
											Name: "hello-2",
										},
									},
								},
							},
						},
					},
				},
			},
			expected: &batchv1.CronJob{
				Spec: batchv1.CronJobSpec{
					JobTemplate: batchv1.JobTemplateSpec{
						Spec: batchv1.JobSpec{
							Template: corev1.PodTemplateSpec{
								ObjectMeta: metav1.ObjectMeta{
									Annotations: map[string]string{},
								},
								Spec: corev1.PodSpec{
									InitContainers: []corev1.Container{
										{
											Name: "init-hello",
											Env: []corev1.EnvVar{
												{
													Name:  "SERVICE_BINDING_ROOT",
													Value: "/bindings",
												},
											},
											VolumeMounts: []corev1.VolumeMount{},
										},
										{
											Name: "init-hello-2",
											Env: []corev1.EnvVar{
												{
													Name:  "SERVICE_BINDING_ROOT",
													Value: "/bindings",
												},
											},
											VolumeMounts: []corev1.VolumeMount{},
										},
									},
									Containers: []corev1.Container{
										{
											Name: "hello",
											Env: []corev1.EnvVar{
												{
													Name:  "SERVICE_BINDING_ROOT",
													Value: "/custom/path",
												},
											},
											VolumeMounts: []corev1.VolumeMount{},
										},
										{
											Name: "hello-2",
											Env: []corev1.EnvVar{
												{
													Name:  "SERVICE_BINDING_ROOT",
													Value: "/bindings",
												},
											},
											VolumeMounts: []corev1.VolumeMount{},
										},
									},
									Volumes: []corev1.Volume{},
								},
							},
						},
					},
				},
			},
		},
		{
			name:     "no containers",
			mapping:  NewStubMapping(&servicebindingv1alpha3.ClusterWorkloadResourceMappingTemplate{}, nil),
			binding:  &servicebindingv1alpha3.ServiceBinding{},
			workload: &appsv1.Deployment{},
			expected: &appsv1.Deployment{
				Spec: appsv1.DeploymentSpec{
					Template: corev1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Annotations: map[string]string{},
						},
						Spec: corev1.PodSpec{
							Volumes: []corev1.Volume{},
						},
					},
				},
			},
		},
		{
			name: "invalid container jsonpath",
			mapping: NewStubMapping(&servicebindingv1alpha3.ClusterWorkloadResourceMappingTemplate{
				Containers: []servicebindingv1alpha3.ClusterWorkloadResourceMappingContainer{
					{
						Path: "[",
					},
				},
			}, nil),
			binding:     &servicebindingv1alpha3.ServiceBinding{},
			workload:    &appsv1.Deployment{},
			expectedErr: true,
		},
		{
			name:        "conversion error",
			mapping:     NewStubMapping(&servicebindingv1alpha3.ClusterWorkloadResourceMappingTemplate{}, nil),
			workload:    &BadMarshalJSON{},
			expectedErr: true,
		},
	}

	for _, c := range tests {
		t.Run(c.name, func(t *testing.T) {
			ctx := context.TODO()

			actual := c.workload.DeepCopyObject().(client.Object)
			err := New(c.mapping).Bind(ctx, c.binding, actual)

			if (err != nil) != c.expectedErr {
				t.Errorf("Bind() expected err: %v", err)
			}
			if c.expectedErr {
				return
			}
			if diff := cmp.Diff(c.expected, actual); diff != "" {
				t.Errorf("Bind() (-expected, +actual): %s", diff)
			}
		})
	}
}

var (
	_ runtime.Object = (*BadMarshalJSON)(nil)
)

type BadMarshalJSON struct {
	metav1.TypeMeta
	metav1.ObjectMeta
}

func (r *BadMarshalJSON) MarshalJSON() ([]byte, error)   { return nil, fmt.Errorf("bad json marshal") }
func (r *BadMarshalJSON) DeepCopyObject() runtime.Object { return r }

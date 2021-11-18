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

package projector

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
	"k8s.io/apimachinery/pkg/types"
)

func TestBinding(t *testing.T) {
	uid := types.UID("26894874-4719-4802-8f43-8ceed127b4c2")
	bindingName := "my-binding"
	secretName := "my-secret"

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
			mapping: NewStaticMapping(&servicebindingv1alpha3.ClusterWorkloadResourceMappingTemplate{}),
			binding: &servicebindingv1alpha3.ServiceBinding{
				ObjectMeta: metav1.ObjectMeta{
					UID: uid,
				},
				Spec: servicebindingv1alpha3.ServiceBindingSpec{
					Name: bindingName,
				},
				Status: servicebindingv1alpha3.ServiceBindingStatus{
					Binding: &servicebindingv1alpha3.ServiceBindingSecretReference{
						Name: secretName,
					},
				},
			},
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
							Annotations: map[string]string{
								"projector.servicebinding.io/secret-26894874-4719-4802-8f43-8ceed127b4c2": secretName,
							},
						},
						Spec: corev1.PodSpec{
							Volumes: []corev1.Volume{
								{
									Name: "servicebinding-26894874-4719-4802-8f43-8ceed127b4c2",
									VolumeSource: corev1.VolumeSource{
										Projected: &corev1.ProjectedVolumeSource{
											Sources: []corev1.VolumeProjection{
												{
													Secret: &corev1.SecretProjection{
														LocalObjectReference: corev1.LocalObjectReference{
															Name: secretName,
														},
													},
												},
											},
										},
									},
								},
							},
							InitContainers: []corev1.Container{
								{
									Name: "init-hello",
									Env: []corev1.EnvVar{
										{
											Name:  "SERVICE_BINDING_ROOT",
											Value: "/bindings",
										},
									},
									VolumeMounts: []corev1.VolumeMount{
										{
											Name:      "servicebinding-26894874-4719-4802-8f43-8ceed127b4c2",
											ReadOnly:  true,
											MountPath: "/bindings/my-binding",
										},
									},
								},
								{
									Name: "init-hello-2",
									Env: []corev1.EnvVar{
										{
											Name:  "SERVICE_BINDING_ROOT",
											Value: "/bindings",
										},
									},
									VolumeMounts: []corev1.VolumeMount{
										{
											Name:      "servicebinding-26894874-4719-4802-8f43-8ceed127b4c2",
											ReadOnly:  true,
											MountPath: "/bindings/my-binding",
										},
									},
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
									VolumeMounts: []corev1.VolumeMount{
										{
											Name:      "servicebinding-26894874-4719-4802-8f43-8ceed127b4c2",
											ReadOnly:  true,
											MountPath: "/custom/path/my-binding",
										},
									},
								},
								{
									Name: "hello-2",
									Env: []corev1.EnvVar{
										{
											Name:  "SERVICE_BINDING_ROOT",
											Value: "/bindings",
										},
									},
									VolumeMounts: []corev1.VolumeMount{
										{
											Name:      "servicebinding-26894874-4719-4802-8f43-8ceed127b4c2",
											ReadOnly:  true,
											MountPath: "/bindings/my-binding",
										},
									},
								},
							},
						},
					},
				},
			},
		},
		{
			name: "almost podspecable",
			mapping: NewStaticMapping(&servicebindingv1alpha3.ClusterWorkloadResourceMappingTemplate{
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
			}),
			binding: &servicebindingv1alpha3.ServiceBinding{
				ObjectMeta: metav1.ObjectMeta{
					UID: uid,
				},
				Spec: servicebindingv1alpha3.ServiceBindingSpec{
					Name: bindingName,
				},
				Status: servicebindingv1alpha3.ServiceBindingStatus{
					Binding: &servicebindingv1alpha3.ServiceBindingSecretReference{
						Name: secretName,
					},
				},
			},
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
									Annotations: map[string]string{
										"projector.servicebinding.io/secret-26894874-4719-4802-8f43-8ceed127b4c2": "my-secret",
									},
								},
								Spec: corev1.PodSpec{
									Volumes: []corev1.Volume{
										{
											Name: "servicebinding-26894874-4719-4802-8f43-8ceed127b4c2",
											VolumeSource: corev1.VolumeSource{
												Projected: &corev1.ProjectedVolumeSource{
													Sources: []corev1.VolumeProjection{
														{
															Secret: &corev1.SecretProjection{
																LocalObjectReference: corev1.LocalObjectReference{
																	Name: "my-secret",
																},
															},
														},
													},
												},
											},
										},
									},
									InitContainers: []corev1.Container{
										{
											Name: "init-hello",
											Env: []corev1.EnvVar{
												{
													Name:  "SERVICE_BINDING_ROOT",
													Value: "/bindings",
												},
											},
											VolumeMounts: []corev1.VolumeMount{
												{
													Name:      "servicebinding-26894874-4719-4802-8f43-8ceed127b4c2",
													ReadOnly:  true,
													MountPath: "/bindings/my-binding",
												},
											},
										},
										{
											Name: "init-hello-2",
											Env: []corev1.EnvVar{
												{
													Name:  "SERVICE_BINDING_ROOT",
													Value: "/bindings",
												},
											},
											VolumeMounts: []corev1.VolumeMount{
												{
													Name:      "servicebinding-26894874-4719-4802-8f43-8ceed127b4c2",
													ReadOnly:  true,
													MountPath: "/bindings/my-binding",
												},
											},
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
											VolumeMounts: []corev1.VolumeMount{
												{
													Name:      "servicebinding-26894874-4719-4802-8f43-8ceed127b4c2",
													ReadOnly:  true,
													MountPath: "/custom/path/my-binding",
												},
											},
										},
										{
											Name: "hello-2",
											Env: []corev1.EnvVar{
												{
													Name:  "SERVICE_BINDING_ROOT",
													Value: "/bindings",
												},
											},
											VolumeMounts: []corev1.VolumeMount{
												{
													Name:      "servicebinding-26894874-4719-4802-8f43-8ceed127b4c2",
													ReadOnly:  true,
													MountPath: "/bindings/my-binding",
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
		{
			name:    "no containers",
			mapping: NewStaticMapping(&servicebindingv1alpha3.ClusterWorkloadResourceMappingTemplate{}),
			binding: &servicebindingv1alpha3.ServiceBinding{
				ObjectMeta: metav1.ObjectMeta{
					UID: uid,
				},
				Spec: servicebindingv1alpha3.ServiceBindingSpec{
					Name: bindingName,
				},
				Status: servicebindingv1alpha3.ServiceBindingStatus{
					Binding: &servicebindingv1alpha3.ServiceBindingSecretReference{
						Name: secretName,
					},
				},
			},
			workload: &appsv1.Deployment{},
			expected: &appsv1.Deployment{
				Spec: appsv1.DeploymentSpec{
					Template: corev1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Annotations: map[string]string{
								"projector.servicebinding.io/secret-26894874-4719-4802-8f43-8ceed127b4c2": "my-secret",
							},
						},
						Spec: corev1.PodSpec{
							Volumes: []corev1.Volume{
								{
									Name: "servicebinding-26894874-4719-4802-8f43-8ceed127b4c2",
									VolumeSource: corev1.VolumeSource{
										Projected: &corev1.ProjectedVolumeSource{
											Sources: []corev1.VolumeProjection{
												{
													Secret: &corev1.SecretProjection{
														LocalObjectReference: corev1.LocalObjectReference{
															Name: "my-secret",
														},
													},
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
		{
			name:    "rotate binding secret",
			mapping: NewStaticMapping(&servicebindingv1alpha3.ClusterWorkloadResourceMappingTemplate{}),
			binding: &servicebindingv1alpha3.ServiceBinding{
				ObjectMeta: metav1.ObjectMeta{
					UID: uid,
				},
				Spec: servicebindingv1alpha3.ServiceBindingSpec{
					Name: bindingName,
				},
				Status: servicebindingv1alpha3.ServiceBindingStatus{
					Binding: &servicebindingv1alpha3.ServiceBindingSecretReference{
						Name: secretName + "-updated",
					},
				},
			},
			workload: &appsv1.Deployment{
				Spec: appsv1.DeploymentSpec{
					Template: corev1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Annotations: map[string]string{
								"projector.servicebinding.io/secret-26894874-4719-4802-8f43-8ceed127b4c2": secretName,
							},
						},
						Spec: corev1.PodSpec{
							Volumes: []corev1.Volume{
								{
									Name: "servicebinding-26894874-4719-4802-8f43-8ceed127b4c2",
									VolumeSource: corev1.VolumeSource{
										Projected: &corev1.ProjectedVolumeSource{
											Sources: []corev1.VolumeProjection{
												{
													Secret: &corev1.SecretProjection{
														LocalObjectReference: corev1.LocalObjectReference{
															Name: secretName,
														},
													},
												},
											},
										},
									},
								},
							},
							Containers: []corev1.Container{
								{
									Env: []corev1.EnvVar{
										{
											Name:  "SERVICE_BINDING_ROOT",
											Value: "/bindings",
										},
									},
									VolumeMounts: []corev1.VolumeMount{
										{
											Name:      "servicebinding-26894874-4719-4802-8f43-8ceed127b4c2",
											ReadOnly:  true,
											MountPath: "/bindings/my-binding",
										},
									},
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
							Annotations: map[string]string{
								"projector.servicebinding.io/secret-26894874-4719-4802-8f43-8ceed127b4c2": secretName + "-updated",
							},
						},
						Spec: corev1.PodSpec{
							Volumes: []corev1.Volume{
								{
									Name: "servicebinding-26894874-4719-4802-8f43-8ceed127b4c2",
									VolumeSource: corev1.VolumeSource{
										Projected: &corev1.ProjectedVolumeSource{
											Sources: []corev1.VolumeProjection{
												{
													Secret: &corev1.SecretProjection{
														LocalObjectReference: corev1.LocalObjectReference{
															Name: secretName + "-updated",
														},
													},
												},
											},
										},
									},
								},
							},
							Containers: []corev1.Container{
								{
									Env: []corev1.EnvVar{
										{
											Name:  "SERVICE_BINDING_ROOT",
											Value: "/bindings",
										},
									},
									VolumeMounts: []corev1.VolumeMount{
										{
											Name:      "servicebinding-26894874-4719-4802-8f43-8ceed127b4c2",
											ReadOnly:  true,
											MountPath: "/bindings/my-binding",
										},
									},
								},
							},
						},
					},
				},
			},
		},
		{
			name:    "project service binding env",
			mapping: NewStaticMapping(&servicebindingv1alpha3.ClusterWorkloadResourceMappingTemplate{}),
			binding: &servicebindingv1alpha3.ServiceBinding{
				ObjectMeta: metav1.ObjectMeta{
					UID: uid,
				},
				Spec: servicebindingv1alpha3.ServiceBindingSpec{
					Name: bindingName,
					Env: []servicebindingv1alpha3.EnvMapping{
						{
							Name: "FOO",
							Key:  "foo",
						},
						{
							Name: "BAR",
							Key:  "bar",
						},
					},
				},
				Status: servicebindingv1alpha3.ServiceBindingStatus{
					Binding: &servicebindingv1alpha3.ServiceBindingSecretReference{
						Name: secretName,
					},
				},
			},
			workload: &appsv1.Deployment{
				Spec: appsv1.DeploymentSpec{
					Template: corev1.PodTemplateSpec{
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{
								{},
							},
						},
					},
				},
			},
			expected: &appsv1.Deployment{
				Spec: appsv1.DeploymentSpec{
					Template: corev1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Annotations: map[string]string{
								"projector.servicebinding.io/secret-26894874-4719-4802-8f43-8ceed127b4c2": secretName,
							},
						},
						Spec: corev1.PodSpec{
							Volumes: []corev1.Volume{
								{
									Name: "servicebinding-26894874-4719-4802-8f43-8ceed127b4c2",
									VolumeSource: corev1.VolumeSource{
										Projected: &corev1.ProjectedVolumeSource{
											Sources: []corev1.VolumeProjection{
												{
													Secret: &corev1.SecretProjection{
														LocalObjectReference: corev1.LocalObjectReference{
															Name: secretName,
														},
													},
												},
											},
										},
									},
								},
							},
							Containers: []corev1.Container{
								{
									Env: []corev1.EnvVar{
										{
											Name:  "SERVICE_BINDING_ROOT",
											Value: "/bindings",
										},
										{
											Name: "BAR",
											ValueFrom: &corev1.EnvVarSource{
												SecretKeyRef: &corev1.SecretKeySelector{
													Key: "bar",
													LocalObjectReference: corev1.LocalObjectReference{
														Name: secretName,
													},
												},
											},
										},
										{
											Name: "FOO",
											ValueFrom: &corev1.EnvVarSource{
												SecretKeyRef: &corev1.SecretKeySelector{
													Key: "foo",
													LocalObjectReference: corev1.LocalObjectReference{
														Name: secretName,
													},
												},
											},
										},
									},
									VolumeMounts: []corev1.VolumeMount{
										{
											Name:      "servicebinding-26894874-4719-4802-8f43-8ceed127b4c2",
											ReadOnly:  true,
											MountPath: "/bindings/my-binding",
										},
									},
								},
							},
						},
					},
				},
			},
		},
		{
			name:    "remove service binding env",
			mapping: NewStaticMapping(&servicebindingv1alpha3.ClusterWorkloadResourceMappingTemplate{}),
			binding: &servicebindingv1alpha3.ServiceBinding{
				ObjectMeta: metav1.ObjectMeta{
					UID: uid,
				},
				Spec: servicebindingv1alpha3.ServiceBindingSpec{
					Name: bindingName,
				},
				Status: servicebindingv1alpha3.ServiceBindingStatus{
					Binding: &servicebindingv1alpha3.ServiceBindingSecretReference{
						Name: secretName,
					},
				},
			},
			workload: &appsv1.Deployment{
				Spec: appsv1.DeploymentSpec{
					Template: corev1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Annotations: map[string]string{
								"projector.servicebinding.io/secret-26894874-4719-4802-8f43-8ceed127b4c2": secretName,
							},
						},
						Spec: corev1.PodSpec{
							Volumes: []corev1.Volume{
								{
									Name: "servicebinding-26894874-4719-4802-8f43-8ceed127b4c2",
									VolumeSource: corev1.VolumeSource{
										Projected: &corev1.ProjectedVolumeSource{
											Sources: []corev1.VolumeProjection{
												{
													Secret: &corev1.SecretProjection{
														LocalObjectReference: corev1.LocalObjectReference{
															Name: secretName,
														},
													},
												},
											},
										},
									},
								},
							},
							Containers: []corev1.Container{
								{
									Env: []corev1.EnvVar{
										{
											Name:  "SERVICE_BINDING_ROOT",
											Value: "/bindings",
										},
										{
											Name: "BAR",
											ValueFrom: &corev1.EnvVarSource{
												SecretKeyRef: &corev1.SecretKeySelector{
													Key: "bar",
													LocalObjectReference: corev1.LocalObjectReference{
														Name: secretName,
													},
												},
											},
										},
										{
											Name: "FOO",
											ValueFrom: &corev1.EnvVarSource{
												SecretKeyRef: &corev1.SecretKeySelector{
													Key: "foo",
													LocalObjectReference: corev1.LocalObjectReference{
														Name: secretName,
													},
												},
											},
										},
									},
									VolumeMounts: []corev1.VolumeMount{
										{
											Name:      "servicebinding-26894874-4719-4802-8f43-8ceed127b4c2",
											ReadOnly:  true,
											MountPath: "/bindings/my-binding",
										},
									},
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
							Annotations: map[string]string{
								"projector.servicebinding.io/secret-26894874-4719-4802-8f43-8ceed127b4c2": secretName,
							},
						},
						Spec: corev1.PodSpec{
							Volumes: []corev1.Volume{
								{
									Name: "servicebinding-26894874-4719-4802-8f43-8ceed127b4c2",
									VolumeSource: corev1.VolumeSource{
										Projected: &corev1.ProjectedVolumeSource{
											Sources: []corev1.VolumeProjection{
												{
													Secret: &corev1.SecretProjection{
														LocalObjectReference: corev1.LocalObjectReference{
															Name: secretName,
														},
													},
												},
											},
										},
									},
								},
							},
							Containers: []corev1.Container{
								{
									Env: []corev1.EnvVar{
										{
											Name:  "SERVICE_BINDING_ROOT",
											Value: "/bindings",
										},
									},
									VolumeMounts: []corev1.VolumeMount{
										{
											Name:      "servicebinding-26894874-4719-4802-8f43-8ceed127b4c2",
											ReadOnly:  true,
											MountPath: "/bindings/my-binding",
										},
									},
								},
							},
						},
					},
				},
			},
		},
		{
			name:    "update service binding env",
			mapping: NewStaticMapping(&servicebindingv1alpha3.ClusterWorkloadResourceMappingTemplate{}),
			binding: &servicebindingv1alpha3.ServiceBinding{
				ObjectMeta: metav1.ObjectMeta{
					UID: uid,
				},
				Spec: servicebindingv1alpha3.ServiceBindingSpec{
					Name: bindingName,
					Env: []servicebindingv1alpha3.EnvMapping{
						{
							Name: "BLEEP",
							Key:  "bleep",
						},
						{
							Name: "BLOOP",
							Key:  "bloop",
						},
					},
				},
				Status: servicebindingv1alpha3.ServiceBindingStatus{
					Binding: &servicebindingv1alpha3.ServiceBindingSecretReference{
						Name: secretName,
					},
				},
			},
			workload: &appsv1.Deployment{
				Spec: appsv1.DeploymentSpec{
					Template: corev1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Annotations: map[string]string{
								"projector.servicebinding.io/secret-26894874-4719-4802-8f43-8ceed127b4c2": secretName,
							},
						},
						Spec: corev1.PodSpec{
							Volumes: []corev1.Volume{
								{
									Name: "servicebinding-26894874-4719-4802-8f43-8ceed127b4c2",
									VolumeSource: corev1.VolumeSource{
										Projected: &corev1.ProjectedVolumeSource{
											Sources: []corev1.VolumeProjection{
												{
													Secret: &corev1.SecretProjection{
														LocalObjectReference: corev1.LocalObjectReference{
															Name: secretName,
														},
													},
												},
											},
										},
									},
								},
							},
							Containers: []corev1.Container{
								{
									Env: []corev1.EnvVar{
										{
											Name:  "SERVICE_BINDING_ROOT",
											Value: "/bindings",
										},
										{
											Name: "BAR",
											ValueFrom: &corev1.EnvVarSource{
												SecretKeyRef: &corev1.SecretKeySelector{
													Key: "bar",
													LocalObjectReference: corev1.LocalObjectReference{
														Name: secretName,
													},
												},
											},
										},
										{
											Name: "FOO",
											ValueFrom: &corev1.EnvVarSource{
												SecretKeyRef: &corev1.SecretKeySelector{
													Key: "foo",
													LocalObjectReference: corev1.LocalObjectReference{
														Name: secretName,
													},
												},
											},
										},
									},
									VolumeMounts: []corev1.VolumeMount{
										{
											Name:      "servicebinding-26894874-4719-4802-8f43-8ceed127b4c2",
											ReadOnly:  true,
											MountPath: "/bindings/my-binding",
										},
									},
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
							Annotations: map[string]string{
								"projector.servicebinding.io/secret-26894874-4719-4802-8f43-8ceed127b4c2": secretName,
							},
						},
						Spec: corev1.PodSpec{
							Volumes: []corev1.Volume{
								{
									Name: "servicebinding-26894874-4719-4802-8f43-8ceed127b4c2",
									VolumeSource: corev1.VolumeSource{
										Projected: &corev1.ProjectedVolumeSource{
											Sources: []corev1.VolumeProjection{
												{
													Secret: &corev1.SecretProjection{
														LocalObjectReference: corev1.LocalObjectReference{
															Name: secretName,
														},
													},
												},
											},
										},
									},
								},
							},
							Containers: []corev1.Container{
								{
									Env: []corev1.EnvVar{
										{
											Name:  "SERVICE_BINDING_ROOT",
											Value: "/bindings",
										},
										{
											Name: "BLEEP",
											ValueFrom: &corev1.EnvVarSource{
												SecretKeyRef: &corev1.SecretKeySelector{
													Key: "bleep",
													LocalObjectReference: corev1.LocalObjectReference{
														Name: secretName,
													},
												},
											},
										},
										{
											Name: "BLOOP",
											ValueFrom: &corev1.EnvVarSource{
												SecretKeyRef: &corev1.SecretKeySelector{
													Key: "bloop",
													LocalObjectReference: corev1.LocalObjectReference{
														Name: secretName,
													},
												},
											},
										},
									},
									VolumeMounts: []corev1.VolumeMount{
										{
											Name:      "servicebinding-26894874-4719-4802-8f43-8ceed127b4c2",
											ReadOnly:  true,
											MountPath: "/bindings/my-binding",
										},
									},
								},
							},
						},
					},
				},
			},
		},
		{
			name:    "project service binding type and provider for env and volume",
			mapping: NewStaticMapping(&servicebindingv1alpha3.ClusterWorkloadResourceMappingTemplate{}),
			binding: &servicebindingv1alpha3.ServiceBinding{
				ObjectMeta: metav1.ObjectMeta{
					UID: uid,
				},
				Spec: servicebindingv1alpha3.ServiceBindingSpec{
					Name:     bindingName,
					Type:     "my-type",
					Provider: "my-provider",
					Env: []servicebindingv1alpha3.EnvMapping{
						{
							Name: "TYPE",
							Key:  "type",
						},
						{
							Name: "PROVIDER",
							Key:  "provider",
						},
					},
				},
				Status: servicebindingv1alpha3.ServiceBindingStatus{
					Binding: &servicebindingv1alpha3.ServiceBindingSecretReference{
						Name: secretName,
					},
				},
			},
			workload: &appsv1.Deployment{
				Spec: appsv1.DeploymentSpec{
					Template: corev1.PodTemplateSpec{
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{
								{},
							},
						},
					},
				},
			},
			expected: &appsv1.Deployment{
				Spec: appsv1.DeploymentSpec{
					Template: corev1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Annotations: map[string]string{
								"projector.servicebinding.io/secret-26894874-4719-4802-8f43-8ceed127b4c2":   secretName,
								"projector.servicebinding.io/type-26894874-4719-4802-8f43-8ceed127b4c2":     "my-type",
								"projector.servicebinding.io/provider-26894874-4719-4802-8f43-8ceed127b4c2": "my-provider",
							},
						},
						Spec: corev1.PodSpec{
							Volumes: []corev1.Volume{
								{
									Name: "servicebinding-26894874-4719-4802-8f43-8ceed127b4c2",
									VolumeSource: corev1.VolumeSource{
										Projected: &corev1.ProjectedVolumeSource{
											Sources: []corev1.VolumeProjection{
												{
													Secret: &corev1.SecretProjection{
														LocalObjectReference: corev1.LocalObjectReference{
															Name: secretName,
														},
													},
												},
												{
													DownwardAPI: &corev1.DownwardAPIProjection{
														Items: []corev1.DownwardAPIVolumeFile{
															{
																Path: "type",
																FieldRef: &corev1.ObjectFieldSelector{
																	FieldPath: "metadata.annotations['projector.servicebinding.io/type-26894874-4719-4802-8f43-8ceed127b4c2']",
																},
															},
														},
													},
												},
												{
													DownwardAPI: &corev1.DownwardAPIProjection{
														Items: []corev1.DownwardAPIVolumeFile{
															{
																Path: "provider",
																FieldRef: &corev1.ObjectFieldSelector{
																	FieldPath: "metadata.annotations['projector.servicebinding.io/provider-26894874-4719-4802-8f43-8ceed127b4c2']",
																},
															},
														},
													},
												},
											},
										},
									},
								},
							},
							Containers: []corev1.Container{
								{
									Env: []corev1.EnvVar{
										{
											Name:  "SERVICE_BINDING_ROOT",
											Value: "/bindings",
										},
										{
											Name: "PROVIDER",
											ValueFrom: &corev1.EnvVarSource{
												FieldRef: &corev1.ObjectFieldSelector{
													FieldPath: "metadata.annotations['projector.servicebinding.io/provider-26894874-4719-4802-8f43-8ceed127b4c2']",
												},
											},
										},
										{
											Name: "TYPE",
											ValueFrom: &corev1.EnvVarSource{
												FieldRef: &corev1.ObjectFieldSelector{
													FieldPath: "metadata.annotations['projector.servicebinding.io/type-26894874-4719-4802-8f43-8ceed127b4c2']",
												},
											},
										},
									},
									VolumeMounts: []corev1.VolumeMount{
										{
											Name:      "servicebinding-26894874-4719-4802-8f43-8ceed127b4c2",
											ReadOnly:  true,
											MountPath: "/bindings/my-binding",
										},
									},
								},
							},
						},
					},
				},
			},
		},
		{
			name:    "update service binding type and provider",
			mapping: NewStaticMapping(&servicebindingv1alpha3.ClusterWorkloadResourceMappingTemplate{}),
			binding: &servicebindingv1alpha3.ServiceBinding{
				ObjectMeta: metav1.ObjectMeta{
					UID: uid,
				},
				Spec: servicebindingv1alpha3.ServiceBindingSpec{
					Name: bindingName,
					Env: []servicebindingv1alpha3.EnvMapping{
						{
							Name: "TYPE",
							Key:  "type",
						},
						{
							Name: "PROVIDER",
							Key:  "provider",
						},
					},
				},
				Status: servicebindingv1alpha3.ServiceBindingStatus{
					Binding: &servicebindingv1alpha3.ServiceBindingSecretReference{
						Name: secretName,
					},
				},
			},
			workload: &appsv1.Deployment{
				Spec: appsv1.DeploymentSpec{
					Template: corev1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Annotations: map[string]string{
								"projector.servicebinding.io/secret-26894874-4719-4802-8f43-8ceed127b4c2":   secretName,
								"projector.servicebinding.io/type-26894874-4719-4802-8f43-8ceed127b4c2":     "my-type",
								"projector.servicebinding.io/provider-26894874-4719-4802-8f43-8ceed127b4c2": "my-provider",
							},
						},
						Spec: corev1.PodSpec{
							Volumes: []corev1.Volume{
								{
									Name: "servicebinding-26894874-4719-4802-8f43-8ceed127b4c2",
									VolumeSource: corev1.VolumeSource{
										Projected: &corev1.ProjectedVolumeSource{
											Sources: []corev1.VolumeProjection{
												{
													Secret: &corev1.SecretProjection{
														LocalObjectReference: corev1.LocalObjectReference{
															Name: secretName,
														},
													},
												},
												{
													DownwardAPI: &corev1.DownwardAPIProjection{
														Items: []corev1.DownwardAPIVolumeFile{
															{
																Path: "type",
																FieldRef: &corev1.ObjectFieldSelector{
																	FieldPath: "metadata.annotations['projector.servicebinding.io/type-26894874-4719-4802-8f43-8ceed127b4c2']",
																},
															},
														},
													},
												},
												{
													DownwardAPI: &corev1.DownwardAPIProjection{
														Items: []corev1.DownwardAPIVolumeFile{
															{
																Path: "provider",
																FieldRef: &corev1.ObjectFieldSelector{
																	FieldPath: "metadata.annotations['projector.servicebinding.io/provider-26894874-4719-4802-8f43-8ceed127b4c2']",
																},
															},
														},
													},
												},
											},
										},
									},
								},
							},
							Containers: []corev1.Container{
								{
									Env: []corev1.EnvVar{
										{
											Name:  "SERVICE_BINDING_ROOT",
											Value: "/bindings",
										},
										{
											Name: "TYPE",
											ValueFrom: &corev1.EnvVarSource{
												FieldRef: &corev1.ObjectFieldSelector{
													FieldPath: "metadata.annotations['projector.servicebinding.io/type-26894874-4719-4802-8f43-8ceed127b4c2']",
												},
											},
										},
										{
											Name: "PROVIDER",
											ValueFrom: &corev1.EnvVarSource{
												FieldRef: &corev1.ObjectFieldSelector{
													FieldPath: "metadata.annotations['projector.servicebinding.io/provider-26894874-4719-4802-8f43-8ceed127b4c2']",
												},
											},
										},
									},
									VolumeMounts: []corev1.VolumeMount{
										{
											Name:      "servicebinding-26894874-4719-4802-8f43-8ceed127b4c2",
											ReadOnly:  true,
											MountPath: "/bindings/my-binding",
										},
									},
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
							Annotations: map[string]string{
								"projector.servicebinding.io/secret-26894874-4719-4802-8f43-8ceed127b4c2": secretName,
							},
						},
						Spec: corev1.PodSpec{
							Volumes: []corev1.Volume{
								{
									Name: "servicebinding-26894874-4719-4802-8f43-8ceed127b4c2",
									VolumeSource: corev1.VolumeSource{
										Projected: &corev1.ProjectedVolumeSource{
											Sources: []corev1.VolumeProjection{
												{
													Secret: &corev1.SecretProjection{
														LocalObjectReference: corev1.LocalObjectReference{
															Name: secretName,
														},
													},
												},
											},
										},
									},
								},
							},
							Containers: []corev1.Container{
								{
									Env: []corev1.EnvVar{
										{
											Name:  "SERVICE_BINDING_ROOT",
											Value: "/bindings",
										},
										{
											Name: "PROVIDER",
											ValueFrom: &corev1.EnvVarSource{
												SecretKeyRef: &corev1.SecretKeySelector{
													LocalObjectReference: corev1.LocalObjectReference{
														Name: secretName,
													},
													Key: "provider",
												},
											},
										},
										{
											Name: "TYPE",
											ValueFrom: &corev1.EnvVarSource{
												SecretKeyRef: &corev1.SecretKeySelector{
													LocalObjectReference: corev1.LocalObjectReference{
														Name: secretName,
													},
													Key: "type",
												},
											},
										},
									},
									VolumeMounts: []corev1.VolumeMount{
										{
											Name:      "servicebinding-26894874-4719-4802-8f43-8ceed127b4c2",
											ReadOnly:  true,
											MountPath: "/bindings/my-binding",
										},
									},
								},
							},
						},
					},
				},
			},
		},
		{
			name:    "no binding if missing secret",
			mapping: NewStaticMapping(&servicebindingv1alpha3.ClusterWorkloadResourceMappingTemplate{}),
			binding: &servicebindingv1alpha3.ServiceBinding{
				ObjectMeta: metav1.ObjectMeta{
					UID: uid,
				},
				Spec: servicebindingv1alpha3.ServiceBindingSpec{
					Name: bindingName,
				},
			},
			workload: &appsv1.Deployment{
				Spec: appsv1.DeploymentSpec{
					Template: corev1.PodTemplateSpec{
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{
								{},
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
							Volumes: []corev1.Volume{},
							Containers: []corev1.Container{
								{
									Env:          []corev1.EnvVar{},
									VolumeMounts: []corev1.VolumeMount{},
								},
							},
						},
					},
				},
			},
		},
		{
			name:    "only bind to allowed containers",
			mapping: NewStaticMapping(&servicebindingv1alpha3.ClusterWorkloadResourceMappingTemplate{}),
			binding: &servicebindingv1alpha3.ServiceBinding{
				ObjectMeta: metav1.ObjectMeta{
					UID: uid,
				},
				Spec: servicebindingv1alpha3.ServiceBindingSpec{
					Name: bindingName,
					Workload: servicebindingv1alpha3.ServiceBindingWorkloadReference{
						Containers: []string{"bind"},
					},
				},
				Status: servicebindingv1alpha3.ServiceBindingStatus{
					Binding: &servicebindingv1alpha3.ServiceBindingSecretReference{
						Name: secretName,
					},
				},
			},
			workload: &appsv1.Deployment{
				Spec: appsv1.DeploymentSpec{
					Template: corev1.PodTemplateSpec{
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{
								{
									Name: "bind",
								},
								{
									Name: "skip",
								},
								{
									Name: "",
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
							Annotations: map[string]string{
								"projector.servicebinding.io/secret-26894874-4719-4802-8f43-8ceed127b4c2": secretName,
							},
						},
						Spec: corev1.PodSpec{
							Volumes: []corev1.Volume{
								{
									Name: "servicebinding-26894874-4719-4802-8f43-8ceed127b4c2",
									VolumeSource: corev1.VolumeSource{
										Projected: &corev1.ProjectedVolumeSource{
											Sources: []corev1.VolumeProjection{
												{
													Secret: &corev1.SecretProjection{
														LocalObjectReference: corev1.LocalObjectReference{
															Name: secretName,
														},
													},
												},
											},
										},
									},
								},
							},
							Containers: []corev1.Container{
								{
									Name: "bind",
									Env: []corev1.EnvVar{
										{
											Name:  "SERVICE_BINDING_ROOT",
											Value: "/bindings",
										},
									},
									VolumeMounts: []corev1.VolumeMount{
										{
											Name:      "servicebinding-26894874-4719-4802-8f43-8ceed127b4c2",
											ReadOnly:  true,
											MountPath: "/bindings/my-binding",
										},
									},
								},
								{
									Name:         "skip",
									Env:          []corev1.EnvVar{},
									VolumeMounts: []corev1.VolumeMount{},
								},
								{
									Name:         "",
									Env:          []corev1.EnvVar{},
									VolumeMounts: []corev1.VolumeMount{},
								},
							},
						},
					},
				},
			},
		},
		{
			name:    "preserve other bindings",
			mapping: NewStaticMapping(&servicebindingv1alpha3.ClusterWorkloadResourceMappingTemplate{}),
			binding: &servicebindingv1alpha3.ServiceBinding{
				ObjectMeta: metav1.ObjectMeta{
					UID: uid,
				},
				Spec: servicebindingv1alpha3.ServiceBindingSpec{
					Name: bindingName,
					Env: []servicebindingv1alpha3.EnvMapping{
						{
							Name: "FOO",
							Key:  "foo",
						},
					},
				},
				Status: servicebindingv1alpha3.ServiceBindingStatus{
					Binding: &servicebindingv1alpha3.ServiceBindingSecretReference{
						Name: secretName,
					},
				},
			},
			workload: &appsv1.Deployment{
				Spec: appsv1.DeploymentSpec{
					Template: corev1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Annotations: map[string]string{
								"projector.servicebinding.io/secret-11111111-1111-1111-1111-111111111111": "secret-1",
								"projector.servicebinding.io/secret-22222222-2222-2222-2222-222222222222": "secret-2",
								"projector.servicebinding.io/secret-33333333-3333-3333-3333-333333333333": "secret-3",
							},
						},
						Spec: corev1.PodSpec{
							Volumes: []corev1.Volume{
								{
									Name: "preexisting",
									VolumeSource: corev1.VolumeSource{
										EmptyDir: &corev1.EmptyDirVolumeSource{},
									},
								},
								{
									Name: "z_existing",
									VolumeSource: corev1.VolumeSource{
										EmptyDir: &corev1.EmptyDirVolumeSource{},
									},
								},
								{
									Name: "servicebinding-33333333-3333-3333-3333-333333333333",
									VolumeSource: corev1.VolumeSource{
										Projected: &corev1.ProjectedVolumeSource{
											Sources: []corev1.VolumeProjection{
												{
													Secret: &corev1.SecretProjection{
														LocalObjectReference: corev1.LocalObjectReference{
															Name: "secret-3",
														},
													},
												},
											},
										},
									},
								},
								{
									Name: "servicebinding-22222222-2222-2222-2222-222222222222",
									VolumeSource: corev1.VolumeSource{
										Projected: &corev1.ProjectedVolumeSource{
											Sources: []corev1.VolumeProjection{
												{
													Secret: &corev1.SecretProjection{
														LocalObjectReference: corev1.LocalObjectReference{
															Name: "secret-2",
														},
													},
												},
											},
										},
									},
								},
								{
									Name: "servicebinding-11111111-1111-1111-1111-111111111111",
									VolumeSource: corev1.VolumeSource{
										Projected: &corev1.ProjectedVolumeSource{
											Sources: []corev1.VolumeProjection{
												{
													Secret: &corev1.SecretProjection{
														LocalObjectReference: corev1.LocalObjectReference{
															Name: "secret-1",
														},
													},
												},
												{
													DownwardAPI: &corev1.DownwardAPIProjection{
														Items: []corev1.DownwardAPIVolumeFile{
															{
																Path: "type",
																FieldRef: &corev1.ObjectFieldSelector{
																	FieldPath: "metadata.annotations['projector.servicebinding.io/type-11111111-1111-1111-1111-111111111111']",
																},
															},
														},
													},
												},
												{
													DownwardAPI: &corev1.DownwardAPIProjection{
														Items: []corev1.DownwardAPIVolumeFile{
															{
																Path: "provider",
																FieldRef: &corev1.ObjectFieldSelector{
																	FieldPath: "metadata.annotations['projector.servicebinding.io/provider-11111111-1111-1111-1111-111111111111']",
																},
															},
														},
													},
												},
											},
										},
									},
								},
							},
							Containers: []corev1.Container{
								{
									Env: []corev1.EnvVar{
										{
											Name:  "PREEXISTING",
											Value: "env",
										},
										{
											Name:  "Z_EXISTING",
											Value: "env",
										},
										{
											Name:  "SERVICE_BINDING_ROOT",
											Value: "/bindings",
										},
										{
											Name: "TYPE",
											ValueFrom: &corev1.EnvVarSource{
												FieldRef: &corev1.ObjectFieldSelector{
													FieldPath: "metadata.annotations['projector.servicebinding.io/type-11111111-1111-1111-1111-111111111111']",
												},
											},
										},
										{
											Name: "PROVIDER",
											ValueFrom: &corev1.EnvVarSource{
												FieldRef: &corev1.ObjectFieldSelector{
													FieldPath: "metadata.annotations['projector.servicebinding.io/provider-11111111-1111-1111-1111-111111111111']",
												},
											},
										},
									},
									VolumeMounts: []corev1.VolumeMount{
										{
											Name:      "preexisting",
											MountPath: "/var/mount",
										},
										{
											Name:      "z_existing",
											MountPath: "/var/mount",
										},
										{
											Name:      "servicebinding-33333333-3333-3333-3333-333333333333",
											ReadOnly:  true,
											MountPath: "/bindings/binding-3",
										},
										{
											Name:      "servicebinding-22222222-2222-2222-2222-222222222222",
											ReadOnly:  true,
											MountPath: "/bindings/binding-2",
										},
										{
											Name:      "servicebinding-11111111-1111-1111-1111-111111111111",
											ReadOnly:  true,
											MountPath: "/bindings/binding-1",
										},
									},
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
							Annotations: map[string]string{
								"projector.servicebinding.io/secret-11111111-1111-1111-1111-111111111111": "secret-1",
								"projector.servicebinding.io/secret-22222222-2222-2222-2222-222222222222": "secret-2",
								"projector.servicebinding.io/secret-26894874-4719-4802-8f43-8ceed127b4c2": secretName,
								"projector.servicebinding.io/secret-33333333-3333-3333-3333-333333333333": "secret-3",
							},
						},
						Spec: corev1.PodSpec{
							Volumes: []corev1.Volume{
								{
									Name: "preexisting",
									VolumeSource: corev1.VolumeSource{
										EmptyDir: &corev1.EmptyDirVolumeSource{},
									},
								},
								{
									Name: "z_existing",
									VolumeSource: corev1.VolumeSource{
										EmptyDir: &corev1.EmptyDirVolumeSource{},
									},
								},
								{
									Name: "servicebinding-11111111-1111-1111-1111-111111111111",
									VolumeSource: corev1.VolumeSource{
										Projected: &corev1.ProjectedVolumeSource{
											Sources: []corev1.VolumeProjection{
												{
													Secret: &corev1.SecretProjection{
														LocalObjectReference: corev1.LocalObjectReference{
															Name: "secret-1",
														},
													},
												},
												{
													DownwardAPI: &corev1.DownwardAPIProjection{
														Items: []corev1.DownwardAPIVolumeFile{
															{
																Path: "type",
																FieldRef: &corev1.ObjectFieldSelector{
																	FieldPath: "metadata.annotations['projector.servicebinding.io/type-11111111-1111-1111-1111-111111111111']",
																},
															},
														},
													},
												},
												{
													DownwardAPI: &corev1.DownwardAPIProjection{
														Items: []corev1.DownwardAPIVolumeFile{
															{
																Path: "provider",
																FieldRef: &corev1.ObjectFieldSelector{
																	FieldPath: "metadata.annotations['projector.servicebinding.io/provider-11111111-1111-1111-1111-111111111111']",
																},
															},
														},
													},
												},
											},
										},
									},
								},
								{
									Name: "servicebinding-22222222-2222-2222-2222-222222222222",
									VolumeSource: corev1.VolumeSource{
										Projected: &corev1.ProjectedVolumeSource{
											Sources: []corev1.VolumeProjection{
												{
													Secret: &corev1.SecretProjection{
														LocalObjectReference: corev1.LocalObjectReference{
															Name: "secret-2",
														},
													},
												},
											},
										},
									},
								},
								{
									Name: "servicebinding-26894874-4719-4802-8f43-8ceed127b4c2",
									VolumeSource: corev1.VolumeSource{
										Projected: &corev1.ProjectedVolumeSource{
											Sources: []corev1.VolumeProjection{
												{
													Secret: &corev1.SecretProjection{
														LocalObjectReference: corev1.LocalObjectReference{
															Name: secretName,
														},
													},
												},
											},
										},
									},
								},
								{
									Name: "servicebinding-33333333-3333-3333-3333-333333333333",
									VolumeSource: corev1.VolumeSource{
										Projected: &corev1.ProjectedVolumeSource{
											Sources: []corev1.VolumeProjection{
												{
													Secret: &corev1.SecretProjection{
														LocalObjectReference: corev1.LocalObjectReference{
															Name: "secret-3",
														},
													},
												},
											},
										},
									},
								},
							},

							Containers: []corev1.Container{
								{
									Env: []corev1.EnvVar{
										{
											Name:  "PREEXISTING",
											Value: "env",
										},
										{
											Name:  "Z_EXISTING",
											Value: "env",
										},
										{
											Name:  "SERVICE_BINDING_ROOT",
											Value: "/bindings",
										},
										{
											Name: "FOO",
											ValueFrom: &corev1.EnvVarSource{
												SecretKeyRef: &corev1.SecretKeySelector{
													Key: "foo",
													LocalObjectReference: corev1.LocalObjectReference{
														Name: "my-secret",
													},
												},
											},
										},
										{
											Name: "PROVIDER",
											ValueFrom: &corev1.EnvVarSource{
												FieldRef: &corev1.ObjectFieldSelector{
													FieldPath: "metadata.annotations['projector.servicebinding.io/provider-11111111-1111-1111-1111-111111111111']",
												},
											},
										},
										{
											Name: "TYPE",
											ValueFrom: &corev1.EnvVarSource{
												FieldRef: &corev1.ObjectFieldSelector{
													FieldPath: "metadata.annotations['projector.servicebinding.io/type-11111111-1111-1111-1111-111111111111']",
												},
											},
										},
									},
									VolumeMounts: []corev1.VolumeMount{
										{
											Name:      "preexisting",
											MountPath: "/var/mount",
										},
										{
											Name:      "z_existing",
											MountPath: "/var/mount",
										},
										{
											Name:      "servicebinding-11111111-1111-1111-1111-111111111111",
											ReadOnly:  true,
											MountPath: "/bindings/binding-1",
										},
										{
											Name:      "servicebinding-22222222-2222-2222-2222-222222222222",
											ReadOnly:  true,
											MountPath: "/bindings/binding-2",
										},
										{
											Name:      "servicebinding-26894874-4719-4802-8f43-8ceed127b4c2",
											ReadOnly:  true,
											MountPath: "/bindings/my-binding",
										},
										{
											Name:      "servicebinding-33333333-3333-3333-3333-333333333333",
											ReadOnly:  true,
											MountPath: "/bindings/binding-3",
										},
									},
								},
							},
						},
					},
				},
			},
		},
		{
			name:    "apply binding should be idempotent",
			mapping: NewStaticMapping(&servicebindingv1alpha3.ClusterWorkloadResourceMappingTemplate{}),
			binding: &servicebindingv1alpha3.ServiceBinding{
				ObjectMeta: metav1.ObjectMeta{
					UID: uid,
				},
				Status: servicebindingv1alpha3.ServiceBindingStatus{
					Binding: &servicebindingv1alpha3.ServiceBindingSecretReference{
						Name: secretName,
					},
				},
			},
			workload: &appsv1.Deployment{
				Spec: appsv1.DeploymentSpec{
					Template: corev1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Annotations: map[string]string{
								"projector.servicebinding.io/secret-11111111-1111-1111-1111-111111111111": "secret-1",
								"projector.servicebinding.io/secret-26894874-4719-4802-8f43-8ceed127b4c2": "my-secret",
							},
						},
						Spec: corev1.PodSpec{
							Volumes: []corev1.Volume{
								{
									Name: "preexisting",
									VolumeSource: corev1.VolumeSource{
										EmptyDir: &corev1.EmptyDirVolumeSource{},
									},
								},
								{
									Name: "servicebinding-11111111-1111-1111-1111-111111111111",
									VolumeSource: corev1.VolumeSource{
										Projected: &corev1.ProjectedVolumeSource{
											Sources: []corev1.VolumeProjection{
												{
													Secret: &corev1.SecretProjection{
														LocalObjectReference: corev1.LocalObjectReference{
															Name: "secret-1",
														},
													},
												},
											},
										},
									},
								},
								{
									Name: "servicebinding-26894874-4719-4802-8f43-8ceed127b4c2",
									VolumeSource: corev1.VolumeSource{
										Projected: &corev1.ProjectedVolumeSource{
											Sources: []corev1.VolumeProjection{
												{
													Secret: &corev1.SecretProjection{
														LocalObjectReference: corev1.LocalObjectReference{
															Name: "my-secret",
														},
													},
												},
											},
										},
									},
								},
								{
									Name: "preexisting2",
									VolumeSource: corev1.VolumeSource{
										EmptyDir: &corev1.EmptyDirVolumeSource{},
									},
								},
							},
							Containers: []corev1.Container{
								{
									Env: []corev1.EnvVar{
										{
											Name:  "PREEXISTING",
											Value: "env",
										},
										{
											Name:  "SERVICE_BINDING_ROOT",
											Value: "/bindings",
										},
									},
									VolumeMounts: []corev1.VolumeMount{
										{
											Name:      "preexisting",
											MountPath: "/var/mount",
										},
										{
											Name:      "servicebinding-11111111-1111-1111-1111-111111111111",
											ReadOnly:  true,
											MountPath: "/bindings/binding-1",
										},
										{
											Name:      "servicebinding-26894874-4719-4802-8f43-8ceed127b4c2",
											ReadOnly:  true,
											MountPath: "/bindings/my-binding",
										},
										{
											Name:      "preexisting2",
											MountPath: "/var/mount",
										},
									},
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
							Annotations: map[string]string{
								"projector.servicebinding.io/secret-11111111-1111-1111-1111-111111111111": "secret-1",
								"projector.servicebinding.io/secret-26894874-4719-4802-8f43-8ceed127b4c2": "my-secret",
							},
						},
						Spec: corev1.PodSpec{
							Volumes: []corev1.Volume{
								{
									Name: "preexisting",
									VolumeSource: corev1.VolumeSource{
										EmptyDir: &corev1.EmptyDirVolumeSource{},
									},
								},
								{
									Name: "preexisting2",
									VolumeSource: corev1.VolumeSource{
										EmptyDir: &corev1.EmptyDirVolumeSource{},
									},
								},
								{
									Name: "servicebinding-11111111-1111-1111-1111-111111111111",
									VolumeSource: corev1.VolumeSource{
										Projected: &corev1.ProjectedVolumeSource{
											Sources: []corev1.VolumeProjection{
												{
													Secret: &corev1.SecretProjection{
														LocalObjectReference: corev1.LocalObjectReference{
															Name: "secret-1",
														},
													},
												},
											},
										},
									},
								},
								{
									Name: "servicebinding-26894874-4719-4802-8f43-8ceed127b4c2",
									VolumeSource: corev1.VolumeSource{
										Projected: &corev1.ProjectedVolumeSource{
											Sources: []corev1.VolumeProjection{
												{
													Secret: &corev1.SecretProjection{
														LocalObjectReference: corev1.LocalObjectReference{
															Name: "my-secret",
														},
													},
												},
											},
										},
									},
								},
							},
							Containers: []corev1.Container{
								{
									Env: []corev1.EnvVar{
										{
											Name:  "PREEXISTING",
											Value: "env",
										},
										{
											Name:  "SERVICE_BINDING_ROOT",
											Value: "/bindings",
										},
									},
									VolumeMounts: []corev1.VolumeMount{
										{
											Name:      "preexisting",
											MountPath: "/var/mount",
										},
										{
											Name:      "preexisting2",
											MountPath: "/var/mount",
										},
										{
											Name:      "servicebinding-11111111-1111-1111-1111-111111111111",
											ReadOnly:  true,
											MountPath: "/bindings/binding-1",
										},
										{
											Name:      "servicebinding-26894874-4719-4802-8f43-8ceed127b4c2",
											ReadOnly:  true,
											MountPath: "/bindings",
										},
									},
								},
							},
						},
					},
				},
			},
		},
		{
			name: "invalid container jsonpath",
			mapping: NewStaticMapping(&servicebindingv1alpha3.ClusterWorkloadResourceMappingTemplate{
				Containers: []servicebindingv1alpha3.ClusterWorkloadResourceMappingContainer{
					{
						Path: "[",
					},
				},
			}),
			binding:     &servicebindingv1alpha3.ServiceBinding{},
			workload:    &appsv1.Deployment{},
			expectedErr: true,
		},
		{
			name:        "conversion error",
			mapping:     NewStaticMapping(&servicebindingv1alpha3.ClusterWorkloadResourceMappingTemplate{}),
			workload:    &BadMarshalJSON{},
			expectedErr: true,
		},
	}

	for _, c := range tests {
		t.Run(c.name, func(t *testing.T) {
			ctx := context.TODO()

			actual := c.workload.DeepCopyObject()
			err := New(c.mapping).Project(ctx, c.binding, actual)

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

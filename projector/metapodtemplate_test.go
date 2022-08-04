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

package projector

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/pointer"

	servicebindingv1beta1 "github.com/servicebinding/runtime/apis/v1beta1"
)

func TestNewMetaPodTemplate(t *testing.T) {
	testAnnotations := map[string]string{
		"key": "value",
	}
	testEnv := corev1.EnvVar{
		Name:  "NAME",
		Value: "value",
	}
	testVolume := corev1.Volume{
		Name: "name",
		VolumeSource: corev1.VolumeSource{
			Secret: &corev1.SecretVolumeSource{
				SecretName: "my-secret",
			},
		},
	}
	testVolumeMount := corev1.VolumeMount{
		Name:      "name",
		MountPath: "/mount/path",
	}

	tests := []struct {
		name        string
		mapping     *servicebindingv1beta1.ClusterWorkloadResourceMappingTemplate
		workload    runtime.Object
		expected    *metaPodTemplate
		expectedErr bool
	}{
		{
			name:    "podspecable",
			mapping: &servicebindingv1beta1.ClusterWorkloadResourceMappingTemplate{},
			workload: &appsv1.Deployment{
				Spec: appsv1.DeploymentSpec{
					Template: corev1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Annotations: testAnnotations,
						},
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
									Name:         "hello",
									Env:          []corev1.EnvVar{testEnv},
									VolumeMounts: []corev1.VolumeMount{testVolumeMount},
								},
								{
									Name: "hello-2",
								},
							},
							Volumes: []corev1.Volume{testVolume},
						},
					},
				},
			},
			expected: &metaPodTemplate{
				Annotations: testAnnotations,
				Containers: []metaContainer{
					{
						Name:         pointer.String("init-hello"),
						Env:          []corev1.EnvVar{},
						VolumeMounts: []corev1.VolumeMount{},
					},
					{
						Name:         pointer.String("init-hello-2"),
						Env:          []corev1.EnvVar{},
						VolumeMounts: []corev1.VolumeMount{},
					},
					{
						Name:         pointer.String("hello"),
						Env:          []corev1.EnvVar{testEnv},
						VolumeMounts: []corev1.VolumeMount{testVolumeMount},
					},
					{
						Name:         pointer.String("hello-2"),
						Env:          []corev1.EnvVar{},
						VolumeMounts: []corev1.VolumeMount{},
					},
				},
				Volumes: []corev1.Volume{testVolume},
			},
		},
		{
			name: "almost podspecable",
			mapping: &servicebindingv1beta1.ClusterWorkloadResourceMappingTemplate{
				Annotations: ".spec.jobTemplate.spec.template.metadata.annotations",
				Containers: []servicebindingv1beta1.ClusterWorkloadResourceMappingContainer{
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
			workload: &batchv1.CronJob{
				Spec: batchv1.CronJobSpec{
					JobTemplate: batchv1.JobTemplateSpec{
						Spec: batchv1.JobSpec{
							Template: corev1.PodTemplateSpec{
								ObjectMeta: metav1.ObjectMeta{
									Annotations: testAnnotations,
								},
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
											Name:         "hello",
											Env:          []corev1.EnvVar{testEnv},
											VolumeMounts: []corev1.VolumeMount{testVolumeMount},
										},
										{
											Name: "hello-2",
										},
									},
									Volumes: []corev1.Volume{testVolume},
								},
							},
						},
					},
				},
			},
			expected: &metaPodTemplate{
				Annotations: testAnnotations,
				Containers: []metaContainer{
					{
						Name:         pointer.String("init-hello"),
						Env:          []corev1.EnvVar{},
						VolumeMounts: []corev1.VolumeMount{},
					},
					{
						Name:         pointer.String("init-hello-2"),
						Env:          []corev1.EnvVar{},
						VolumeMounts: []corev1.VolumeMount{},
					},
					{
						Name:         pointer.String("hello"),
						Env:          []corev1.EnvVar{testEnv},
						VolumeMounts: []corev1.VolumeMount{testVolumeMount},
					},
					{
						Name:         pointer.String("hello-2"),
						Env:          []corev1.EnvVar{},
						VolumeMounts: []corev1.VolumeMount{},
					},
				},
				Volumes: []corev1.Volume{testVolume},
			},
		},
		{
			name:     "no containers",
			mapping:  &servicebindingv1beta1.ClusterWorkloadResourceMappingTemplate{},
			workload: &appsv1.Deployment{},
			expected: &metaPodTemplate{
				Annotations: map[string]string{},
				Containers:  []metaContainer{},
				Volumes:     []corev1.Volume{},
			},
		},
		{
			name:    "empty container",
			mapping: &servicebindingv1beta1.ClusterWorkloadResourceMappingTemplate{},
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
			expected: &metaPodTemplate{
				Annotations: map[string]string{},
				Containers: []metaContainer{
					{
						Name:         pointer.String(""),
						Env:          []corev1.EnvVar{},
						VolumeMounts: []corev1.VolumeMount{},
					},
				},
				Volumes: []corev1.Volume{},
			},
		},
		{
			name: "unmapped container name",
			mapping: &servicebindingv1beta1.ClusterWorkloadResourceMappingTemplate{
				Annotations: ".spec.template.metadata.annotations",
				Containers: []servicebindingv1beta1.ClusterWorkloadResourceMappingContainer{
					{
						Path: ".spec.template.spec.initContainers[*]",
					},
					{
						Path: ".spec.template.spec.containers[*]",
					},
				},
				Volumes: ".spec.template.spec.volumes",
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
			expected: &metaPodTemplate{
				Annotations: map[string]string{},
				Containers: []metaContainer{
					{
						Name:         nil,
						Env:          []corev1.EnvVar{},
						VolumeMounts: []corev1.VolumeMount{},
					},
				},
				Volumes: []corev1.Volume{},
			},
		},
		{
			name: "misaligned path",
			mapping: &servicebindingv1beta1.ClusterWorkloadResourceMappingTemplate{
				Containers: []servicebindingv1beta1.ClusterWorkloadResourceMappingContainer{
					{
						Path: ".foo.bar",
					},
				},
			},
			workload: &appsv1.Deployment{
				Spec: appsv1.DeploymentSpec{
					Template: corev1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Annotations: testAnnotations,
						},
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{
								{
									Name: "hello",
									Env: []corev1.EnvVar{
										testEnv,
									},
									VolumeMounts: []corev1.VolumeMount{
										testVolumeMount,
									},
								},
							},
							Volumes: []corev1.Volume{
								testVolume,
							},
						},
					},
				},
			},
			expected: &metaPodTemplate{
				Annotations: testAnnotations,
				Containers:  []metaContainer{},
				Volumes: []corev1.Volume{
					testVolume,
				},
			},
		},
		{
			name: "misaligned pointers",
			mapping: &servicebindingv1beta1.ClusterWorkloadResourceMappingTemplate{
				Annotations: ".foo/nar",
				Containers: []servicebindingv1beta1.ClusterWorkloadResourceMappingContainer{
					{
						Path:         ".spec.template.spec.containers[*]",
						Name:         ".foo.bar",
						Env:          ".foo.bar",
						VolumeMounts: ".foo.bar",
					},
				},
				Volumes: ".foo.bar",
			},
			workload: &appsv1.Deployment{
				Spec: appsv1.DeploymentSpec{
					Template: corev1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Annotations: testAnnotations,
						},
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{
								{
									Name:         "hello",
									Env:          []corev1.EnvVar{testEnv},
									VolumeMounts: []corev1.VolumeMount{testVolumeMount},
								},
							},
							Volumes: []corev1.Volume{
								testVolume,
							},
						},
					},
				},
			},
			expected: &metaPodTemplate{
				Annotations: map[string]string{},
				Containers: []metaContainer{
					{
						Name:         pointer.String(""),
						Env:          []corev1.EnvVar{},
						VolumeMounts: []corev1.VolumeMount{},
					},
				},
				Volumes: []corev1.Volume{},
			},
		},
		{
			name: "invalid container jsonpath",
			mapping: &servicebindingv1beta1.ClusterWorkloadResourceMappingTemplate{
				Containers: []servicebindingv1beta1.ClusterWorkloadResourceMappingContainer{
					{
						Path: "[",
					},
				},
			},
			workload:    &appsv1.Deployment{},
			expectedErr: true,
		},
		{
			name:        "conversion error",
			mapping:     &servicebindingv1beta1.ClusterWorkloadResourceMappingTemplate{},
			workload:    &BadMarshalJSON{},
			expectedErr: true,
		},
	}

	for _, c := range tests {
		t.Run(c.name, func(t *testing.T) {
			ctx := context.TODO()
			c.mapping.Default()
			actual, err := NewMetaPodTemplate(ctx, c.workload, c.mapping)

			if c.expectedErr && err == nil {
				t.Errorf("NewMetaPodTemplate() expected to err")
				return
			} else if !c.expectedErr && err != nil {
				t.Errorf("NewMetaPodTemplate() unexpected err: %v", err)
				return
			}
			if c.expectedErr {
				return
			}
			if diff := cmp.Diff(c.expected, actual, cmpopts.IgnoreUnexported(metaPodTemplate{})); diff != "" {
				t.Errorf("NewMetaPodTemplate() (-expected, +actual): %s", diff)
			}
		})
	}
}

func TestMetaPodTemplate_WriteToWorkload(t *testing.T) {
	testAnnotations := map[string]string{
		"key": "value",
	}
	testEnv := corev1.EnvVar{
		Name:  "NAME",
		Value: "value",
	}
	testVolume := corev1.Volume{
		Name: "name",
		VolumeSource: corev1.VolumeSource{
			Secret: &corev1.SecretVolumeSource{
				SecretName: "my-secret",
			},
		},
	}
	testVolumeMount := corev1.VolumeMount{
		Name:      "name",
		MountPath: "/mount/path",
	}

	tests := []struct {
		name        string
		mapping     *servicebindingv1beta1.ClusterWorkloadResourceMappingTemplate
		metadata    metaPodTemplate
		workload    runtime.Object
		expected    runtime.Object
		expectedErr bool
	}{
		{
			name:    "podspecable",
			mapping: &servicebindingv1beta1.ClusterWorkloadResourceMappingTemplate{},
			metadata: metaPodTemplate{
				Annotations: testAnnotations,
				Containers: []metaContainer{
					{
						Name:         pointer.String("init-hello"),
						Env:          []corev1.EnvVar{},
						VolumeMounts: []corev1.VolumeMount{},
					},
					{
						Name:         pointer.String("init-hello-2"),
						Env:          []corev1.EnvVar{},
						VolumeMounts: []corev1.VolumeMount{},
					},
					{
						Name:         pointer.String("hello"),
						Env:          []corev1.EnvVar{testEnv},
						VolumeMounts: []corev1.VolumeMount{testVolumeMount},
					},
					{
						Name:         pointer.String("hello-2"),
						Env:          []corev1.EnvVar{},
						VolumeMounts: []corev1.VolumeMount{},
					},
				},
				Volumes: []corev1.Volume{testVolume},
			},
			workload: &appsv1.Deployment{
				Spec: appsv1.DeploymentSpec{
					Template: corev1.PodTemplateSpec{
						Spec: corev1.PodSpec{
							InitContainers: []corev1.Container{
								{},
								{},
							},
							Containers: []corev1.Container{
								{},
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
							Annotations: testAnnotations,
						},
						Spec: corev1.PodSpec{
							InitContainers: []corev1.Container{
								{
									Name:         "init-hello",
									Env:          []corev1.EnvVar{},
									VolumeMounts: []corev1.VolumeMount{},
								},
								{
									Name:         "init-hello-2",
									Env:          []corev1.EnvVar{},
									VolumeMounts: []corev1.VolumeMount{},
								},
							},
							Containers: []corev1.Container{
								{
									Name:         "hello",
									Env:          []corev1.EnvVar{testEnv},
									VolumeMounts: []corev1.VolumeMount{testVolumeMount},
								},
								{
									Name:         "hello-2",
									Env:          []corev1.EnvVar{},
									VolumeMounts: []corev1.VolumeMount{},
								},
							},
							Volumes: []corev1.Volume{testVolume},
						},
					},
				},
			},
		},
		{
			name: "almost podspecable",
			mapping: &servicebindingv1beta1.ClusterWorkloadResourceMappingTemplate{
				Annotations: ".spec.jobTemplate.spec.template.metadata.annotations",
				Containers: []servicebindingv1beta1.ClusterWorkloadResourceMappingContainer{
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
			metadata: metaPodTemplate{
				Annotations: testAnnotations,
				Containers: []metaContainer{
					{
						Name:         pointer.String("init-hello"),
						Env:          []corev1.EnvVar{},
						VolumeMounts: []corev1.VolumeMount{},
					},
					{
						Name:         pointer.String("init-hello-2"),
						Env:          []corev1.EnvVar{},
						VolumeMounts: []corev1.VolumeMount{},
					},
					{
						Name:         pointer.String("hello"),
						Env:          []corev1.EnvVar{testEnv},
						VolumeMounts: []corev1.VolumeMount{testVolumeMount},
					},
					{
						Name:         pointer.String("hello-2"),
						Env:          []corev1.EnvVar{},
						VolumeMounts: []corev1.VolumeMount{},
					},
				},
				Volumes: []corev1.Volume{testVolume},
			},
			workload: &batchv1.CronJob{
				Spec: batchv1.CronJobSpec{
					JobTemplate: batchv1.JobTemplateSpec{
						Spec: batchv1.JobSpec{
							Template: corev1.PodTemplateSpec{
								Spec: corev1.PodSpec{
									InitContainers: []corev1.Container{
										{},
										{},
									},
									Containers: []corev1.Container{
										{},
										{},
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
									Annotations: testAnnotations,
								},
								Spec: corev1.PodSpec{
									InitContainers: []corev1.Container{
										{
											Name:         "init-hello",
											Env:          []corev1.EnvVar{},
											VolumeMounts: []corev1.VolumeMount{},
										},
										{
											Name:         "init-hello-2",
											Env:          []corev1.EnvVar{},
											VolumeMounts: []corev1.VolumeMount{},
										},
									},
									Containers: []corev1.Container{
										{
											Name:         "hello",
											Env:          []corev1.EnvVar{testEnv},
											VolumeMounts: []corev1.VolumeMount{testVolumeMount},
										},
										{
											Name:         "hello-2",
											Env:          []corev1.EnvVar{},
											VolumeMounts: []corev1.VolumeMount{},
										},
									},
									Volumes: []corev1.Volume{testVolume},
								},
							},
						},
					},
				},
			},
		},
		{
			name:    "no containers",
			mapping: &servicebindingv1beta1.ClusterWorkloadResourceMappingTemplate{},
			metadata: metaPodTemplate{
				Annotations: map[string]string{},
				Containers:  []metaContainer{},
				Volumes:     []corev1.Volume{},
			},
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
			name:    "empty container",
			mapping: &servicebindingv1beta1.ClusterWorkloadResourceMappingTemplate{},
			metadata: metaPodTemplate{
				Annotations: map[string]string{},
				Containers: []metaContainer{
					{
						Name:         pointer.String(""),
						Env:          []corev1.EnvVar{},
						VolumeMounts: []corev1.VolumeMount{},
					},
				},
				Volumes: []corev1.Volume{},
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
							Containers: []corev1.Container{
								{
									Env:          []corev1.EnvVar{},
									VolumeMounts: []corev1.VolumeMount{},
								},
							},
							Volumes: []corev1.Volume{},
						},
					},
				},
			},
		},
	}

	for _, c := range tests {
		t.Run(c.name, func(t *testing.T) {
			ctx := context.TODO()

			// set unexported values
			c.mapping.Default()
			c.metadata.mapping = c.mapping
			c.metadata.workload = c.workload
			err := c.metadata.WriteToWorkload(ctx)

			if c.expectedErr && err == nil {
				t.Errorf("WriteToWorkload() expected to err")
				return
			} else if !c.expectedErr && err != nil {
				t.Errorf("WriteToWorkload() unexpected err: %v", err)
				return
			}
			if c.expectedErr {
				return
			}
			if diff := cmp.Diff(c.expected, c.workload); diff != "" {
				t.Errorf("WriteToWorkload() (-expected, +actual): %s", diff)
			}
		})
	}
}

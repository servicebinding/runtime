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

package resolver_test

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	servicebindingv1alpha3 "github.com/servicebinding/service-binding-controller/apis/v1alpha3"
	"github.com/servicebinding/service-binding-controller/resolver"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestClusterResolver_LookupMapping(t *testing.T) {
	scheme := runtime.NewScheme()
	utilruntime.Must(appsv1.AddToScheme(scheme))
	utilruntime.Must(batchv1.AddToScheme(scheme))
	utilruntime.Must(servicebindingv1alpha3.AddToScheme(scheme))
	restMapper := meta.NewDefaultRESTMapper([]schema.GroupVersion{})
	restMapper.Add(schema.GroupVersionKind{Group: "apps", Version: "v1", Kind: "Deployment"}, nil)
	restMapper.Add(schema.GroupVersionKind{Group: "batch", Version: "v1", Kind: "CronJob"}, nil)

	tests := []struct {
		name         string
		givenObjects []client.Object
		workload     client.Object
		expected     *servicebindingv1alpha3.ClusterWorkloadResourceMappingTemplate
		expectedErr  bool
	}{
		{
			name:         "default mapping",
			givenObjects: []client.Object{},
			workload:     &appsv1.Deployment{},
			expected: &servicebindingv1alpha3.ClusterWorkloadResourceMappingTemplate{
				Version:     "*",
				Annotations: ".spec.template.metadata.annotations",
				Containers: []servicebindingv1alpha3.ClusterWorkloadResourceMappingContainer{
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
		{
			name: "custom mapping",
			givenObjects: []client.Object{
				&servicebindingv1alpha3.ClusterWorkloadResourceMapping{
					ObjectMeta: metav1.ObjectMeta{
						Name: "cronjobs.batch",
					},
					Spec: servicebindingv1alpha3.ClusterWorkloadResourceMappingSpec{
						Versions: []servicebindingv1alpha3.ClusterWorkloadResourceMappingTemplate{
							{
								Version:     "v1",
								Annotations: ".spec.jobTemplate.spec.template.metadata.annotations",
								Containers: []servicebindingv1alpha3.ClusterWorkloadResourceMappingContainer{
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
			},
			workload: &batchv1.CronJob{},
			expected: &servicebindingv1alpha3.ClusterWorkloadResourceMappingTemplate{
				Version:     "v1",
				Annotations: ".spec.jobTemplate.spec.template.metadata.annotations",
				Containers: []servicebindingv1alpha3.ClusterWorkloadResourceMappingContainer{
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
		{
			name: "custom mapping with wildcard",
			givenObjects: []client.Object{
				&servicebindingv1alpha3.ClusterWorkloadResourceMapping{
					ObjectMeta: metav1.ObjectMeta{
						Name: "cronjobs.batch",
					},
					Spec: servicebindingv1alpha3.ClusterWorkloadResourceMappingSpec{
						Versions: []servicebindingv1alpha3.ClusterWorkloadResourceMappingTemplate{
							{
								Version:     "*",
								Annotations: ".spec.jobTemplate.spec.template.metadata.annotations",
								Containers: []servicebindingv1alpha3.ClusterWorkloadResourceMappingContainer{
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
			},
			workload: &batchv1.CronJob{},
			expected: &servicebindingv1alpha3.ClusterWorkloadResourceMappingTemplate{
				Version:     "*",
				Annotations: ".spec.jobTemplate.spec.template.metadata.annotations",
				Containers: []servicebindingv1alpha3.ClusterWorkloadResourceMappingContainer{
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
		{
			name: "default mapping is used when resource version is not defined, and no wildcard is defined",
			givenObjects: []client.Object{
				&servicebindingv1alpha3.ClusterWorkloadResourceMapping{
					ObjectMeta: metav1.ObjectMeta{
						Name: "cronjobs.batch",
					},
					Spec: servicebindingv1alpha3.ClusterWorkloadResourceMappingSpec{
						Versions: []servicebindingv1alpha3.ClusterWorkloadResourceMappingTemplate{
							{
								Version:     "v1beta1", // the workload is version v1
								Annotations: ".spec.jobTemplate.spec.template.metadata.annotations",
								Containers: []servicebindingv1alpha3.ClusterWorkloadResourceMappingContainer{
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
			},
			workload: &batchv1.CronJob{},
			expected: &servicebindingv1alpha3.ClusterWorkloadResourceMappingTemplate{
				Version: "*",
				// default PodSpecable mapping, it won't actually work for a CronJob,
				// but absent an explicit mapping, this is what's required.
				Annotations: ".spec.template.metadata.annotations",
				Containers: []servicebindingv1alpha3.ClusterWorkloadResourceMappingContainer{
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
		{
			name: "error if workload type not found in scheme",
			givenObjects: []client.Object{
				&servicebindingv1alpha3.ClusterWorkloadResourceMapping{
					ObjectMeta: metav1.ObjectMeta{
						Name: "myworkloads.workload.local",
					},
					Spec: servicebindingv1alpha3.ClusterWorkloadResourceMappingSpec{
						Versions: []servicebindingv1alpha3.ClusterWorkloadResourceMappingTemplate{
							{
								Version: "*",
							},
						},
					},
				},
			},
			// this is a bogus workload type, but it's sufficient for the test (we need a type object that is not registered with the scheme)
			workload:    &networkingv1.Ingress{},
			expectedErr: true,
		},
		{
			name: "error if workload type not found in restmapper",
			givenObjects: []client.Object{
				&servicebindingv1alpha3.ClusterWorkloadResourceMapping{
					ObjectMeta: metav1.ObjectMeta{
						Name: "myworkloads.workload.local",
					},
					Spec: servicebindingv1alpha3.ClusterWorkloadResourceMappingSpec{
						Versions: []servicebindingv1alpha3.ClusterWorkloadResourceMappingTemplate{
							{
								Version: "*",
							},
						},
					},
				},
			},
			workload: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "workload.local/v1",
					"kind":       "MyWorkload",
				},
			},
			expectedErr: true,
		},
	}

	for _, c := range tests {
		t.Run(c.name, func(t *testing.T) {
			ctx := context.TODO()

			client := fakeclient.NewClientBuilder().
				WithScheme(scheme).
				WithRESTMapper(restMapper).
				WithObjects(c.givenObjects...).
				Build()
			resolver := resolver.NewClusterResolver(client)

			actual, err := resolver.LookupMapping(ctx, c.workload)

			if (err != nil) != c.expectedErr {
				t.Errorf("LookupMapping() expected err: %v", err)
			}
			if c.expectedErr {
				return
			}
			if diff := cmp.Diff(c.expected, actual); diff != "" {
				t.Errorf("LookupMapping() (-expected, +actual): %s", diff)
			}
		})
	}
}

func TestClusterResolver_LookupBindingSecret(t *testing.T) {
	scheme := runtime.NewScheme()
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))

	tests := []struct {
		name         string
		givenObjects []client.Object
		serviceRef   corev1.ObjectReference
		expected     string
		expectedErr  bool
	}{
		{
			name:         "direct binding",
			givenObjects: []client.Object{},
			serviceRef: corev1.ObjectReference{
				APIVersion: "v1",
				Kind:       "Secret",
				Namespace:  "my-namespace",
				Name:       "my-secret",
			},
			expected: "my-secret",
		},
		{
			name: "found provisioned service",
			givenObjects: []client.Object{
				&unstructured.Unstructured{
					Object: map[string]interface{}{
						"apiVersion": "service.local/v1",
						"kind":       "ProvisionedService",
						"metadata": map[string]interface{}{
							"namespace": "my-namespace",
							"name":      "my-service",
						},
						"status": map[string]interface{}{
							"binding": map[string]interface{}{
								"name": "my-secret",
							},
						},
					},
				},
			},
			serviceRef: corev1.ObjectReference{
				APIVersion: "service.local/v1",
				Kind:       "ProvisionedService",
				Namespace:  "my-namespace",
				Name:       "my-service",
			},
			expected: "my-secret",
		},
		{
			name: "found, but not a provisioned service",
			givenObjects: []client.Object{
				&unstructured.Unstructured{
					Object: map[string]interface{}{
						"apiVersion": "service.local/v1",
						"kind":       "NotAProvisionedService",
						"metadata": map[string]interface{}{
							"namespace": "my-namespace",
							"name":      "my-service",
						},
						"status": map[string]interface{}{},
					},
				},
			},
			serviceRef: corev1.ObjectReference{
				APIVersion: "service.local/v1",
				Kind:       "NotAProvisionedService",
				Namespace:  "my-namespace",
				Name:       "my-service",
			},
			expected: "",
		},
		{
			name:         "not found",
			givenObjects: []client.Object{},
			serviceRef: corev1.ObjectReference{
				APIVersion: "service.local/v1",
				Kind:       "ProvisionedService",
				Namespace:  "my-namespace",
				Name:       "my-service",
			},
			expectedErr: true,
		},
	}

	for _, c := range tests {
		t.Run(c.name, func(t *testing.T) {
			ctx := context.TODO()

			client := fakeclient.NewClientBuilder().
				WithScheme(scheme).
				WithObjects(c.givenObjects...).
				Build()
			resolver := resolver.NewClusterResolver(client)

			actual, err := resolver.LookupBindingSecret(ctx, c.serviceRef)

			if (err != nil) != c.expectedErr {
				t.Errorf("LookupBindingSecret() expected err: %v", err)
			}
			if c.expectedErr {
				return
			}
			if diff := cmp.Diff(c.expected, actual); diff != "" {
				t.Errorf("LookupBindingSecret() (-expected, +actual): %s", diff)
			}
		})
	}
}

func TestClusterResolver_LookupWorkload(t *testing.T) {
	scheme := runtime.NewScheme()
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))

	tests := []struct {
		name         string
		givenObjects []client.Object
		serviceRef   corev1.ObjectReference
		expected     client.Object
		expectedErr  bool
	}{
		{
			name:         "not found error",
			givenObjects: []client.Object{},
			serviceRef: corev1.ObjectReference{
				APIVersion: "apps/v1",
				Kind:       "Deployment",
				Namespace:  "my-namespace",
				Name:       "my-workload",
			},
			expectedErr: true,
		},
		{
			name: "found workload from scheme",
			givenObjects: []client.Object{
				&appsv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "my-namespace",
						Name:      "my-workload",
					},
				},
			},
			serviceRef: corev1.ObjectReference{
				APIVersion: "apps/v1",
				Kind:       "Deployment",
				Namespace:  "my-namespace",
				Name:       "my-workload",
			},
			expected: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "apps/v1",
					"kind":       "Deployment",
					"metadata": map[string]interface{}{
						"creationTimestamp": nil,
						"name":              "my-workload",
						"namespace":         "my-namespace",
						"resourceVersion":   "999",
					},
					"spec": map[string]interface{}{
						"selector": nil,
						"strategy": map[string]interface{}{},
						"template": map[string]interface{}{
							"metadata": map[string]interface{}{
								"creationTimestamp": nil,
							},
							"spec": map[string]interface{}{
								"containers": nil,
							},
						},
					},
					"status": map[string]interface{}{},
				},
			},
		},
		{
			name: "found workload not from scheme",
			givenObjects: []client.Object{
				&unstructured.Unstructured{
					Object: map[string]interface{}{
						"apiVersion": "workload.local/v1",
						"kind":       "MyWorkload",
						"metadata": map[string]interface{}{
							"name":      "my-workload",
							"namespace": "my-namespace",
						},
					},
				},
			},
			serviceRef: corev1.ObjectReference{
				APIVersion: "workload.local/v1",
				Kind:       "MyWorkload",
				Namespace:  "my-namespace",
				Name:       "my-workload",
			},
			expected: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "workload.local/v1",
					"kind":       "MyWorkload",
					"metadata": map[string]interface{}{
						"name":            "my-workload",
						"namespace":       "my-namespace",
						"resourceVersion": "999",
					},
				},
			},
		},
	}

	for _, c := range tests {
		t.Run(c.name, func(t *testing.T) {
			ctx := context.TODO()

			client := fakeclient.NewClientBuilder().
				WithScheme(scheme).
				WithObjects(c.givenObjects...).
				Build()
			resolver := resolver.NewClusterResolver(client)

			actual, err := resolver.LookupWorkload(ctx, c.serviceRef)

			if (err != nil) != c.expectedErr {
				t.Errorf("LookupWorkload() expected err: %v", err)
			}
			if c.expectedErr {
				return
			}
			if diff := cmp.Diff(c.expected, actual); diff != "" {
				t.Errorf("LookupWorkload() (-expected, +actual): %s", diff)
			}
		})
	}
}

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

package resolver_test

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	rtesting "github.com/vmware-labs/reconciler-runtime/testing"
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

	servicebindingv1beta1 "github.com/servicebinding/runtime/apis/v1beta1"
	"github.com/servicebinding/runtime/resolver"
)

func TestClusterResolver_LookupRESTMapping(t *testing.T) {
	scheme := runtime.NewScheme()
	utilruntime.Must(appsv1.AddToScheme(scheme))
	utilruntime.Must(batchv1.AddToScheme(scheme))
	utilruntime.Must(servicebindingv1beta1.AddToScheme(scheme))

	deploymentRESTMapping := &meta.RESTMapping{
		GroupVersionKind: schema.GroupVersionKind{Group: "apps", Version: "v1", Kind: "Deployment"},
		Resource:         schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"},
		Scope:            meta.RESTScopeNamespace,
	}
	cronJobRESTMapping := &meta.RESTMapping{
		GroupVersionKind: schema.GroupVersionKind{Group: "batch", Version: "v1", Kind: "CronJob"},
		Resource:         schema.GroupVersionResource{Group: "batch", Version: "v1", Resource: "cronjobs"},
		Scope:            meta.RESTScopeNamespace,
	}

	tests := []struct {
		name         string
		givenObjects []client.Object
		workload     client.Object
		expected     *meta.RESTMapping
		expectedErr  bool
	}{
		{
			name:         "deloyment mapping",
			givenObjects: []client.Object{},
			workload:     &appsv1.Deployment{},
			expected:     deploymentRESTMapping,
		},
		{
			name:         "cronjob mapping",
			givenObjects: []client.Object{},
			workload:     &batchv1.CronJob{},
			expected:     cronJobRESTMapping,
		},
		{
			name: "error if workload type not found in scheme",
			givenObjects: []client.Object{
				&servicebindingv1beta1.ClusterWorkloadResourceMapping{
					ObjectMeta: metav1.ObjectMeta{
						Name: "myworkloads.workload.local",
					},
					Spec: servicebindingv1beta1.ClusterWorkloadResourceMappingSpec{
						Versions: []servicebindingv1beta1.ClusterWorkloadResourceMappingTemplate{
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
				&servicebindingv1beta1.ClusterWorkloadResourceMapping{
					ObjectMeta: metav1.ObjectMeta{
						Name: "myworkloads.workload.local",
					},
					Spec: servicebindingv1beta1.ClusterWorkloadResourceMappingSpec{
						Versions: []servicebindingv1beta1.ClusterWorkloadResourceMappingTemplate{
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

			client := rtesting.NewFakeClient(scheme, c.givenObjects...)
			restMapper := client.RESTMapper().(*meta.DefaultRESTMapper)
			restMapper.Add(deploymentRESTMapping.GroupVersionKind, deploymentRESTMapping.Scope)
			restMapper.Add(cronJobRESTMapping.GroupVersionKind, cronJobRESTMapping.Scope)
			resolver := resolver.New(client)

			actual, err := resolver.LookupRESTMapping(ctx, c.workload)

			if (err != nil) != c.expectedErr {
				t.Errorf("LookupRESTMapping() expected err: %v", err)
			}
			if c.expectedErr {
				return
			}
			scopeComp := cmp.Comparer(func(a, b meta.RESTScope) bool { return a.Name() == b.Name() })
			if diff := cmp.Diff(c.expected, actual, scopeComp); diff != "" {
				t.Errorf("LookupRESTMapping() gvr (-expected, +actual): %s", diff)
			}
		})
	}
}

func TestClusterResolver_LookupWorkloadMapping(t *testing.T) {
	scheme := runtime.NewScheme()
	utilruntime.Must(appsv1.AddToScheme(scheme))
	utilruntime.Must(batchv1.AddToScheme(scheme))
	utilruntime.Must(servicebindingv1beta1.AddToScheme(scheme))

	tests := []struct {
		name                string
		givenObjects        []client.Object
		gvr                 schema.GroupVersionResource
		expected            *servicebindingv1beta1.ClusterWorkloadResourceMappingSpec
		expectedRESTMapping *meta.RESTMapping
		expectedErr         bool
	}{
		{
			name:         "default mapping",
			givenObjects: []client.Object{},
			gvr:          schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"},
			expected: &servicebindingv1beta1.ClusterWorkloadResourceMappingSpec{
				Versions: []servicebindingv1beta1.ClusterWorkloadResourceMappingTemplate{
					{
						Version:     "*",
						Annotations: ".spec.template.metadata.annotations",
						Containers: []servicebindingv1beta1.ClusterWorkloadResourceMappingContainer{
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
		{
			name: "custom mapping",
			givenObjects: []client.Object{
				&servicebindingv1beta1.ClusterWorkloadResourceMapping{
					ObjectMeta: metav1.ObjectMeta{
						Name: "cronjobs.batch",
					},
					Spec: servicebindingv1beta1.ClusterWorkloadResourceMappingSpec{
						Versions: []servicebindingv1beta1.ClusterWorkloadResourceMappingTemplate{
							{
								Version:     "v1",
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
						},
					},
				},
			},
			gvr: schema.GroupVersionResource{Group: "batch", Version: "v1", Resource: "cronjobs"},
			expected: &servicebindingv1beta1.ClusterWorkloadResourceMappingSpec{
				Versions: []servicebindingv1beta1.ClusterWorkloadResourceMappingTemplate{
					{
						Version:     "v1",
						Annotations: ".spec.jobTemplate.spec.template.metadata.annotations",
						Containers: []servicebindingv1beta1.ClusterWorkloadResourceMappingContainer{
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
		{
			name: "custom mapping with wildcard",
			givenObjects: []client.Object{
				&servicebindingv1beta1.ClusterWorkloadResourceMapping{
					ObjectMeta: metav1.ObjectMeta{
						Name: "cronjobs.batch",
					},
					Spec: servicebindingv1beta1.ClusterWorkloadResourceMappingSpec{
						Versions: []servicebindingv1beta1.ClusterWorkloadResourceMappingTemplate{
							{
								Version:     "*",
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
						},
					},
				},
			},
			gvr: schema.GroupVersionResource{Group: "batch", Version: "v1", Resource: "cronjobs"},
			expected: &servicebindingv1beta1.ClusterWorkloadResourceMappingSpec{
				Versions: []servicebindingv1beta1.ClusterWorkloadResourceMappingTemplate{
					{
						Version:     "*",
						Annotations: ".spec.jobTemplate.spec.template.metadata.annotations",
						Containers: []servicebindingv1beta1.ClusterWorkloadResourceMappingContainer{
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
	}

	for _, c := range tests {
		t.Run(c.name, func(t *testing.T) {
			ctx := context.TODO()

			client := rtesting.NewFakeClient(scheme, c.givenObjects...)
			resolver := resolver.New(client)

			actual, err := resolver.LookupWorkloadMapping(ctx, c.gvr)

			if (err != nil) != c.expectedErr {
				t.Errorf("LookupWorkloadMapping() expected err: %v", err)
			}
			if c.expectedErr {
				return
			}
			if diff := cmp.Diff(c.expected, actual); diff != "" {
				t.Errorf("LookupWorkloadMapping() (-expected, +actual): %s", diff)
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

			client := rtesting.NewFakeClient(scheme, c.givenObjects...)
			resolver := resolver.New(client)

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

func TestClusterResolver_LookupWorkloads(t *testing.T) {
	scheme := runtime.NewScheme()
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))

	tests := []struct {
		name         string
		givenObjects []client.Object
		serviceRef   corev1.ObjectReference
		selector     *metav1.LabelSelector
		expected     []runtime.Object
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
			expected: []runtime.Object{
				&unstructured.Unstructured{
					Object: map[string]interface{}{
						"apiVersion": "apps/v1",
						"kind":       "Deployment",
						"metadata": map[string]interface{}{
							"name":      "my-workload",
							"namespace": "my-namespace",
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
			expected: []runtime.Object{
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
		},
		{
			name: "list workloads from scheme",
			givenObjects: []client.Object{
				&appsv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "my-namespace",
						Name:      "my-workload-1",
						Labels: map[string]string{
							"app": "my",
						},
					},
				},
				&appsv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "my-namespace",
						Name:      "my-workload-2",
						Labels: map[string]string{
							"app": "my",
						},
					},
				},
				&appsv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "my-namespace",
						Name:      "not-my-workload",
						Labels: map[string]string{
							"app": "not",
						},
					},
				},
			},
			serviceRef: corev1.ObjectReference{
				APIVersion: "apps/v1",
				Kind:       "Deployment",
				Namespace:  "my-namespace",
			},
			selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": "my",
				},
			},
			expected: []runtime.Object{
				&unstructured.Unstructured{
					Object: map[string]interface{}{
						"apiVersion": "apps/v1",
						"kind":       "Deployment",
						"metadata": map[string]interface{}{
							"name":      "my-workload-1",
							"namespace": "my-namespace",
							"labels": map[string]interface{}{
								"app": "my",
							},
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
				&unstructured.Unstructured{
					Object: map[string]interface{}{
						"apiVersion": "apps/v1",
						"kind":       "Deployment",
						"metadata": map[string]interface{}{
							"name":      "my-workload-2",
							"namespace": "my-namespace",
							"labels": map[string]interface{}{
								"app": "my",
							},
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
		},
		{
			name: "list workloads not from scheme",
			givenObjects: []client.Object{
				&unstructured.Unstructured{
					Object: map[string]interface{}{
						"apiVersion": "workload.local/v1",
						"kind":       "MyWorkload",
						"metadata": map[string]interface{}{
							"name":      "my-workload-1",
							"namespace": "my-namespace",
							"labels": map[string]interface{}{
								"app": "my",
							},
						},
					},
				},
				&unstructured.Unstructured{
					Object: map[string]interface{}{
						"apiVersion": "workload.local/v1",
						"kind":       "MyWorkload",
						"metadata": map[string]interface{}{
							"name":      "my-workload-2",
							"namespace": "my-namespace",
							"labels": map[string]interface{}{
								"app": "my",
							},
						},
					},
				},
				&unstructured.Unstructured{
					Object: map[string]interface{}{
						"apiVersion": "workload.local/v1",
						"kind":       "MyWorkload",
						"metadata": map[string]interface{}{
							"name":      "not-my-workload",
							"namespace": "my-namespace",
							"labels": map[string]interface{}{
								"app": "not",
							},
						},
					},
				},
			},
			serviceRef: corev1.ObjectReference{
				APIVersion: "workload.local/v1",
				Kind:       "MyWorkload",
				Namespace:  "my-namespace",
			},
			selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": "my",
				},
			},
			expected: []runtime.Object{
				&unstructured.Unstructured{
					Object: map[string]interface{}{
						"apiVersion": "workload.local/v1",
						"kind":       "MyWorkload",
						"metadata": map[string]interface{}{
							"name":      "my-workload-1",
							"namespace": "my-namespace",
							"labels": map[string]interface{}{
								"app": "my",
							},
						},
					},
				},
				&unstructured.Unstructured{
					Object: map[string]interface{}{
						"apiVersion": "workload.local/v1",
						"kind":       "MyWorkload",
						"metadata": map[string]interface{}{
							"name":      "my-workload-2",
							"namespace": "my-namespace",
							"labels": map[string]interface{}{
								"app": "my",
							},
						},
					},
				},
			},
		},
	}

	for _, c := range tests {
		t.Run(c.name, func(t *testing.T) {
			ctx := context.TODO()

			client := rtesting.NewFakeClient(scheme, c.givenObjects...)
			resolver := resolver.New(client)

			actual, err := resolver.LookupWorkloads(ctx, c.serviceRef, c.selector)

			if (err != nil) != c.expectedErr {
				t.Errorf("LookupWorkloads() expected err: %v", err)
			}
			if c.expectedErr {
				return
			}
			if diff := cmp.Diff(c.expected, actual, rtesting.IgnoreResourceVersion, rtesting.IgnoreCreationTimestamp); diff != "" {
				t.Errorf("LookupWorkloads() (-expected, +actual): %s", diff)
			}
		})
	}
}

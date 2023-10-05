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

package resolver

import (
	"context"
	"fmt"
	"sort"

	apierrs "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"

	servicebindingv1beta1 "github.com/servicebinding/runtime/apis/v1beta1"
)

// New creates a new resolver backed by a controller-runtime client
func New(client client.Client) Resolver {
	return &clusterResolver{
		client: client,
	}
}

type clusterResolver struct {
	client client.Client
}

func (m *clusterResolver) LookupRESTMapping(ctx context.Context, obj runtime.Object) (*meta.RESTMapping, error) {
	gvk, err := apiutil.GVKForObject(obj, m.client.Scheme())
	if err != nil {
		return nil, err
	}
	rm, err := m.client.RESTMapper().RESTMapping(gvk.GroupKind(), gvk.Version)
	if err != nil {
		return nil, err
	}
	return rm, nil
}

func (m *clusterResolver) LookupWorkloadMapping(ctx context.Context, gvr schema.GroupVersionResource) (*servicebindingv1beta1.ClusterWorkloadResourceMappingSpec, error) {
	wrm := &servicebindingv1beta1.ClusterWorkloadResourceMapping{}

	if err := m.client.Get(ctx, types.NamespacedName{Name: fmt.Sprintf("%s.%s", gvr.Resource, gvr.Group)}, wrm); err != nil {
		if !apierrs.IsNotFound(err) {
			return nil, err
		}
		wrm.Spec = servicebindingv1beta1.ClusterWorkloadResourceMappingSpec{
			Versions: []servicebindingv1beta1.ClusterWorkloadResourceMappingTemplate{
				{
					Version: "*",
				},
			},
		}
	}

	for i := range wrm.Spec.Versions {
		wrm.Spec.Versions[i].Default()
	}

	return &wrm.Spec, nil
}

func (r *clusterResolver) LookupBindingSecret(ctx context.Context, serviceBinding *servicebindingv1beta1.ServiceBinding) (string, error) {
	serviceRef := serviceBinding.Spec.Service
	if serviceRef.APIVersion == "v1" && serviceRef.Kind == "Secret" {
		// direct secret reference
		return serviceRef.Name, nil
	}
	service := &unstructured.Unstructured{}
	service.SetAPIVersion(serviceRef.APIVersion)
	service.SetKind(serviceRef.Kind)
	if err := r.client.Get(ctx, client.ObjectKey{Namespace: serviceBinding.Namespace, Name: serviceRef.Name}, service); err != nil {
		return "", err
	}
	secretName, exists, err := unstructured.NestedString(service.UnstructuredContent(), "status", "binding", "name")
	// treat missing values as empty
	_ = exists
	return secretName, err
}

const (
	mappingAnnotationPrefix = "projector.servicebinding.io/mapping-"
)

func (r *clusterResolver) LookupWorkloads(ctx context.Context, serviceBinding *servicebindingv1beta1.ServiceBinding) ([]runtime.Object, error) {
	workloadRef := serviceBinding.Spec.Workload

	list := &unstructured.UnstructuredList{}
	list.SetAPIVersion(workloadRef.APIVersion)
	list.SetKind(fmt.Sprintf("%sList", workloadRef.Kind))

	var ls labels.Selector
	if workloadRef.Selector != nil {
		var err error
		ls, err = metav1.LabelSelectorAsSelector(workloadRef.Selector)
		if err != nil {
			return nil, err
		}
	}

	if err := r.client.List(ctx, list, client.InNamespace(serviceBinding.Namespace)); err != nil {
		return nil, err
	}
	workloads := []runtime.Object{}
	for i := range list.Items {
		workload := &list.Items[i]
		if annotations := workload.GetAnnotations(); annotations != nil {
			if _, ok := annotations[fmt.Sprintf("%s%s", mappingAnnotationPrefix, serviceBinding.UID)]; ok {
				workloads = append(workloads, workload)
				continue
			}
		}
		if workloadRef.Name != "" {
			if workload.GetName() == workloadRef.Name {
				workloads = append(workloads, workload)
			}
			continue
		}
		if ls.Matches(labels.Set(workload.GetLabels())) {
			workloads = append(workloads, workload)
		}
	}

	sort.Slice(workloads, func(i, j int) bool {
		return workloads[i].(metav1.Object).GetName() < workloads[j].(metav1.Object).GetName()
	})

	return workloads, nil
}

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

package resolver

import (
	"context"
	"fmt"

	servicebindingv1beta1 "github.com/servicebinding/service-binding-controller/apis/v1beta1"
	corev1 "k8s.io/api/core/v1"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
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

func (m *clusterResolver) LookupMapping(ctx context.Context, workload runtime.Object) (*servicebindingv1beta1.ClusterWorkloadResourceMappingTemplate, error) {
	gvk, err := apiutil.GVKForObject(workload, m.client.Scheme())
	if err != nil {
		return nil, err
	}
	rm, err := m.client.RESTMapper().RESTMapping(gvk.GroupKind(), gvk.Version)
	if err != nil {
		return nil, err
	}
	wrm := &servicebindingv1beta1.ClusterWorkloadResourceMapping{}
	err = m.client.Get(ctx, types.NamespacedName{Name: fmt.Sprintf("%s.%s", rm.Resource.Resource, rm.Resource.Group)}, wrm)
	if err != nil {
		if !apierrs.IsNotFound(err) {
			return nil, err
		}
	}

	// find version mapping
	wildcardMapping := servicebindingv1beta1.ClusterWorkloadResourceMappingTemplate{Version: "*"}
	var mapping *servicebindingv1beta1.ClusterWorkloadResourceMappingTemplate
	for _, v := range wrm.Spec.Versions {
		switch v.Version {
		case gvk.Version:
			mapping = &v
		case "*":
			wildcardMapping = v
		}
	}
	if mapping == nil {
		// use wildcard version by default
		mapping = &wildcardMapping
	}

	mapping = mapping.DeepCopy()
	mapping.Default()

	return mapping, nil
}

func (r *clusterResolver) LookupBindingSecret(ctx context.Context, serviceRef corev1.ObjectReference) (string, error) {
	if serviceRef.APIVersion == "v1" && serviceRef.Kind == "Secret" {
		// direct secret reference
		return serviceRef.Name, nil
	}
	service := &unstructured.Unstructured{}
	service.SetAPIVersion(serviceRef.APIVersion)
	service.SetKind(serviceRef.Kind)
	if err := r.client.Get(ctx, client.ObjectKey{Namespace: serviceRef.Namespace, Name: serviceRef.Name}, service); err != nil {
		return "", err
	}
	secretName, exists, err := unstructured.NestedString(service.UnstructuredContent(), "status", "binding", "name")
	// treat missing values as empty
	_ = exists
	return secretName, err
}

func (r *clusterResolver) LookupWorkload(ctx context.Context, workloadRef corev1.ObjectReference) (client.Object, error) {
	workload := &unstructured.Unstructured{}
	workload.SetAPIVersion(workloadRef.APIVersion)
	workload.SetKind(workloadRef.Kind)
	if err := r.client.Get(ctx, client.ObjectKey{Namespace: workloadRef.Namespace, Name: workloadRef.Name}, workload); err != nil {
		return nil, err
	}
	return workload, nil
}

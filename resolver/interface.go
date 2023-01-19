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

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"

	servicebindingv1beta1 "github.com/servicebinding/runtime/apis/v1beta1"
)

type Resolver interface {
	// LookupRESTMapping returns the RESTMapping for the workload type. The rest mapping contains a GroupVersionResource which can
	// be used to fetch the workload mapping.
	LookupRESTMapping(ctx context.Context, obj runtime.Object) (*meta.RESTMapping, error)

	// LookupWorkloadMapping the mapping template for the workload. Typically a ClusterWorkloadResourceMapping is defined for the
	//  workload's fully qualified resource `{resource}.{group}`.
	LookupWorkloadMapping(ctx context.Context, gvr schema.GroupVersionResource) (*servicebindingv1beta1.ClusterWorkloadResourceMappingSpec, error)

	// LookupBindingSecret returns the binding secret name exposed by the service following the Provisioned Service duck-type
	// (`.status.binding.name`). If a direction binding is used (where the referenced service is itself a Secret) the referenced Secret is
	// returned without a lookup.
	LookupBindingSecret(ctx context.Context, serviceRef corev1.ObjectReference) (string, error)

	// LookupWorkloads returns the referenced objects. Often a unstructured Object is used to sidestep issues with schemes and registered
	// types. The selector is mutually exclusive with the reference name.
	LookupWorkloads(ctx context.Context, workloadRef corev1.ObjectReference, selector *metav1.LabelSelector) ([]runtime.Object, error)
}

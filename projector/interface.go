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

	servicebindingv1alpha3 "github.com/servicebinding/service-binding-controller/apis/v1alpha3"
	"k8s.io/apimachinery/pkg/runtime"
)

type ServiceBindingProjector interface {
	// Project the service into the workload as defined by the ServiceBinding.
	Project(ctx context.Context, binding *servicebindingv1alpha3.ServiceBinding, workload runtime.Object) error
	// Unproject the serice from the workload as defined by the ServiceBinding.
	Unproject(ctx context.Context, binding *servicebindingv1alpha3.ServiceBinding, workload runtime.Object) error
}

type MappingSource interface {
	// LookupMapping the mapping template for the workload. Typically a ClusterWorkloadResourceMapping is defined for the workload's
	// fully qualified resource `{resource}.{group}`. The workload's version is either directly matched, or the wildcard version `*`
	// mapping template is returned. If no explicit mapping is found, a mapping appropriate for a PodSpecable resource may be used.
	LookupMapping(ctx context.Context, workload runtime.Object) (*servicebindingv1alpha3.ClusterWorkloadResourceMappingTemplate, error)
}

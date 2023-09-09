/*
Copyright 2023 the original author or authors.

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

package lifecycle

import (
	"context"

	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	servicebindingv1beta1 "github.com/servicebinding/runtime/apis/v1beta1"
	"github.com/servicebinding/runtime/projector"
	"github.com/servicebinding/runtime/resolver"
)

type ServiceBindingHooks struct {
	// ResolverFactory returns a resolver which is used to lookup binding
	// related values.
	//
	// +optional
	ResolverFactory func(client.Client) resolver.Resolver

	// ProjectorFactory returns a projector which is used to bind/unbind the
	// service to/from the workload.
	//
	// +optional
	ProjectorFactory func(projector.MappingSource) projector.ServiceBindingProjector

	// ServiceBindingPreProjection can be used to alter the resolved
	// ServiceBinding before the projection.
	//
	// +optional
	ServiceBindingPreProjection func(ctx context.Context, binding *servicebindingv1beta1.ServiceBinding) error

	// ServiceBindingPostProjection can be used to alter the projected
	// ServiceBinding before mutations are persisted.
	//
	// +optional
	ServiceBindingPostProjection func(ctx context.Context, binding *servicebindingv1beta1.ServiceBinding) error

	// WorkloadPreProjection can be used to alter the resolved workload before
	// the projection.
	//
	// +optional
	WorkloadPreProjection func(ctx context.Context, workload runtime.Object) error

	// WorkloadPostProjection can be used to alter the projected workload
	// before mutations are persisted.
	//
	// +optional
	WorkloadPostProjection func(ctx context.Context, workload runtime.Object) error
}

func (h *ServiceBindingHooks) GetResolver(c client.Client) resolver.Resolver {
	if h.ResolverFactory == nil {
		return resolver.New(c)
	}
	return h.ResolverFactory(c)
}

func (h *ServiceBindingHooks) GetProjector(r projector.MappingSource) projector.ServiceBindingProjector {
	if h.ProjectorFactory == nil {
		return projector.New(r)
	}
	return h.ProjectorFactory(r)
}

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
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ ServiceBindingProjector = (*serviceBindingProjector)(nil)

type serviceBindingProjector struct {
	mappingSource MappingSource
}

// New creates a service binding projector configured for the mapping source. The binding projector is typically created
// once and applied to multiple workloads.
func New(mappingSource MappingSource) ServiceBindingProjector {
	return &serviceBindingProjector{
		mappingSource: mappingSource,
	}
}

func (i *serviceBindingProjector) Project(ctx context.Context, binding *servicebindingv1alpha3.ServiceBinding, workload client.Object) error {
	mapping, err := i.mappingSource.Lookup(ctx, workload)
	if err != nil {
		return err
	}
	mpt, err := NewMetaPodTemplate(ctx, workload, mapping)
	if err != nil {
		return err
	}
	if err := i.project(ctx, binding, mpt); err != nil {
		return err
	}
	return mpt.WriteToWorkload(ctx)
}

func (i *serviceBindingProjector) Unproject(ctx context.Context, binding *servicebindingv1alpha3.ServiceBinding, workload client.Object) error {
	mapping, err := i.mappingSource.Lookup(ctx, workload)
	if err != nil {
		return err
	}
	mpt, err := NewMetaPodTemplate(ctx, workload, mapping)
	if err != nil {
		return err
	}
	if err := i.unproject(ctx, binding, mpt); err != nil {
		return err
	}
	return mpt.WriteToWorkload(ctx)
}

func (i *serviceBindingProjector) project(ctx context.Context, binding *servicebindingv1alpha3.ServiceBinding, mpt *MetaPodTemplate) error {
	// rather than attempt to merge an existing binding, unproject it
	if err := i.unproject(ctx, binding, mpt); err != nil {
		return err
	}

	for i := range mpt.Containers {
		c := &mpt.Containers[i]
		// TODO skip container if not allowed

		serviceBindingRoot := ""
		for _, e := range c.Env {
			if e.Name == "SERVICE_BINDING_ROOT" {
				serviceBindingRoot = e.Value
				break
			}
		}
		if serviceBindingRoot == "" {
			serviceBindingRoot = "/bindings"
			c.Env = append(c.Env, corev1.EnvVar{
				Name:  "SERVICE_BINDING_ROOT",
				Value: serviceBindingRoot,
			})
		}

		// TODO do remaining projection
	}

	return nil
}

func (i *serviceBindingProjector) unproject(ctx context.Context, binding *servicebindingv1alpha3.ServiceBinding, mpt *MetaPodTemplate) error {
	// TODO undo projection

	return nil
}

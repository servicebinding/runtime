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
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ MappingSource = (*staticMapping)(nil)

type staticMapping struct {
	mapping *servicebindingv1alpha3.ClusterWorkloadResourceMappingTemplate
}

// NewStaticMapping returns a single ClusterWorkloadResourceMappingTemplate for each lookup. It is useful for
// testing.
func NewStaticMapping(mapping *servicebindingv1alpha3.ClusterWorkloadResourceMappingTemplate) MappingSource {
	mapping = mapping.DeepCopy()
	mapping.Default()

	return &staticMapping{
		mapping: mapping,
	}
}

func (m *staticMapping) LookupMapping(ctx context.Context, workload client.Object) (*servicebindingv1alpha3.ClusterWorkloadResourceMappingTemplate, error) {
	return m.mapping, nil
}

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

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"

	servicebindingv1 "github.com/servicebinding/runtime/apis/v1"
)

// The workload's version is either directly matched, or the wildcard version `*`
// mapping template is returned. If no explicit mapping is found, a mapping appropriate for a PodSpecable resource may be used.
func MappingVersion(version string, mappings *servicebindingv1.ClusterWorkloadResourceMappingSpec) *servicebindingv1.ClusterWorkloadResourceMappingTemplate {
	wildcardMapping := servicebindingv1.ClusterWorkloadResourceMappingTemplate{Version: "*"}
	var mapping *servicebindingv1.ClusterWorkloadResourceMappingTemplate
	for _, v := range mappings.Versions {
		switch v.Version {
		case version:
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

	return mapping
}

var _ MappingSource = (*staticMapping)(nil)

type staticMapping struct {
	workloadMapping *servicebindingv1.ClusterWorkloadResourceMappingSpec
	restMapping     *meta.RESTMapping
}

// NewStaticMapping returns a single ClusterWorkloadResourceMappingSpec for each lookup. It is useful for
// testing.
func NewStaticMapping(wm *servicebindingv1.ClusterWorkloadResourceMappingSpec, rm *meta.RESTMapping) MappingSource {
	if len(wm.Versions) == 0 {
		wm.Versions = []servicebindingv1.ClusterWorkloadResourceMappingTemplate{
			{
				Version: "*",
			},
		}
	}
	for i := range wm.Versions {
		wm.Versions[i].Default()
	}

	return &staticMapping{
		workloadMapping: wm,
		restMapping:     rm,
	}
}

func (m *staticMapping) LookupRESTMapping(ctx context.Context, obj runtime.Object) (*meta.RESTMapping, error) {
	return m.restMapping, nil
}

func (m *staticMapping) LookupWorkloadMapping(ctx context.Context, gvr schema.GroupVersionResource) (*servicebindingv1.ClusterWorkloadResourceMappingSpec, error) {
	return m.workloadMapping, nil
}

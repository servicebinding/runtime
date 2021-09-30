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
	"fmt"

	servicebindingv1alpha3 "github.com/servicebinding/service-binding-controller/apis/v1alpha3"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
)

// DefaultWorkloadMapping applies default values to a workload mapping. The default values are appropriate for a
// PodSpecable resource. A deep copy of the workload mapping is made before applying the defaults.
func DefaultWorkloadMapping(mapping *servicebindingv1alpha3.ClusterWorkloadResourceMappingTemplate) *servicebindingv1alpha3.ClusterWorkloadResourceMappingTemplate {
	mapping = mapping.DeepCopy()

	if mapping.Annotations == "" {
		mapping.Annotations = ".spec.template.metadata.annotations"
	}
	if len(mapping.Containers) == 0 {
		mapping.Containers = []servicebindingv1alpha3.ClusterWorkloadResourceMappingContainer{
			{
				Path: ".spec.template.spec.initContainers[*]",
				Name: ".name",
			},
			{
				Path: ".spec.template.spec.containers[*]",
				Name: ".name",
			},
		}
	}
	for i := range mapping.Containers {
		c := &mapping.Containers[i]
		if c.Env == "" {
			c.Env = ".env"
		}
		if c.VolumeMounts == "" {
			c.VolumeMounts = ".volumeMounts"
		}
	}
	if mapping.Volumes == "" {
		mapping.Volumes = ".spec.template.spec.volumes"
	}

	return mapping
}

var _ MappingSource = (*mappingSource)(nil)

type mappingSource struct {
	client client.Client
}

// NewMappingSource creates a MappingSource confired to lookup a ClusterWorkloadResourceMapping against a cluster. The
// client should typically be backed by an informer for the ClusterWorkloadResourceMapping kind.
func NewMappingSource(client client.Client) MappingSource {
	return &mappingSource{
		client: client,
	}
}

func (m *mappingSource) Lookup(ctx context.Context, workload client.Object) (*servicebindingv1alpha3.ClusterWorkloadResourceMappingTemplate, error) {
	gvk, err := apiutil.GVKForObject(workload, m.client.Scheme())
	if err != nil {
		return nil, err
	}
	rm, err := m.client.RESTMapper().RESTMapping(gvk.GroupKind(), gvk.Version)
	if err != nil {
		return nil, err
	}
	wrm := &servicebindingv1alpha3.ClusterWorkloadResourceMapping{}
	err = m.client.Get(ctx, types.NamespacedName{Name: fmt.Sprintf("%s.%s", rm.Resource.Resource, rm.Resource.Group)}, wrm)
	if err != nil {
		if !apierrs.IsNotFound(err) {
			return nil, err
		}
		// use wildcard version default
		wrm.Spec.Versions = []servicebindingv1alpha3.ClusterWorkloadResourceMappingTemplate{
			{Version: "*"},
		}
	}

	// find version mapping
	var wildcardMapping *servicebindingv1alpha3.ClusterWorkloadResourceMappingTemplate
	var mapping *servicebindingv1alpha3.ClusterWorkloadResourceMappingTemplate
	for _, v := range wrm.Spec.Versions {
		switch v.Version {
		case gvk.Version:
			mapping = &v
		case "*":
			wildcardMapping = &v
		}
	}
	if mapping == nil {
		mapping = wildcardMapping
	}
	if mapping == nil {
		return nil, fmt.Errorf("no matching version found for %q", gvk)
	}

	return DefaultWorkloadMapping(mapping), nil
}

var _ MappingSource = (*staticMapping)(nil)

type staticMapping struct {
	mapping *servicebindingv1alpha3.ClusterWorkloadResourceMappingTemplate
}

// NewStaticMapping returns a single ClusterWorkloadResourceMappingTemplate for each lookup. It is useful for
// testing.
func NewStaticMapping(mapping *servicebindingv1alpha3.ClusterWorkloadResourceMappingTemplate) MappingSource {
	return &staticMapping{
		mapping: DefaultWorkloadMapping(mapping),
	}
}

func (m *staticMapping) Lookup(ctx context.Context, workload client.Object) (*servicebindingv1alpha3.ClusterWorkloadResourceMappingTemplate, error) {
	return m.mapping, nil
}

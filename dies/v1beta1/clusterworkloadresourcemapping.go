/*
Copyright 2022 The Kubernetes Authors.

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

package v1beta1

import (
	servicebindingv1beta1 "github.com/servicebinding/runtime/apis/v1beta1"
)

// +die:object=true
type _ = servicebindingv1beta1.ClusterWorkloadResourceMapping

// +die
type _ = servicebindingv1beta1.ClusterWorkloadResourceMappingSpec

func (d *ClusterWorkloadResourceMappingSpecDie) VersionsDie(version string, fn func(d *ClusterWorkloadResourceMappingTemplateDie)) *ClusterWorkloadResourceMappingSpecDie {
	return d.DieStamp(func(r *servicebindingv1beta1.ClusterWorkloadResourceMappingSpec) {
		for i := range r.Versions {
			if version == r.Versions[i].Version {
				d := ClusterWorkloadResourceMappingTemplateBlank.DieImmutable(false).DieFeed(r.Versions[i])
				fn(d)
				r.Versions[i] = d.DieRelease()
				return
			}
		}

		d := ClusterWorkloadResourceMappingTemplateBlank.DieImmutable(false).DieFeed(servicebindingv1beta1.ClusterWorkloadResourceMappingTemplate{Version: version})
		fn(d)
		r.Versions = append(r.Versions, d.DieRelease())
	})
}

// +die
type _ = servicebindingv1beta1.ClusterWorkloadResourceMappingTemplate

func (d *ClusterWorkloadResourceMappingTemplateDie) ContainersDie(containers ...*ClusterWorkloadResourceMappingContainerDie) *ClusterWorkloadResourceMappingTemplateDie {
	return d.DieStamp(func(r *servicebindingv1beta1.ClusterWorkloadResourceMappingTemplate) {
		r.Containers = make([]servicebindingv1beta1.ClusterWorkloadResourceMappingContainer, len(containers))
		for i := range containers {
			r.Containers[i] = containers[i].DieRelease()
		}
	})
}

// +die
type _ = servicebindingv1beta1.ClusterWorkloadResourceMappingContainer

/*
Copyright 2022 the original author or authors.

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

package v1

import (
	servicebindingv1 "github.com/servicebinding/runtime/apis/v1"
)

// +die:object=true
type _ = servicebindingv1.ClusterWorkloadResourceMapping

// +die
// +die:field:name=Versions,die=ClusterWorkloadResourceMappingTemplateDie,listMapKey=Version
type _ = servicebindingv1.ClusterWorkloadResourceMappingSpec

// deprecated use VersionDie
func (d *ClusterWorkloadResourceMappingSpecDie) VersionsDie(version string, fn func(d *ClusterWorkloadResourceMappingTemplateDie)) *ClusterWorkloadResourceMappingSpecDie {
	return d.VersionDie(version, fn)
}

// +die
// +die:field:name=Containers,die=ClusterWorkloadResourceMappingContainerDie,listType=atomic
type _ = servicebindingv1.ClusterWorkloadResourceMappingTemplate

// +die
type _ = servicebindingv1.ClusterWorkloadResourceMappingContainer

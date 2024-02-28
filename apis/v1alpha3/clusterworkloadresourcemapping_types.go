/*
 * Copyright 2021 the original author or authors.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      https://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package v1alpha3

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	servicebindingv1 "github.com/servicebinding/runtime/apis/v1"
)

// ClusterWorkloadResourceMappingTemplate defines the mapping for a specific version of an workload resource to a
// logical PodTemplateSpec-like structure.
type ClusterWorkloadResourceMappingTemplate = servicebindingv1.ClusterWorkloadResourceMappingTemplate

// ClusterWorkloadResourceMappingContainer defines the mapping for a specific fragment of an workload resource
// to a Container-like structure.
//
// Each mapping defines exactly one path that may match multiple container-like fragments within the workload
// resource. For each object matching the path the name, env and volumeMounts expressions are resolved to find those
// structures.
type ClusterWorkloadResourceMappingContainer = servicebindingv1.ClusterWorkloadResourceMappingContainer

// ClusterWorkloadResourceMappingSpec defines the desired state of ClusterWorkloadResourceMapping
type ClusterWorkloadResourceMappingSpec = servicebindingv1.ClusterWorkloadResourceMappingSpec

// +kubebuilder:deprecatedversion:warning="servicebinding.io/v1alpha3 is deprecated and will be removed in a future release, use v1 instead"
// +kubebuilder:object:root=true
// +kubebuilder:resource:scope=Cluster
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// ClusterWorkloadResourceMapping is the Schema for the clusterworkloadresourcemappings API
type ClusterWorkloadResourceMapping struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec ClusterWorkloadResourceMappingSpec `json:"spec,omitempty"`
}

// +kubebuilder:object:root=true

// ClusterWorkloadResourceMappingList contains a list of ClusterWorkloadResourceMapping
type ClusterWorkloadResourceMappingList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []ClusterWorkloadResourceMapping `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ClusterWorkloadResourceMapping{}, &ClusterWorkloadResourceMappingList{})
}

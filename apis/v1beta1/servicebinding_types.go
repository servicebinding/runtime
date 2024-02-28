/*
 * Copyright 2020 the original author or authors.
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

package v1beta1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	servicebindingv1 "github.com/servicebinding/runtime/apis/v1"
)

// ServiceBindingWorkloadReference defines a subset of corev1.ObjectReference with extensions
type ServiceBindingWorkloadReference = servicebindingv1.ServiceBindingWorkloadReference

// ServiceBindingServiceReference defines a subset of corev1.ObjectReference
type ServiceBindingServiceReference = servicebindingv1.ServiceBindingServiceReference

// ServiceBindingSecretReference defines a mirror of corev1.LocalObjectReference
type ServiceBindingSecretReference = servicebindingv1.ServiceBindingSecretReference

// EnvMapping defines a mapping from the value of a Secret entry to an environment variable
type EnvMapping = servicebindingv1.EnvMapping

// ServiceBindingSpec defines the desired state of ServiceBinding
type ServiceBindingSpec = servicebindingv1.ServiceBindingSpec

// These are valid conditions of ServiceBinding.
const (
	// ServiceBindingReady means the ServiceBinding has projected the ProvisionedService
	// secret and the Workload is ready to start. It does not indicate the condition
	// of either the Service or the Workload resources referenced.
	ServiceBindingConditionReady = servicebindingv1.ServiceBindingConditionReady
	// ServiceBindingConditionServiceAvailable means the ServiceBinding's service
	// reference resolved to a ProvisionedService and found a secret. It does not
	// indicate the condition of the Service.
	ServiceBindingConditionServiceAvailable = servicebindingv1.ServiceBindingConditionServiceAvailable
	// ServiceBindingConditionWorkloadProjected means the ServiceBinding has projected
	// the ProvisionedService secret and the Workload is ready to start. It does not
	// indicate the condition of the Workload resources referenced.
	//
	// Not a standardized condition.
	ServiceBindingConditionWorkloadProjected = servicebindingv1.ServiceBindingConditionWorkloadProjected
)

// ServiceBindingStatus defines the observed state of ServiceBinding
type ServiceBindingStatus = servicebindingv1.ServiceBindingStatus

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Secret",type=string,JSONPath=`.status.binding.name`
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].status`
// +kubebuilder:printcolumn:name="Reason",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].reason`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// ServiceBinding is the Schema for the servicebindings API
type ServiceBinding struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ServiceBindingSpec   `json:"spec,omitempty"`
	Status ServiceBindingStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ServiceBindingList contains a list of ServiceBinding
type ServiceBindingList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []ServiceBinding `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ServiceBinding{}, &ServiceBindingList{})
}

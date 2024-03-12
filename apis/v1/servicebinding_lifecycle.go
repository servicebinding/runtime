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

// Package v1 contains API Schema definitions for the servicebinding.io v1 API group
// +kubebuilder:object:generate=true
// +groupName=servicebinding.io
package v1

import (
	"github.com/vmware-labs/reconciler-runtime/apis"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// These are valid conditions of ServiceBinding.
const (
	// ServiceBindingReady means the ServiceBinding has projected the ProvisionedService
	// secret and the Workload is ready to start. It does not indicate the condition
	// of either the Service or the Workload resources referenced.
	ServiceBindingConditionReady = apis.ConditionReady
	// ServiceBindingConditionServiceAvailable means the ServiceBinding's service
	// reference resolved to a ProvisionedService and found a secret. It does not
	// indicate the condition of the Service.
	ServiceBindingConditionServiceAvailable = "ServiceAvailable"
	// ServiceBindingConditionWorkloadProjected means the ServiceBinding has projected
	// the ProvisionedService secret and the Workload is ready to start. It does not
	// indicate the condition of the Workload resources referenced.
	//
	// Not a standardized condition.
	ServiceBindingConditionWorkloadProjected = "WorkloadProjected"
)

var servicebindingCondSet = apis.NewLivingConditionSetWithHappyReason(
	"ServiceBound",
	ServiceBindingConditionServiceAvailable,
	ServiceBindingConditionWorkloadProjected,
)

func (s *ServiceBinding) GetConditionsAccessor() apis.ConditionsAccessor {
	return &s.Status
}

func (s *ServiceBinding) GetConditionSet() apis.ConditionSet {
	return servicebindingCondSet
}

func (s *ServiceBinding) GetConditionManager() apis.ConditionManager {
	return servicebindingCondSet.Manage(&s.Status)
}

func (s *ServiceBindingStatus) InitializeConditions() {
	conditionManager := servicebindingCondSet.Manage(s)
	conditionManager.InitializeConditions()
	// reset existing managed conditions
	conditionManager.MarkUnknown(ServiceBindingConditionServiceAvailable, "Initializing", "")
	conditionManager.MarkUnknown(ServiceBindingConditionWorkloadProjected, "Initializing", "")
}

var _ apis.ConditionsAccessor = (*ServiceBindingStatus)(nil)

// GetConditions implements ConditionsAccessor
func (s *ServiceBindingStatus) GetConditions() []metav1.Condition {
	return s.Conditions
}

// SetConditions implements ConditionsAccessor
func (s *ServiceBindingStatus) SetConditions(c []metav1.Condition) {
	s.Conditions = c
}

// GetCondition fetches the condition of the specified type.
func (s *ServiceBindingStatus) GetCondition(t string) *metav1.Condition {
	for _, cond := range s.Conditions {
		if cond.Type == t {
			return &cond
		}
	}
	return nil
}

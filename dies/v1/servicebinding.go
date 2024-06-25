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
	diemetav1 "reconciler.io/dies/apis/meta/v1"

	servicebindingv1 "github.com/servicebinding/runtime/apis/v1"
)

// +die:object=true
type _ = servicebindingv1.ServiceBinding

// +die
// +die:field:name=Workload,die=ServiceBindingWorkloadReferenceDie
// +die:field:name=Service,die=ServiceBindingServiceReferenceDie
// +die:field:name=Env,die=EnvMappingDie,listType=map
type _ = servicebindingv1.ServiceBindingSpec

// +die
// +die:field:name=Selector,package=_/meta/v1,die=LabelSelectorDie,pointer=true
type _ = servicebindingv1.ServiceBindingWorkloadReference

// +die
type _ = servicebindingv1.ServiceBindingServiceReference

// +die
type _ = servicebindingv1.EnvMapping

// +die
// +die:field:name=Conditions,package=_/meta/v1,die=ConditionDie,listType=atomic
// +die:field:name=Binding,die=ServiceBindingSecretReferenceDie,pointer=true
type _ = servicebindingv1.ServiceBindingStatus

var ServiceBindingConditionReady = diemetav1.ConditionBlank.Type(servicebindingv1.ServiceBindingConditionReady).Unknown().Reason("Initializing")
var ServiceBindingConditionServiceAvailable = diemetav1.ConditionBlank.Type(servicebindingv1.ServiceBindingConditionServiceAvailable).Unknown().Reason("Initializing")
var ServiceBindingConditionWorkloadProjected = diemetav1.ConditionBlank.Type(servicebindingv1.ServiceBindingConditionWorkloadProjected).Unknown().Reason("Initializing")

// +die
type _ = servicebindingv1.ServiceBindingSecretReference

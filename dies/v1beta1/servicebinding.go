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

package v1beta1

import (
	dieservicebindingv1 "github.com/servicebinding/runtime/dies/v1"
)

var ServiceBindingBlank = dieservicebindingv1.ServiceBindingBlank

type ServiceBindingDie = dieservicebindingv1.ServiceBindingDie

var ServiceBindingSpecBlank = dieservicebindingv1.ServiceBindingSpecBlank

type ServiceBindingSpecDie = dieservicebindingv1.ServiceBindingSpecDie

var ServiceBindingWorkloadReferenceBlank = dieservicebindingv1.ServiceBindingWorkloadReferenceBlank

type ServiceBindingWorkloadReferenceDie = dieservicebindingv1.ServiceBindingWorkloadReferenceDie

var ServiceBindingServiceReferenceBlank = dieservicebindingv1.ServiceBindingServiceReferenceBlank

type ServiceBindingServiceReferenceDie = dieservicebindingv1.ServiceBindingServiceReferenceDie

var EnvMappingBlank = dieservicebindingv1.EnvMappingBlank

type EnvMappingDie = dieservicebindingv1.EnvMappingDie

var ServiceBindingStatusBlank = dieservicebindingv1.ServiceBindingStatusBlank

type ServiceBindingStatusDie = dieservicebindingv1.ServiceBindingStatusDie

var ServiceBindingConditionReady = dieservicebindingv1.ServiceBindingConditionReady
var ServiceBindingConditionServiceAvailable = dieservicebindingv1.ServiceBindingConditionServiceAvailable
var ServiceBindingConditionWorkloadProjected = dieservicebindingv1.ServiceBindingConditionWorkloadProjected

var ServiceBindingSecretReferenceBlank = dieservicebindingv1.ServiceBindingSecretReferenceBlank

type ServiceBindingSecretReferenceDie = dieservicebindingv1.ServiceBindingSecretReferenceDie

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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	diemetav1 "reconciler.io/dies/apis/meta/v1"

	servicebindingv1 "github.com/servicebinding/runtime/apis/v1"
)

// +die:object=true
type _ = servicebindingv1.ServiceBinding

// +die
type _ = servicebindingv1.ServiceBindingSpec

func (d *ServiceBindingSpecDie) WorkloadDie(fn func(d *ServiceBindingWorkloadReferenceDie)) *ServiceBindingSpecDie {
	return d.DieStamp(func(r *servicebindingv1.ServiceBindingSpec) {
		d := ServiceBindingWorkloadReferenceBlank.DieImmutable(false).DieFeed(r.Workload)
		fn(d)
		r.Workload = d.DieRelease()
	})
}

func (d *ServiceBindingSpecDie) ServiceDie(fn func(d *ServiceBindingServiceReferenceDie)) *ServiceBindingSpecDie {
	return d.DieStamp(func(r *servicebindingv1.ServiceBindingSpec) {
		d := ServiceBindingServiceReferenceBlank.DieImmutable(false).DieFeed(r.Service)
		fn(d)
		r.Service = d.DieRelease()
	})
}

func (d *ServiceBindingSpecDie) EnvDie(name string, fn func(d *EnvMappingDie)) *ServiceBindingSpecDie {
	return d.DieStamp(func(r *servicebindingv1.ServiceBindingSpec) {
		for i := range r.Env {
			if name == r.Env[i].Name {
				d := EnvMappingBlank.DieImmutable(false).DieFeed(r.Env[i])
				fn(d)
				r.Env[i] = d.DieRelease()
				return
			}
		}

		d := EnvMappingBlank.DieImmutable(false).DieFeed(servicebindingv1.EnvMapping{Name: name})
		fn(d)
		r.Env = append(r.Env, d.DieRelease())
	})
}

// +die
type _ = servicebindingv1.ServiceBindingWorkloadReference

func (d *ServiceBindingWorkloadReferenceDie) SelectorDie(fn func(d *diemetav1.LabelSelectorDie)) *ServiceBindingWorkloadReferenceDie {
	return d.DieStamp(func(r *servicebindingv1.ServiceBindingWorkloadReference) {
		d := diemetav1.LabelSelectorBlank.DieImmutable(false).DieFeedPtr(r.Selector)
		fn(d)
		r.Selector = d.DieReleasePtr()
	})
}

// +die
type _ = servicebindingv1.ServiceBindingServiceReference

// +die
type _ = servicebindingv1.EnvMapping

// +die
type _ = servicebindingv1.ServiceBindingStatus

func (d *ServiceBindingStatusDie) ConditionsDie(conditions ...*diemetav1.ConditionDie) *ServiceBindingStatusDie {
	return d.DieStamp(func(r *servicebindingv1.ServiceBindingStatus) {
		r.Conditions = make([]metav1.Condition, len(conditions))
		for i := range conditions {
			r.Conditions[i] = conditions[i].DieRelease()
		}
	})
}

var ServiceBindingConditionReady = diemetav1.ConditionBlank.Type(servicebindingv1.ServiceBindingConditionReady).Unknown().Reason("Initializing")
var ServiceBindingConditionServiceAvailable = diemetav1.ConditionBlank.Type(servicebindingv1.ServiceBindingConditionServiceAvailable).Unknown().Reason("Initializing")
var ServiceBindingConditionWorkloadProjected = diemetav1.ConditionBlank.Type(servicebindingv1.ServiceBindingConditionWorkloadProjected).Unknown().Reason("Initializing")

func (d *ServiceBindingStatusDie) BindingDie(fn func(d *ServiceBindingSecretReferenceDie)) *ServiceBindingStatusDie {
	return d.DieStamp(func(r *servicebindingv1.ServiceBindingStatus) {
		d := ServiceBindingSecretReferenceBlank.DieImmutable(false).DieFeedPtr(r.Binding)
		fn(d)
		r.Binding = d.DieReleasePtr()
	})
}

// +die
type _ = servicebindingv1.ServiceBindingSecretReference

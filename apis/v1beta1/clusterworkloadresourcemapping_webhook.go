/*
Copyright 2023 the original author or authors.

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
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	servicebindingv1 "github.com/servicebinding/runtime/apis/v1"
)

func (r *ClusterWorkloadResourceMapping) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

var _ webhook.Defaulter = &ClusterWorkloadResourceMapping{}

// Default implements webhook.Defaulter so a webhook will be registered for the type
func (r *ClusterWorkloadResourceMapping) Default() {
	r1 := &servicebindingv1.ClusterWorkloadResourceMapping{}
	r.ConvertTo(r1)
	r1.Default()
	r.ConvertFrom(r1)
}

//+kubebuilder:webhook:path=/validate-servicebinding-io-v1beta1-clusterworkloadresourcemapping,mutating=false,failurePolicy=fail,sideEffects=None,groups=servicebinding.io,resources=clusterworkloadresourcemappings,verbs=create;update,versions=v1beta1,name=v1beta1.clusterworkloadresourcemappings.servicebinding.io,admissionReviewVersions={v1,v1beta1}

var _ webhook.Validator = &ClusterWorkloadResourceMapping{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *ClusterWorkloadResourceMapping) ValidateCreate() (admission.Warnings, error) {
	r1 := &servicebindingv1.ClusterWorkloadResourceMapping{}
	if err := r.ConvertTo(r1); err != nil {
		return nil, err
	}
	return r1.ValidateCreate()
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *ClusterWorkloadResourceMapping) ValidateUpdate(old runtime.Object) (admission.Warnings, error) {
	r1 := &servicebindingv1.ClusterWorkloadResourceMapping{}
	if err := r.ConvertTo(r1); err != nil {
		return nil, err
	}
	return r1.ValidateUpdate(old)
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *ClusterWorkloadResourceMapping) ValidateDelete() (admission.Warnings, error) {
	r1 := &servicebindingv1.ClusterWorkloadResourceMapping{}
	if err := r.ConvertTo(r1); err != nil {
		return nil, err
	}
	return r1.ValidateDelete()
}

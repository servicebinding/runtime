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

package rbac

import (
	"context"
	"sync"
	"time"

	"github.com/go-logr/logr"
	authorizationv1 "k8s.io/api/authorization/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type AccessChecker interface {
	CanI(ctx context.Context, group string, resource string) bool
	WithVerb(operation string) AccessChecker
}

func NewAccessChecker(client client.Client, ttl time.Duration) AccessChecker {
	return &accessChecker{
		client: client,
		verb:   "*",
		ttl:    ttl,
		cache:  map[authorizationv1.ResourceAttributes]authorizationv1.SelfSubjectAccessReview{},
	}
}

type accessChecker struct {
	client client.Client
	verb   string
	ttl    time.Duration
	cache  map[authorizationv1.ResourceAttributes]authorizationv1.SelfSubjectAccessReview
	m      sync.Mutex
}

func (ac *accessChecker) WithVerb(verb string) AccessChecker {
	return &accessChecker{
		client: ac.client,
		verb:   verb,
		ttl:    ac.ttl,
		cache:  map[authorizationv1.ResourceAttributes]authorizationv1.SelfSubjectAccessReview{},
	}
}

func (ac *accessChecker) CanI(ctx context.Context, group string, resource string) bool {
	key := authorizationv1.ResourceAttributes{
		Namespace: "",
		Verb:      ac.verb,
		Group:     group,
		Resource:  resource,
	}

	ac.m.Lock()
	defer ac.m.Unlock()

	ssar, ok := ac.cache[key]
	if ok {
		// ensure cached value is sufficiently recent
		if ssar.GetCreationTimestamp().Add(ac.ttl).After(time.Now()) {
			return ssar.Status.Allowed
		}
		delete(ac.cache, key)
		ssar = authorizationv1.SelfSubjectAccessReview{}
	}

	ssar.Spec = authorizationv1.SelfSubjectAccessReviewSpec{
		ResourceAttributes: &key,
	}
	if err := ac.client.Create(ctx, &ssar); err != nil {
		logr.FromContextOrDiscard(ctx).Error(err, "unable to check access", "resource", key)
		// treat errors as not allowed
		ssar.SetCreationTimestamp(metav1.Now())
	}
	ac.cache[key] = ssar
	return ssar.Status.Allowed
}

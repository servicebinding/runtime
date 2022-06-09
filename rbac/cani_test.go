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
	"fmt"
	"testing"
	"time"

	"github.com/vmware-labs/reconciler-runtime/reconcilers"
	rtesting "github.com/vmware-labs/reconciler-runtime/testing"
	appsv1 "k8s.io/api/apps/v1"
	authorizationv1 "k8s.io/api/authorization/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestAccessChecker(t *testing.T) {
	resource := &appsv1.Deployment{}
	var ac *accessChecker

	scheme := runtime.NewScheme()
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))

	rts := rtesting.SubReconcilerTestSuite{{
		Name:     "allow, added to cache",
		Resource: resource,
		WithReactors: []rtesting.ReactionFunc{
			allowSelfSubjectAccessReviewFor("apps", "deployments", "get"),
		},
		ExpectCreates: []client.Object{
			selfSubjectAccessReviewFor("apps", "deployments", "get"),
		},
		CleanUp: func(t *testing.T) error {
			if len(ac.cache) != 1 {
				t.Errorf("unexpected cache")
			}
			if ssar, ok := ac.cache[authorizationv1.ResourceAttributes{
				Group:    "apps",
				Resource: "deployments",
				Verb:     "get",
			}]; !ok {
				t.Errorf("review should be in cache")
			} else if !ssar.Status.Allowed {
				t.Errorf("cached review should be allowed")
			}
			return nil
		},
	}, {
		Name:     "allow, from cache",
		Resource: resource,
		Prepare: func(t *testing.T) error {
			ac.cache[authorizationv1.ResourceAttributes{
				Group:    "apps",
				Resource: "deployments",
				Verb:     "get",
			}] = authorizationv1.SelfSubjectAccessReview{
				ObjectMeta: v1.ObjectMeta{
					CreationTimestamp: metav1.Now(),
				},
				Status: authorizationv1.SubjectAccessReviewStatus{
					Allowed: true,
				},
			}
			return nil
		},
		CleanUp: func(t *testing.T) error {
			if len(ac.cache) != 1 {
				t.Errorf("unexpected cache")
			}
			if ssar, ok := ac.cache[authorizationv1.ResourceAttributes{
				Group:    "apps",
				Resource: "deployments",
				Verb:     "get",
			}]; !ok {
				t.Errorf("review should be in cache")
			} else if !ssar.Status.Allowed {
				t.Errorf("cached review should be allowed")
			}
			return nil
		},
	}, {
		Name:     "deny, added to cache",
		Resource: resource,
		ExpectCreates: []client.Object{
			selfSubjectAccessReviewFor("apps", "deployments", "get"),
		},
		CleanUp: func(t *testing.T) error {
			if len(ac.cache) != 1 {
				t.Errorf("unexpected cache")
			}
			if ssar, ok := ac.cache[authorizationv1.ResourceAttributes{
				Group:    "apps",
				Resource: "deployments",
				Verb:     "get",
			}]; !ok {
				t.Errorf("review should be in cache")
			} else if ssar.Status.Allowed {
				t.Errorf("cached review should be denied")
			}
			return nil
		},
		ShouldErr: true,
	}, {
		Name:     "deny, from cache",
		Resource: resource,
		Prepare: func(t *testing.T) error {
			ac.cache[authorizationv1.ResourceAttributes{
				Group:    "apps",
				Resource: "deployments",
				Verb:     "get",
			}] = authorizationv1.SelfSubjectAccessReview{
				ObjectMeta: v1.ObjectMeta{
					CreationTimestamp: metav1.Now(),
				},
				Status: authorizationv1.SubjectAccessReviewStatus{
					Allowed: false,
				},
			}
			return nil
		},
		CleanUp: func(t *testing.T) error {
			if len(ac.cache) != 1 {
				t.Errorf("unexpected cache")
			}
			if ssar, ok := ac.cache[authorizationv1.ResourceAttributes{
				Group:    "apps",
				Resource: "deployments",
				Verb:     "get",
			}]; !ok {
				t.Errorf("review should be in cache")
			} else if ssar.Status.Allowed {
				t.Errorf("cached review should be denied")
			}
			return nil
		},
		ShouldErr: true,
	}, {
		Name:     "refresh stale cache",
		Resource: resource,
		Prepare: func(t *testing.T) error {
			ac.cache[authorizationv1.ResourceAttributes{
				Group:    "apps",
				Resource: "deployments",
				Verb:     "get",
			}] = authorizationv1.SelfSubjectAccessReview{
				ObjectMeta: v1.ObjectMeta{
					CreationTimestamp: metav1.NewTime(time.Now().Add(-2 * time.Hour)),
				},
				Status: authorizationv1.SubjectAccessReviewStatus{
					Allowed: false,
				},
			}
			return nil
		},
		WithReactors: []rtesting.ReactionFunc{
			allowSelfSubjectAccessReviewFor("apps", "deployments", "get"),
		},
		ExpectCreates: []client.Object{
			selfSubjectAccessReviewFor("apps", "deployments", "get"),
		},
		CleanUp: func(t *testing.T) error {
			if len(ac.cache) != 1 {
				t.Errorf("unexpected cache")
			}
			if ssar, ok := ac.cache[authorizationv1.ResourceAttributes{
				Group:    "apps",
				Resource: "deployments",
				Verb:     "get",
			}]; !ok {
				t.Errorf("review should be in cache")
			} else if !ssar.Status.Allowed {
				t.Errorf("cached review should be allowed")
			}
			return nil
		},
	}}

	rts.Run(t, scheme, func(t *testing.T, rtc *rtesting.SubReconcilerTestCase, c reconcilers.Config) reconcilers.SubReconciler {
		ac = NewAccessChecker(c, time.Hour).WithVerb("get").(*accessChecker)
		return &reconcilers.SyncReconciler{
			Sync: func(ctx context.Context, _ client.Object) error {
				if !ac.CanI(ctx, "apps", "deployments") {
					return fmt.Errorf("access denied")
				}
				return nil
			},
		}
	})
}

func selfSubjectAccessReviewFor(group, resource, verb string) *authorizationv1.SelfSubjectAccessReview {
	return &authorizationv1.SelfSubjectAccessReview{
		Spec: authorizationv1.SelfSubjectAccessReviewSpec{
			ResourceAttributes: &authorizationv1.ResourceAttributes{
				Group:    group,
				Resource: resource,
				Verb:     verb,
			},
		},
	}
}

func allowSelfSubjectAccessReviewFor(group, resource, verb string) rtesting.ReactionFunc {
	return func(action rtesting.Action) (handled bool, ret runtime.Object, err error) {
		r := action.GetResource()
		if r.Group != "authorization.k8s.io" || r.Resource != "SelfSubjectAccessReview" || r.Version != "v1" || action.GetVerb() != "create" {
			// ignore, not creating a SelfSubjectAccessReview
			return false, nil, nil
		}
		ssar := action.(rtesting.CreateAction).GetObject().(*authorizationv1.SelfSubjectAccessReview)
		if ra := ssar.Spec.ResourceAttributes; ra != nil {
			if ra.Group == group && ra.Resource == resource && ra.Verb == verb {
				ssar.Status.Allowed = true
				return true, ssar, nil
			}
		}
		return false, nil, nil
	}
}

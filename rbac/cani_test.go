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
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestAccessChecker(t *testing.T) {
	scheme := runtime.NewScheme()
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))

	rts := rtesting.SubReconcilerTests{
		"allow, added to cache": {
			Resource: &appsv1.Deployment{},
			WithReactors: []rtesting.ReactionFunc{
				allowSelfSubjectAccessReviewFor("apps", "deployments", "get"),
			},
			ExpectCreates: []client.Object{
				selfSubjectAccessReviewFor("apps", "deployments", "get"),
			},
			CleanUp: func(t *testing.T, ctx context.Context, tc *rtesting.SubReconcilerTestCase) error {
				ac := tc.Metadata["accessChecker"].(*accessChecker)
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
		},
		"allow, from cache": {
			Resource: &appsv1.Deployment{},
			Metadata: map[string]interface{}{
				"cache": map[authorizationv1.ResourceAttributes]authorizationv1.SelfSubjectAccessReview{
					{
						Group:    "apps",
						Resource: "deployments",
						Verb:     "get",
					}: {
						ObjectMeta: metav1.ObjectMeta{
							CreationTimestamp: metav1.Now(),
						},
						Status: authorizationv1.SubjectAccessReviewStatus{
							Allowed: true,
						},
					},
				},
			},
			CleanUp: func(t *testing.T, ctx context.Context, tc *rtesting.SubReconcilerTestCase) error {
				ac := tc.Metadata["accessChecker"].(*accessChecker)
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
		},
		"deny, added to cache": {
			Resource: &appsv1.Deployment{},
			ExpectCreates: []client.Object{
				selfSubjectAccessReviewFor("apps", "deployments", "get"),
			},
			CleanUp: func(t *testing.T, ctx context.Context, tc *rtesting.SubReconcilerTestCase) error {
				ac := tc.Metadata["accessChecker"].(*accessChecker)
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
		},
		"deny, from cache": {
			Resource: &appsv1.Deployment{},
			Metadata: map[string]interface{}{
				"cache": map[authorizationv1.ResourceAttributes]authorizationv1.SelfSubjectAccessReview{
					{
						Group:    "apps",
						Resource: "deployments",
						Verb:     "get",
					}: {
						ObjectMeta: metav1.ObjectMeta{
							CreationTimestamp: metav1.Now(),
						},
						Status: authorizationv1.SubjectAccessReviewStatus{
							Allowed: false,
						},
					},
				},
			},
			CleanUp: func(t *testing.T, ctx context.Context, tc *rtesting.SubReconcilerTestCase) error {
				ac := tc.Metadata["accessChecker"].(*accessChecker)
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
		},
		"refresh stale cache": {
			Resource: &appsv1.Deployment{},
			Metadata: map[string]interface{}{
				"cache": map[authorizationv1.ResourceAttributes]authorizationv1.SelfSubjectAccessReview{
					{
						Group:    "apps",
						Resource: "deployments",
						Verb:     "get",
					}: {
						ObjectMeta: metav1.ObjectMeta{
							CreationTimestamp: metav1.NewTime(time.Now().Add(-2 * time.Hour)),
						},
						Status: authorizationv1.SubjectAccessReviewStatus{
							Allowed: false,
						},
					},
				},
			},
			WithReactors: []rtesting.ReactionFunc{
				allowSelfSubjectAccessReviewFor("apps", "deployments", "get"),
			},
			ExpectCreates: []client.Object{
				selfSubjectAccessReviewFor("apps", "deployments", "get"),
			},
			CleanUp: func(t *testing.T, ctx context.Context, tc *rtesting.SubReconcilerTestCase) error {
				ac := tc.Metadata["accessChecker"].(*accessChecker)
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
		},
	}

	rts.Run(t, scheme, func(t *testing.T, tc *rtesting.SubReconcilerTestCase, c reconcilers.Config) reconcilers.SubReconciler {
		ac := NewAccessChecker(c, time.Hour).WithVerb("get").(*accessChecker)
		if cache, ok := tc.Metadata["cache"].(map[authorizationv1.ResourceAttributes]authorizationv1.SelfSubjectAccessReview); ok {
			ac.cache = cache
		}
		tc.Metadata["accessChecker"] = ac

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

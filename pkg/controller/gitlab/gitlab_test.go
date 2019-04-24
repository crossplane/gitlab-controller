/*
Copyright 2019 The GitLab-Controller Authors.

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

package gitlab

import (
	"context"
	"testing"

	xpcorev1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/core/v1alpha1"
	"github.com/go-test/deep"
	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/crossplaneio/gitlab-controller/pkg/apis/controller"
	"github.com/crossplaneio/gitlab-controller/pkg/apis/controller/v1alpha1"
	"github.com/crossplaneio/gitlab-controller/pkg/test"
)

const (
	testNamespace = "default"
	testName      = "test-gitlab"
)

var (
	testKey = types.NamespacedName{
		Namespace: testNamespace,
		Name:      testName,
	}

	testRequest = reconcile.Request{NamespacedName: testKey}
)

func init() {
	_ = controller.AddToScheme(scheme.Scheme)
}

func TestReconciler_Reconcile(t *testing.T) {
	testError := errors.New("test-error")
	type want struct {
		res reconcile.Result
		err error
	}
	tests := []struct {
		name    string
		client  client.Client
		request reconcile.Request
		want    want
	}{
		{
			name:    "ErrorRetrieving NotFound",
			client:  fake.NewFakeClient(),
			request: testRequest,
			want:    want{res: reconcile.Result{}},
		},
		{
			name: "ErrorRetrieving Other",
			client: &test.MockClient{
				MockGet: func(ctx context.Context, key client.ObjectKey, obj runtime.Object) error {
					return testError
				},
			},
			request: testRequest,
			want:    want{res: reconcile.Result{}, err: testError},
		},
		{
			name: "Success",
			client: &test.MockClient{
				MockGet: func(ctx context.Context, key client.ObjectKey, obj runtime.Object) error { return nil },
				MockStatusUpdate: func(ctx context.Context, obj runtime.Object) error {
					g, ok := obj.(*v1alpha1.GitLab)
					if !ok {
						t.Errorf("Reconciler.Reconcile() unexpected type = %T, want %T", obj, &v1alpha1.GitLab{})
					}
					if !g.Status.IsReady() {
						t.Errorf("Reconciler.Reconcile() invalid status = %v, wantErr %v", g.Status.State,
							xpcorev1alpha1.Ready)
					}
					return nil
				},
			},
			request: testRequest,
			want:    want{res: reconcile.Result{RequeueAfter: requeueAfterSuccess}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &Reconciler{
				Client: tt.client,
			}
			got, err := r.Reconcile(tt.request)
			if diff := deep.Equal(err, tt.want.err); diff != nil {
				t.Errorf("Reconciler.Reconcile() error = %v, wantErr %v", err, tt.want.err)
			}
			if diff := deep.Equal(got, tt.want.res); diff != nil {
				t.Errorf("Reconciler.Reconcile() = %v, want %v\n%s", got, tt.want, diff)
			}
		})
	}
}

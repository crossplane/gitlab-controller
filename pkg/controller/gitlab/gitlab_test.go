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

	"github.com/google/go-cmp/cmp"
	"github.com/pkg/errors"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
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

	testMeta = metav1.ObjectMeta{
		Namespace: testNamespace,
		Name:      testName,
	}

	testRequest = reconcile.Request{NamespacedName: testKey}

	cmpErrors = cmp.Comparer(func(x, y error) bool {
		if x == nil || y == nil {
			return x == nil && y == nil
		}
		return x.Error() == y.Error()
	})
)

func init() {
	if err := controller.AddToScheme(scheme.Scheme); err != nil {
		panic(err)
	}
}

func TestReconciler_Reconcile(t *testing.T) {
	testError := errors.New("test-error")
	type fields struct {
		client client.Client
		mill   reconcilerMill
	}
	type want struct {
		res reconcile.Result
		err error
	}
	tests := []struct {
		name    string
		request reconcile.Request
		fields  fields
		want    want
	}{
		{
			name:    "ErrorRetrievingNotFound",
			request: testRequest,
			fields: fields{
				client: &test.MockClient{
					MockGet: func(ctx context.Context, key client.ObjectKey, obj runtime.Object) error {
						return kerrors.NewNotFound(schema.GroupResource{}, "test")
					},
				},
			},
			want: want{res: reconcile.Result{}},
		},
		{
			name:    "ErrorRetrievingOther",
			request: testRequest,
			fields: fields{
				client: &test.MockClient{
					MockGet: func(ctx context.Context, key client.ObjectKey, obj runtime.Object) error {
						return testError
					},
				},
			},
			want: want{res: reconcile.Result{}, err: testError},
		},
		{
			name:    "Success",
			request: testRequest,
			fields: fields{
				client: &test.MockClient{
					MockGet: func(ctx context.Context, key client.ObjectKey, obj runtime.Object) error { return nil },
				},
				mill: &mockGitLabReconcilerMill{
					mockReconciler: func(gl *v1alpha1.GitLab, c client.Client) reconciler {
						return &mockReconciler{
							mockReconcile: func(ctx context.Context) (reconcile.Result, error) {
								return reconcileSuccess, nil
							},
						}
					},
				},
			},
			want: want{res: reconcile.Result{RequeueAfter: requeueAfterOnSuccess}},
		},
	}

	testCaseName := "Reconciler.Reconcile()"
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &Reconciler{
				Client: tt.fields.client,
				mill:   tt.fields.mill,
			}
			got, err := r.Reconcile(tt.request)
			if diff := cmp.Diff(err, tt.want.err, cmpErrors); diff != "" {
				t.Errorf("%s error %s", testCaseName, diff)
			}
			if diff := cmp.Diff(got, tt.want.res); diff != "" {
				t.Errorf("%s %s", testCaseName, diff)
			}
		})
	}
}

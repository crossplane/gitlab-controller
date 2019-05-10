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
	xpstoragev1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/storage/v1alpha1"
	"github.com/google/go-cmp/cmp"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplaneio/gitlab-controller/pkg/apis/controller/v1alpha1"
	"github.com/crossplaneio/gitlab-controller/pkg/test"
	"github.com/crossplaneio/gitlab-controller/pkg/util"
)

func Test_postgresReconciler_reconcile(t *testing.T) {
	ctx := context.TODO()
	testError := errors.New("test-error")
	type fields struct {
		base   *baseResourceReconciler
		finder resourceClassFinder
	}
	type want struct {
		err    error
		status *xpcorev1alpha1.ResourceClaimStatus
	}
	tests := map[string]struct {
		fields fields
		want   want
	}{
		"FailToFindResourceClass": {
			fields: fields{
				base: &baseResourceReconciler{
					GitLab: newGitLabBuilder().build(),
				},
				finder: &mockResourceClassFinder{
					mockFind: func(ctx context.Context, provider corev1.ObjectReference, resource string) (*corev1.ObjectReference, error) {
						return nil, testError
					},
				},
			},
			want: want{err: errors.Wrapf(testError, errorFmtFailedToFindResourceClass, postgresqlClaimKind,
				newGitLabBuilder().build().GetProviderRef())},
		},
		"FailToCreate": {
			fields: fields{
				base: &baseResourceReconciler{
					GitLab: newGitLabBuilder().withMeta(testMeta).build(),
					client: &test.MockClient{
						MockGet: func(ctx context.Context, key client.ObjectKey, obj runtime.Object) error {
							return kerrors.NewNotFound(schema.GroupResource{}, "")
						},
						MockCreate: func(ctx context.Context, obj runtime.Object) error {
							return testError
						},
					},
				},
				finder: &mockResourceClassFinder{
					mockFind: func(ctx context.Context, provider corev1.ObjectReference, resource string) (*corev1.ObjectReference, error) {
						return nil, nil
					},
				},
			},
			want: want{err: errors.Wrapf(testError, errorFmtFailedToCreate, postgresqlClaimKind, testKey)},
		},
		"FailToRetrieveObject-Other": {
			fields: fields{
				base: &baseResourceReconciler{
					GitLab: newGitLabBuilder().withMeta(testMeta).build(),
					client: &test.MockClient{
						MockGet: func(ctx context.Context, key client.ObjectKey, obj runtime.Object) error {
							return testError
						},
					},
				},
				finder: &mockResourceClassFinder{
					mockFind: func(ctx context.Context, provider corev1.ObjectReference, resource string) (*corev1.ObjectReference, error) {
						return nil, nil
					},
				},
			},
			want: want{err: errors.Wrapf(testError, errorFmtFailedToRetrieveInstance, postgresqlClaimKind, testKey)},
		},
		"CreateSuccessful": {
			fields: fields{
				base: &baseResourceReconciler{
					GitLab: newGitLabBuilder().withMeta(testMeta).build(),
					client: &test.MockClient{
						MockGet: func(ctx context.Context, key client.ObjectKey, obj runtime.Object) error {
							return kerrors.NewNotFound(schema.GroupResource{}, "")
						},
						MockCreate: func(ctx context.Context, obj runtime.Object) error { return nil },
					},
				},
				finder: &mockResourceClassFinder{
					mockFind: func(ctx context.Context, provider corev1.ObjectReference, resource string) (*corev1.ObjectReference, error) {
						return nil, nil
					},
				},
			},
			want: want{},
		},
		"Successful": {
			fields: fields{
				base: &baseResourceReconciler{
					GitLab: newGitLabBuilder().withMeta(testMeta).build(),
					client: &test.MockClient{
						MockGet: func(ctx context.Context, key client.ObjectKey, obj runtime.Object) error {
							psql, ok := obj.(*xpstoragev1alpha1.PostgreSQLInstance)
							if !ok {
								return errors.Errorf("postgresReconciler.reconcile() type: %T", obj)
							}
							psql.Status = *newResourceClaimStatusBuilder().withReadyStatus().build()
							return nil
						},
						MockCreate: func(ctx context.Context, obj runtime.Object) error { return nil },
					},
				},
				finder: &mockResourceClassFinder{
					mockFind: func(ctx context.Context, provider corev1.ObjectReference, resource string) (*corev1.ObjectReference, error) {
						return nil, nil
					},
				},
			},
			want: want{
				status: newResourceClaimStatusBuilder().withReadyStatus().build(),
			},
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			r := &postgresReconciler{
				baseResourceReconciler: tt.fields.base,
				resourceClassFinder:    tt.fields.finder,
			}
			if diff := cmp.Diff(r.reconcile(ctx), tt.want.err, cmpErrors); diff != "" {
				t.Errorf("postgresReconciler.reconcile() error %s", diff)
			}
			if diff := cmp.Diff(r.status, tt.want.status, cmp.Comparer(util.EqualConditionedStatus)); diff != "" {
				t.Errorf("postgresReconciler.reconcile() status %s", diff)
			}
		})
	}
}

func Test_postgresReconciler_getClaimKind(t *testing.T) {
	r := &postgresReconciler{}
	if diff := cmp.Diff(r.getClaimKind(), postgresqlClaimKind); diff != "" {
		t.Errorf("postgresReconciler.getClaimKind() %s", diff)
	}
}

func Test_newPostgresReconciler(t *testing.T) {
	gitlab := &v1alpha1.GitLab{}
	clnt := test.NewMockClient()

	r := newPostgresReconciler(gitlab, clnt)
	if diff := cmp.Diff(r.GitLab, gitlab); diff != "" {
		t.Errorf("newPostgresReconciler() GitLab %s", diff)
	}
}

func Test_postgresReconciler_getHelmValues(t *testing.T) {
	ctx := context.TODO()
	type fields struct {
		baseResourceReconciler *baseResourceReconciler
		resourceClassFinder    resourceClassFinder
	}
	tests := map[string]struct {
		fields fields
		args   map[string]string
		want   error
	}{
		"Failure": {
			fields: fields{
				baseResourceReconciler: newBaseResourceReconciler(newGitLabBuilder().build(), test.NewMockClient(), testBucket),
			},
			want: errors.New(errorResourceStatusIsNotFound),
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			r := &postgresReconciler{
				baseResourceReconciler: tt.fields.baseResourceReconciler,
				resourceClassFinder:    tt.fields.resourceClassFinder,
			}
			if diff := cmp.Diff(r.getHelmValues(ctx, tt.args), tt.want, cmpErrors); diff != "" {
				t.Errorf("postgresReconciler.getHelmValues() error %s", diff)
			}
		})
	}
}

func Test_postgresHelmValues(t *testing.T) {
	type args struct {
		values map[string]string
		name   string
		secret *corev1.Secret
	}
	type want struct {
		panic bool
		data  map[string]string
	}
	tests := map[string]struct {
		args args
		want want
	}{
		"Failure": {
			args: args{},
			want: want{panic: true},
		},
		"Success-NoValues": {
			args: args{
				values: make(map[string]string),
				name:   "foo",
				secret: &corev1.Secret{Data: map[string][]byte{}},
			},
			want: want{
				data: map[string]string{
					helmValuePsqlHostKey:     "",
					helmValuePsqlUsernameKey: "",
					helmValuePsqlDatabaseKey: "postgres",
				},
			},
		},
		"Success-WithValue": {
			args: args{
				values: make(map[string]string),
				name:   "foo",
				secret: &corev1.Secret{
					Data: map[string][]byte{
						xpcorev1alpha1.ResourceCredentialsSecretEndpointKey: []byte("bar"),
						xpcorev1alpha1.ResourceCredentialsSecretUserKey:     []byte("foo"),
					},
				},
			},
			want: want{
				data: map[string]string{
					helmValuePsqlHostKey:     "bar",
					helmValuePsqlUsernameKey: "foo",
					helmValuePsqlDatabaseKey: "postgres",
				},
			},
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil && !tt.want.panic {
					t.Errorf("posgresHelmValues() panic: %v", r)
				}
			}()
			postgresHelmValues(tt.args.values, tt.args.name, tt.args.secret)
			if diff := cmp.Diff(tt.args.values, tt.want.data); diff != "" {
				t.Errorf("posgresHelmValues() %s", diff)
			}
		})
	}
}

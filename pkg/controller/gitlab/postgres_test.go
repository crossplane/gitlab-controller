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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/helm/pkg/chartutil"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplaneio/gitlab-controller/pkg/apis/controller/v1alpha1"
	"github.com/crossplaneio/gitlab-controller/pkg/test"
)

var _ resourceReconciler = &postgresReconciler{}

func Test_postgresReconciler_reconcile(t *testing.T) {
	ctx := context.TODO()
	testCaseName := "postgresReconciler.reconcile()"
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
					mockFind: func(ctx context.Context, provider corev1.ObjectReference,
						resource string) (*corev1.ObjectReference, error) {
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
					mockFind: func(ctx context.Context, provider corev1.ObjectReference,
						resource string) (*corev1.ObjectReference, error) {
						return nil, nil
					},
				},
			},
			want: want{err: errors.Wrapf(testError, errorFmtFailedToCreate, postgresqlClaimKind,
				testKey.String()+"-"+xpstoragev1alpha1.PostgreSQLInstanceKind)},
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
					mockFind: func(ctx context.Context, provider corev1.ObjectReference,
						resource string) (*corev1.ObjectReference, error) {
						return nil, nil
					},
				},
			},
			want: want{err: errors.Wrapf(testError, errorFmtFailedToRetrieveInstance, postgresqlClaimKind,
				testKey.String()+"-"+xpstoragev1alpha1.PostgreSQLInstanceKind)},
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
					mockFind: func(ctx context.Context, provider corev1.ObjectReference,
						resource string) (*corev1.ObjectReference, error) {
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
								return errors.Errorf("%s type: %T", testCaseName, obj)
							}
							psql.Status = *newResourceClaimStatusBuilder().withReadyStatus().build()
							return nil
						},
						MockCreate: func(ctx context.Context, obj runtime.Object) error { return nil },
					},
				},
				finder: &mockResourceClassFinder{
					mockFind: func(ctx context.Context, provider corev1.ObjectReference,
						resource string) (*corev1.ObjectReference, error) {
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
				t.Errorf("%s -got error, +want error: %s", testCaseName, diff)
			}
			if diff := cmp.Diff(r.status, tt.want.status, cmp.Comparer(test.EqualConditionedStatus)); diff != "" {
				t.Errorf("%s -got status, +want status: %s", testCaseName, diff)
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
	type fields struct {
		baseResourceReconciler *baseResourceReconciler
		resourceClassFinder    resourceClassFinder
	}
	type args struct {
		ctx          context.Context
		values       chartutil.Values
		secretPrefix string
	}
	tests := map[string]struct {
		fields fields
		args   args
		want   error
	}{
		"Failure": {
			fields: fields{
				baseResourceReconciler: newBaseResourceReconciler(newGitLabBuilder().build(), test.NewMockClient(), testBucket),
			},
			args: args{ctx: context.TODO()},
			want: errors.New(errorResourceStatusIsNotFound),
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			r := &postgresReconciler{
				baseResourceReconciler: tt.fields.baseResourceReconciler,
				resourceClassFinder:    tt.fields.resourceClassFinder,
			}
			if diff := cmp.Diff(r.getHelmValues(tt.args.ctx, tt.args.values, tt.args.secretPrefix), tt.want, cmpErrors); diff != "" {
				t.Errorf("postgresReconciler.getHelmValues() error %s", diff)
			}
		})
	}
}

func Test_postgresHelmValues(t *testing.T) {
	endpoint := "http://example.org"
	user := "cooluser"
	password := "verycoolsecret"
	secretName := "coolSecret"
	secretPrefix := "coolPrefix-"

	type args struct {
		values       chartutil.Values
		secret       *corev1.Secret
		name         string
		secretPrefix string
	}
	type want struct {
		values chartutil.Values
	}
	tests := map[string]struct {
		args args
		want want
	}{
		"EmptyValues": {
			args: args{
				values: chartutil.Values{},
				secret: &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{Name: secretName},
					Data: map[string][]byte{
						xpcorev1alpha1.ResourceCredentialsSecretEndpointKey: []byte(endpoint),
						xpcorev1alpha1.ResourceCredentialsSecretUserKey:     []byte(user),
						xpcorev1alpha1.ResourceCredentialsSecretPasswordKey: []byte(password),
					},
				},
				secretPrefix: secretPrefix,
			},
			want: want{
				values: chartutil.Values{
					valuesKeyGlobal: chartutil.Values{
						valuesKeyPSQL: chartutil.Values{
							"host":     "http://example.org",
							"database": defaultPostgresDatabase,
							"username": user,
							"password": chartutil.Values{
								"secret": secretPrefix + secretName,
								"key":    xpcorev1alpha1.ResourceCredentialsSecretPasswordKey,
							},
						},
					},
				},
			},
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			postgresHelmValues(tt.args.values, tt.args.secret, tt.args.name, tt.args.secretPrefix)
			if diff := cmp.Diff(tt.want.values, tt.args.values); diff != "" {
				t.Errorf("postgresHelmValues() -want values, +got values: %s", diff)
			}
		})
	}
}

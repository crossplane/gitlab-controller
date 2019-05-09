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
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplaneio/gitlab-controller/pkg/apis/controller/v1alpha1"
	"github.com/crossplaneio/gitlab-controller/pkg/test"
	"github.com/crossplaneio/gitlab-controller/pkg/util"
)

const testBucket = "test-bucket"

type mockSecretTransformer struct {
	mockTransform func(context.Context) error
}

func (m *mockSecretTransformer) transform(ctx context.Context) error {
	return m.mockTransform(ctx)
}

type mockSecretUpdater struct {
	mockUpdate func(*corev1.Secret) error
}

func (m *mockSecretUpdater) update(s *corev1.Secret) error {
	return m.mockUpdate(s)
}

type mockSecretConnectionCreator struct {
	mockCreate func(*corev1.Secret) error
}

func (m *mockSecretConnectionCreator) create(s *corev1.Secret) error { return m.mockCreate(s) }

func getBucketClaimType(name string) string {
	return bucketClaimKind + "-" + name
}

func Test_bucketReconciler_reconcile(t *testing.T) {
	ctx := context.TODO()
	testError := errors.New("test-error")
	type fields struct {
		bucketName  string
		base        *baseResourceReconciler
		finder      resourceClassFinder
		transformer secretTransformer
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
				bucketName: testBucket,
			},
			want: want{err: errors.Wrapf(testError, errorFmtFailedToFindResourceClass, getBucketClaimType(testBucket),
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
				bucketName: testBucket,
			},
			want: want{err: errors.Wrapf(testError, errorFmtFailedToCreate, getBucketClaimType(testBucket), testKey.String()+"-"+testBucket)},
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
				bucketName: testBucket,
			},
			want: want{err: errors.Wrapf(testError, errorFmtFailedToRetrieveInstance, getBucketClaimType(testBucket), testKey.String()+"-"+testBucket)},
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
				bucketName: testBucket,
			},
			want: want{},
		},
		"SuccessfulNotReady": {
			fields: fields{
				base: &baseResourceReconciler{
					GitLab: newGitLabBuilder().withMeta(testMeta).build(),
					client: &test.MockClient{
						MockGet: func(ctx context.Context, key client.ObjectKey, obj runtime.Object) error {
							psql, ok := obj.(*xpstoragev1alpha1.Bucket)
							if !ok {
								return errors.Errorf("bucketReconciler.reconcile() type: %T", obj)
							}
							psql.Status = *newResourceClaimStatusBuilder().withCreatingStatus().build()
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
				bucketName: testBucket,
			},
			want: want{
				status: newResourceClaimStatusBuilder().withCreatingStatus().build(),
			},
		},
		"SuccessfulReady": {
			fields: fields{
				base: &baseResourceReconciler{
					GitLab: newGitLabBuilder().withMeta(testMeta).build(),
					client: &test.MockClient{
						MockGet: func(ctx context.Context, key client.ObjectKey, obj runtime.Object) error {
							psql, ok := obj.(*xpstoragev1alpha1.Bucket)
							if !ok {
								return errors.Errorf("bucketReconciler.reconcile() type: %T", obj)
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
				transformer: &mockSecretTransformer{
					mockTransform: func(ctx context.Context) error { return nil },
				},
				bucketName: testBucket,
			},
			want: want{
				status: newResourceClaimStatusBuilder().withReadyStatus().build(),
			},
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			r := &bucketReconciler{
				baseResourceReconciler: tt.fields.base,
				resourceClassFinder:    tt.fields.finder,
				bucketName:             tt.fields.bucketName,
				secretTransformer:      tt.fields.transformer,
			}
			if diff := cmp.Diff(r.reconcile(ctx), tt.want.err, cmpErrors); diff != "" {
				t.Errorf("bucketReconciler.reconcile() error %s", diff)
			}
			if diff := cmp.Diff(r.status, tt.want.status, cmp.Comparer(util.EqualConditionedStatus)); diff != "" {
				t.Errorf("bucketReconciler.reconcile() status %s", diff)
			}
		})
	}
}

func Test_bucketReconciler_getClaimKind(t *testing.T) {
	r := &bucketReconciler{bucketName: testBucket}
	if diff := cmp.Diff(r.getClaimKind(), getBucketClaimType(testBucket)); diff != "" {
		t.Errorf("bucketReconciler.getClaimKind() %s", diff)
	}
}

func Test_newBucketReconciler(t *testing.T) {
	gitlab := &v1alpha1.GitLab{}
	clnt := test.NewMockClient()

	r := newBucketReconciler(gitlab, clnt, testBucket)
	if diff := cmp.Diff(r.GitLab, gitlab); diff != "" {
		t.Errorf("newBucketReconciler() GitLab %s", diff)
	}
}

func Test_gitlabSecretTransformer_transform(t *testing.T) {
	ctx := context.TODO()
	testError := errors.New("test-error")
	testSecret := "test-secret"
	testSecretKey := types.NamespacedName{Namespace: testNamespace, Name: testSecret}
	type fields struct {
		baseResourceReconciler *baseResourceReconciler
		secretUpdaters         map[string]secretUpdater
	}
	tests := map[string]struct {
		fields  fields
		wantErr error
	}{
		"NoStatus": {
			fields: fields{
				baseResourceReconciler: &baseResourceReconciler{
					GitLab: newGitLabBuilder().build(),
				},
			},
			wantErr: errors.New(errorResourceStatusIsNotFound),
		},
		"FailedToRetrieveSecret": {
			fields: fields{
				baseResourceReconciler: &baseResourceReconciler{
					GitLab: newGitLabBuilder().withMeta(testMeta).build(),
					client: &test.MockClient{
						MockGet: func(ctx context.Context, key client.ObjectKey, obj runtime.Object) error {
							return testError
						},
					},
					status: newResourceClaimStatusBuilder().withCredentialsSecretRef(testSecret).build(),
				},
			},
			wantErr: errors.Wrapf(testError, errorFmtFailedToRetrieveConnectionSecret, testSecretKey),
		},
		"NotSupportedProvider": {
			fields: fields{
				baseResourceReconciler: &baseResourceReconciler{
					GitLab: newGitLabBuilder().withMeta(testMeta).build(),
					client: &test.MockClient{
						MockGet: func(ctx context.Context, key client.ObjectKey, obj runtime.Object) error { return nil },
					},
					status: newResourceClaimStatusBuilder().withCredentialsSecretRef(testSecret).build(),
				},
			},
			wantErr: errors.Errorf(errorFmtNotSupportedProvider, ""),
		},
		"UpdaterFailed": {
			fields: fields{
				baseResourceReconciler: &baseResourceReconciler{
					GitLab: newGitLabBuilder().withMeta(testMeta).build(),
					client: &test.MockClient{
						MockGet: func(ctx context.Context, key client.ObjectKey, obj runtime.Object) error { return nil },
					},
					status: newResourceClaimStatusBuilder().withCredentialsSecretRef(testSecret).build(),
				},
				secretUpdaters: map[string]secretUpdater{
					"": &mockSecretUpdater{
						mockUpdate: func(secret *corev1.Secret) error { return testError },
					},
				},
			},
			wantErr: errors.Wrapf(testError, errorFmtFailedToUpdateConnectionSecretData, testSecretKey),
		},
		"FailedToUpdateSecretObject": {
			fields: fields{
				baseResourceReconciler: &baseResourceReconciler{
					GitLab: newGitLabBuilder().withMeta(testMeta).build(),
					client: &test.MockClient{
						MockGet:    func(ctx context.Context, key client.ObjectKey, obj runtime.Object) error { return nil },
						MockUpdate: func(ctx context.Context, obj runtime.Object) error { return testError },
					},
					status: newResourceClaimStatusBuilder().withCredentialsSecretRef(testSecret).build(),
				},
				secretUpdaters: map[string]secretUpdater{
					"": &mockSecretUpdater{
						mockUpdate: func(secret *corev1.Secret) error { return nil },
					},
				},
			},
			wantErr: errors.Wrapf(testError, errorFmtFailedToUpdateConnectionSecret, testSecretKey),
		},
		"Successful": {
			fields: fields{
				baseResourceReconciler: &baseResourceReconciler{
					GitLab: newGitLabBuilder().withMeta(testMeta).build(),
					client: &test.MockClient{
						MockGet:    func(ctx context.Context, key client.ObjectKey, obj runtime.Object) error { return nil },
						MockUpdate: func(ctx context.Context, obj runtime.Object) error { return nil },
					},
					status: newResourceClaimStatusBuilder().withCredentialsSecretRef(testSecret).build(),
				},
				secretUpdaters: map[string]secretUpdater{
					"": &mockSecretUpdater{
						mockUpdate: func(secret *corev1.Secret) error { return nil },
					},
				},
			},
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			tr := &gitLabSecretTransformer{
				baseResourceReconciler: tt.fields.baseResourceReconciler,
				secretUpdaters:         tt.fields.secretUpdaters,
			}
			if diff := cmp.Diff(tr.transform(ctx), tt.wantErr, cmpErrors); diff != "" {
				t.Errorf("gitLabSecretTransformer.transform() error %s", diff)
			}
		})
	}
}

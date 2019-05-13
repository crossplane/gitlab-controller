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
	"fmt"
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

type mockSecretDataCreator struct {
	mockCreate func(*corev1.Secret) error
}

func (m *mockSecretDataCreator) create(s *corev1.Secret) error { return m.mockCreate(s) }

func getBucketClaimType(name string) string {
	return bucketClaimKind + "-" + name
}

func Test_bucketReconciler_reconcile(t *testing.T) {
	ctx := context.TODO()
	testError := errors.New("test-error")
	type fields struct {
		gitlab      *v1alpha1.GitLab
		client      client.Client
		bucketName  string
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
				gitlab: newGitLabBuilder().build(),
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
				gitlab: newGitLabBuilder().withMeta(testMeta).build(),
				client: &test.MockClient{
					MockGet: func(ctx context.Context, key client.ObjectKey, obj runtime.Object) error {
						return kerrors.NewNotFound(schema.GroupResource{}, "")
					},
					MockCreate: func(ctx context.Context, obj runtime.Object) error {
						return testError
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
				gitlab: newGitLabBuilder().withMeta(testMeta).build(),
				client: &test.MockClient{
					MockGet: func(ctx context.Context, key client.ObjectKey, obj runtime.Object) error {
						return testError
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
				gitlab: newGitLabBuilder().withMeta(testMeta).build(),
				client: &test.MockClient{
					MockGet: func(ctx context.Context, key client.ObjectKey, obj runtime.Object) error {
						return kerrors.NewNotFound(schema.GroupResource{}, "")
					},
					MockCreate: func(ctx context.Context, obj runtime.Object) error { return nil },
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
				gitlab: newGitLabBuilder().withMeta(testMeta).build(),
				client: &test.MockClient{
					MockGet: func(ctx context.Context, key client.ObjectKey, obj runtime.Object) error {
						o, ok := obj.(*xpstoragev1alpha1.Bucket)
						if !ok {
							return errors.Errorf("bucketReconciler.reconcile() type: %T", obj)
						}
						o.Status = *newResourceClaimStatusBuilder().withCreatingStatus().build()
						return nil
					},
					MockCreate: func(ctx context.Context, obj runtime.Object) error { return nil },
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
				gitlab: newGitLabBuilder().withMeta(testMeta).build(),
				client: &test.MockClient{
					MockGet: func(ctx context.Context, key client.ObjectKey, obj runtime.Object) error {
						o, ok := obj.(*xpstoragev1alpha1.Bucket)
						if !ok {
							return errors.Errorf("bucketReconciler.reconcile() type: %T", obj)
						}
						o.Status = *newResourceClaimStatusBuilder().withReadyStatus().build()
						return nil
					},
					MockCreate: func(ctx context.Context, obj runtime.Object) error { return nil },
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
			f := func(map[string]string, string, *corev1.Secret) {}
			r := newBucketReconciler(tt.fields.gitlab, tt.fields.client, tt.fields.bucketName, f)
			r.resourceClassFinder = tt.fields.finder
			r.secretTransformer = tt.fields.transformer

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
	r := newBucketReconciler(newGitLabBuilder().build(), test.NewMockClient(), testBucket, nil)
	if diff := cmp.Diff(r.getClaimKind(), getBucketClaimType(testBucket)); diff != "" {
		t.Errorf("bucketReconciler.getClaimKind() %s", diff)
	}
}

func Test_bucketReconciler_getHelmValues(t *testing.T) {
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
			r := &bucketReconciler{
				baseResourceReconciler: tt.fields.baseResourceReconciler,
				resourceClassFinder:    tt.fields.resourceClassFinder,
			}
			if diff := cmp.Diff(r.getHelmValues(ctx, tt.args), tt.want, cmpErrors); diff != "" {
				t.Errorf("bucketReconciler.getHelmValues() error %s", diff)
			}
		})
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

func Test_bucketConnectionHelmValues(t *testing.T) {
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
					fmt.Sprintf(helmValueBucketFmt, "foo"):           "",
					fmt.Sprintf(helmValueConnectionSecretFmt, "foo"): "",
					fmt.Sprintf(helmValueConnectionKeyFmt, "foo"):    connectionKey,
				},
			},
		},
		"Success-WithValues": {
			args: args{
				values: make(map[string]string),
				name:   "foo",
				secret: &corev1.Secret{
					ObjectMeta: testMeta,
					Data: map[string][]byte{
						xpcorev1alpha1.ResourceCredentialsSecretEndpointKey: []byte("bar"),
					},
				},
			},
			want: want{
				data: map[string]string{
					fmt.Sprintf(helmValueBucketFmt, "foo"):           "bar",
					fmt.Sprintf(helmValueConnectionSecretFmt, "foo"): testName,
					fmt.Sprintf(helmValueConnectionKeyFmt, "foo"):    connectionKey,
				},
			},
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil && !tt.want.panic {
					t.Errorf("bucketConnectionHelmValues() panic: %v", r)
				}
			}()
			bucketConnectionHelmValues(tt.args.values, tt.args.name, tt.args.secret)
			if diff := cmp.Diff(tt.args.values, tt.want.data); diff != "" {
				t.Errorf("bucketConnectionHelmValues() %s", diff)
			}
		})
	}
}

func Test_bucketBackupsHelmValues(t *testing.T) {
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
					fmt.Sprintf(helmValueBucketFmt, "foo"): "",
					helmValueTaskRunnerSecret:              "",
					helmValueTaskRunnerKey:                 configKey,
				},
			},
		},
		"Success-WithValues": {
			args: args{
				values: make(map[string]string),
				name:   "foo",
				secret: &corev1.Secret{
					ObjectMeta: testMeta,
					Data: map[string][]byte{
						xpcorev1alpha1.ResourceCredentialsSecretEndpointKey: []byte("bar"),
					},
				},
			},
			want: want{
				data: map[string]string{
					fmt.Sprintf(helmValueBucketFmt, "foo"): "bar",
					helmValueTaskRunnerSecret:              testName,
					helmValueTaskRunnerKey:                 configKey,
				},
			},
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil && !tt.want.panic {
					t.Errorf("bucketBackupsHelmValues() panic: %v", r)
				}
			}()
			bucketBackupsHelmValues(tt.args.values, tt.args.name, tt.args.secret)
			if diff := cmp.Diff(tt.args.values, tt.want.data); diff != "" {
				t.Errorf("bucketBackupsHelmValues() %s", diff)
			}
		})
	}
}

func Test_bucketBackupsTempHelmValues(t *testing.T) {
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
					helmValueBackupsTempBucket: "foo",
				},
			},
		},
		"Success-WithValues": {
			args: args{
				values: make(map[string]string),
				name:   "foo",
				secret: &corev1.Secret{
					ObjectMeta: testMeta,
					Data: map[string][]byte{
						xpcorev1alpha1.ResourceCredentialsSecretEndpointKey: []byte("bar"),
					},
				},
			},
			want: want{
				data: map[string]string{
					helmValueBackupsTempBucket: "foo",
				},
			},
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil && !tt.want.panic {
					t.Errorf("bucketBackupsTempHelmValues() panic: %v", r)
				}
			}()
			bucketBackupsTempHelmValues(tt.args.values, tt.args.name, tt.args.secret)
			if diff := cmp.Diff(tt.args.values, tt.want.data); diff != "" {
				t.Errorf("bucketBackupsTempHelmValues() %s", diff)
			}
		})
	}
}

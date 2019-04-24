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
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/crossplaneio/gitlab-controller/pkg/test"
	"github.com/crossplaneio/gitlab-controller/pkg/util"

	"github.com/crossplaneio/gitlab-controller/pkg/apis/controller/v1alpha1"
)

// mockGitLabReconcilerMill
type mockGitLabReconcilerMill struct {
	mockReconciler func(*v1alpha1.GitLab, client.Client) reconciler
}

func (m *mockGitLabReconcilerMill) newReconciler(gl *v1alpha1.GitLab, c client.Client) reconciler {
	return m.mockReconciler(gl, c)
}

var _ reconcilerMill = &mockGitLabReconcilerMill{}

// mockReconciler
type mockReconciler struct {
	mockReconcile func(context.Context) (reconcile.Result, error)
}

func (m *mockReconciler) reconcile(ctx context.Context) (reconcile.Result, error) {
	return m.mockReconcile(ctx)
}

var _ reconciler = &mockReconciler{}

// mockResourceClassFinder
type mockResourceClassFinder struct {
	mockFind func(ctx context.Context, provider corev1.ObjectReference, resource string) (*corev1.ObjectReference, error)
}

func (m *mockResourceClassFinder) find(ctx context.Context, provider corev1.ObjectReference, resource string) (*corev1.ObjectReference, error) {
	return m.mockFind(ctx, provider, resource)
}

var _ resourceClassFinder = &mockResourceClassFinder{}

// resourceClaimStatusBuilder
type resourceClaimStatusBuilder struct {
	*xpcorev1alpha1.ResourceClaimStatus
}

func newResourceClaimStatusBuilder() *resourceClaimStatusBuilder {
	return &resourceClaimStatusBuilder{ResourceClaimStatus: &xpcorev1alpha1.ResourceClaimStatus{}}
}

func (b *resourceClaimStatusBuilder) build() *xpcorev1alpha1.ResourceClaimStatus {
	return b.ResourceClaimStatus
}

func (b *resourceClaimStatusBuilder) withReadyStatus() *resourceClaimStatusBuilder {
	b.SetReady()
	return b
}

func (b *resourceClaimStatusBuilder) withCreatingStatus() *resourceClaimStatusBuilder {
	b.SetCreating()
	return b
}

func (b *resourceClaimStatusBuilder) withFailedStatus(rsn, msg string) *resourceClaimStatusBuilder {
	b.SetFailed(rsn, msg)
	return b
}

func (b *resourceClaimStatusBuilder) withCredentialsSecretRef(name string) *resourceClaimStatusBuilder {
	b.CredentialsSecretRef = corev1.LocalObjectReference{Name: name}
	return b
}

//
type gitlabBuilder struct {
	*v1alpha1.GitLab
}

func (b *gitlabBuilder) build() *v1alpha1.GitLab { return b.GitLab }

func (b *gitlabBuilder) withMeta(meta metav1.ObjectMeta) *gitlabBuilder {
	b.ObjectMeta = meta
	return b
}

func newGitLabBuilder() *gitlabBuilder {
	return &gitlabBuilder{GitLab: &v1alpha1.GitLab{}}
}

// Tests section

func Test_baseResourceReconciler_isReady(t *testing.T) {
	tests := map[string]struct {
		reconciler *baseResourceReconciler
		want       bool
	}{
		"Default": {
			reconciler: &baseResourceReconciler{},
			want:       false,
		},
		"Empty": {
			reconciler: &baseResourceReconciler{status: newResourceClaimStatusBuilder().build()},
			want:       false,
		},
		"Ready": {
			reconciler: &baseResourceReconciler{status: newResourceClaimStatusBuilder().withReadyStatus().build()},
			want:       true,
		},
		"Failed": {
			reconciler: &baseResourceReconciler{status: newResourceClaimStatusBuilder().withFailedStatus("foo", "bar").build()},
			want:       false,
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			if diff := cmp.Diff(tt.reconciler.isReady(), tt.want); diff != "" {
				t.Errorf("base.isReady() %s", diff)
			}
		})
	}
}

func Test_baseResourceReconciler_isFailed(t *testing.T) {
	tests := map[string]struct {
		reconciler *baseResourceReconciler
		want       bool
	}{
		"Default": {
			reconciler: &baseResourceReconciler{},
			want:       false,
		},
		"Empty": {
			reconciler: &baseResourceReconciler{status: newResourceClaimStatusBuilder().build()},
			want:       false,
		},
		"Ready": {
			reconciler: &baseResourceReconciler{status: newResourceClaimStatusBuilder().withReadyStatus().build()},
			want:       false,
		},
		"Failed": {
			reconciler: &baseResourceReconciler{status: newResourceClaimStatusBuilder().withFailedStatus("foo", "bar").build()},
			want:       true,
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			if diff := cmp.Diff(tt.reconciler.isFailed(), tt.want); diff != "" {
				t.Errorf("base.isReady() %s", diff)
			}
		})
	}
}

func Test_baseResourceReconciler_getClaimConnectionSecret(t *testing.T) {
	ctx := context.TODO()
	testError := errors.New("test-error")
	testSecretName := "test-secret"
	validateKey := func(key client.ObjectKey) {
		if diff := cmp.Diff(key, types.NamespacedName{Namespace: testNamespace, Name: testSecretName}); diff != "" {
			t.Errorf("base.getClaimConnectionSecret() unexpected key\n%s", diff)
		}
	}

	type fields struct {
		GitLab *v1alpha1.GitLab
		client client.Client
		status *xpcorev1alpha1.ResourceClaimStatus
	}
	type want struct {
		sec *corev1.Secret
		err error
	}
	tests := map[string]struct {
		fields fields
		want   want
	}{
		"FailureToRetrieve": {
			fields: fields{
				client: &test.MockClient{
					MockGet: func(ctx context.Context, key client.ObjectKey, obj runtime.Object) error {
						validateKey(key)
						return testError
					},
				},
				GitLab: newGitLabBuilder().withMeta(testMeta).build(),
				status: &xpcorev1alpha1.ResourceClaimStatus{
					CredentialsSecretRef: corev1.LocalObjectReference{Name: testSecretName},
				},
			},
			want: want{
				err: errors.Wrapf(testError, errorFmtFailedToRetrieveConnectionSecret, fmt.Sprintf("%s/%s", testNamespace, testSecretName)),
			},
		},
		"Successful": {
			fields: fields{
				client: &test.MockClient{
					MockGet: func(ctx context.Context, key client.ObjectKey, obj runtime.Object) error {
						validateKey(key)
						return nil
					},
				},
				GitLab: newGitLabBuilder().withMeta(testMeta).build(),
				status: &xpcorev1alpha1.ResourceClaimStatus{
					CredentialsSecretRef: corev1.LocalObjectReference{Name: testSecretName},
				},
			},
			want: want{sec: &corev1.Secret{}},
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			r := &baseResourceReconciler{
				GitLab: tt.fields.GitLab,
				client: tt.fields.client,
				status: tt.fields.status,
			}
			got, err := r.getClaimConnectionSecret(ctx)
			if diff := cmp.Diff(err, tt.want.err, cmpErrors); diff != "" {
				t.Errorf("base.getClaimConnectionSecret() error\n%s", diff)
			}
			if diff := cmp.Diff(got, tt.want.sec); diff != "" {
				t.Errorf("base.getClaimConnectionSecret()\n%s", diff)
			}
		})
	}
}

func Test_baseResourceReconciler_findResourceClass(t *testing.T) {
	ctx := context.TODO()
	testProvisioner := "test-resource"
	testProviderName := "test-provider"
	testError := errors.New("test-error")

	newProviderRef := func(ns, n string) corev1.ObjectReference { return corev1.ObjectReference{Namespace: ns, Name: n} }
	copyInto := func(list *xpcorev1alpha1.ResourceClassList, object runtime.Object) error {
		rcl, ok := object.(*xpcorev1alpha1.ResourceClassList)
		if !ok {
			return errors.Errorf("base.find() unexpected list type: %T", object)
		}
		list.DeepCopyInto(rcl)
		return nil
	}

	if err := xpcorev1alpha1.SchemeBuilder.AddToScheme(scheme.Scheme); err != nil {
		panic(err)
	}

	type fields struct {
		GitLab *v1alpha1.GitLab
		client client.Client
		status *xpcorev1alpha1.ResourceClaimStatus
	}
	type args struct {
		provider corev1.ObjectReference
		resource string
	}
	type want struct {
		ref *corev1.ObjectReference
		err error
	}
	tests := map[string]struct {
		fields fields
		args   args
		want   want
	}{
		"ListError": {
			fields: fields{
				client: &test.MockClient{
					MockList: func(ctx context.Context, opts *client.ListOptions, list runtime.Object) error {
						return testError
					},
				},
			},
			args: args{
				provider: newProviderRef(testNamespace, testProviderName),
				resource: testProvisioner,
			},
			want: want{
				err: errors.Wrapf(testError, errorFmtFailedToListResourceClasses, testNamespace, testProviderName, testProvisioner),
			},
		},
		"EmptyProvidersList": {
			fields: fields{
				client: &test.MockClient{
					MockList: func(ctx context.Context, opts *client.ListOptions, list runtime.Object) error {
						return nil
					},
				},
			},
			args: args{
				provider: newProviderRef(testNamespace, testProviderName),
				resource: testProvisioner,
			},
			want: want{
				err: errors.Errorf(errorFmtResourceClassNotFound, testNamespace, testProviderName, testProvisioner),
			},
		},
		"NoMatchingProviders-DifferentNamespace": {
			fields: fields{
				client: &test.MockClient{
					MockList: func(ctx context.Context, opts *client.ListOptions, list runtime.Object) error {
						if err := copyInto(&xpcorev1alpha1.ResourceClassList{
							Items: []xpcorev1alpha1.ResourceClass{
								{
									ObjectMeta: metav1.ObjectMeta{
										Namespace: "foobar",
										Name:      testProviderName,
									},
								},
							},
						}, list); err != nil {
							t.Error(err)
						}
						return nil
					},
				},
			},
			args: args{
				provider: newProviderRef(testNamespace, testProviderName),
				resource: testProvisioner,
			},
			want: want{
				err: errors.Errorf(errorFmtResourceClassNotFound, testNamespace, testProviderName, testProvisioner),
			},
		},
		"NoMatchingProviders-SameNamespace": {
			fields: fields{
				client: &test.MockClient{
					MockList: func(ctx context.Context, opts *client.ListOptions, list runtime.Object) error {
						if err := copyInto(&xpcorev1alpha1.ResourceClassList{
							Items: []xpcorev1alpha1.ResourceClass{
								{
									ObjectMeta: metav1.ObjectMeta{
										Namespace: testNamespace,
										Name:      testName,
									},
									ProviderRef: corev1.LocalObjectReference{Name: "foo"},
								},
							},
						}, list); err != nil {
							t.Error(err)
						}
						return nil
					},
				},
			},
			args: args{
				provider: newProviderRef(testNamespace, testProviderName),
				resource: testProvisioner,
			},
			want: want{
				err: errors.Errorf(errorFmtResourceClassNotFound, testNamespace, testProviderName, testProvisioner),
			},
		},
		"MatchingProvider-NotMatchingResource": {
			fields: fields{
				client: &test.MockClient{
					MockList: func(ctx context.Context, opts *client.ListOptions, list runtime.Object) error {
						if err := copyInto(&xpcorev1alpha1.ResourceClassList{
							Items: []xpcorev1alpha1.ResourceClass{
								{
									ObjectMeta: metav1.ObjectMeta{
										Namespace: testNamespace,
										Name:      testName,
									},
									ProviderRef: corev1.LocalObjectReference{Name: testProviderName},
								},
							},
						}, list); err != nil {
							t.Error(err)
						}
						return nil
					},
				},
			},
			args: args{
				provider: newProviderRef(testNamespace, testProviderName),
				resource: testProvisioner,
			},
			want: want{
				err: errors.Errorf(errorFmtResourceClassNotFound, testNamespace, testProviderName, testProvisioner),
			},
		},
		"MatchingProvider-MatchingResource-Single": {
			fields: fields{
				client: &test.MockClient{
					MockList: func(ctx context.Context, opts *client.ListOptions, list runtime.Object) error {
						if err := copyInto(&xpcorev1alpha1.ResourceClassList{
							Items: []xpcorev1alpha1.ResourceClass{
								{
									ObjectMeta: metav1.ObjectMeta{
										Namespace: testNamespace,
										Name:      "thingOne",
										Annotations: map[string]string{
											resourceAnnotationKey: testProvisioner,
										},
									},
									ProviderRef: corev1.LocalObjectReference{Name: testProviderName},
								},
							},
						}, list); err != nil {
							t.Error(err)
						}
						return nil
					},
				},
			},
			args: args{
				provider: newProviderRef(testNamespace, testProviderName),
				resource: testProvisioner,
			},
			want: want{
				ref: &corev1.ObjectReference{
					Namespace: testNamespace,
					Name:      "thingOne",
				},
			},
		},
		"MatchingProvider-MatchingProvisioner-Multiple": {
			fields: fields{
				client: &test.MockClient{
					MockList: func(ctx context.Context, opts *client.ListOptions, list runtime.Object) error {
						if err := copyInto(&xpcorev1alpha1.ResourceClassList{
							Items: []xpcorev1alpha1.ResourceClass{
								{
									ObjectMeta: metav1.ObjectMeta{
										Namespace: testNamespace,
										Name:      "thingOne",
										Annotations: map[string]string{
											resourceAnnotationKey: testProvisioner,
										},
									},
									ProviderRef: corev1.LocalObjectReference{Name: testProviderName},
								},
								{
									ObjectMeta: metav1.ObjectMeta{
										Namespace: testNamespace,
										Name:      "thingTwo",
										Annotations: map[string]string{
											resourceAnnotationKey: testProvisioner,
										},
									},
									ProviderRef: corev1.LocalObjectReference{Name: testProviderName},
								},
							},
						}, list); err != nil {
							t.Error(err)
						}
						return nil
					},
				},
			},
			args: args{
				provider: newProviderRef(testNamespace, testProviderName),
				resource: testProvisioner,
			},
			want: want{
				ref: &corev1.ObjectReference{
					Namespace: testNamespace,
					Name:      "thingOne",
				},
			},
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			r := &baseResourceReconciler{
				GitLab: tt.fields.GitLab,
				client: tt.fields.client,
				status: tt.fields.status,
			}
			got, err := r.find(ctx, tt.args.provider, tt.args.resource)
			if diff := cmp.Diff(err, tt.want.err, cmpErrors); diff != "" {
				t.Errorf("base.find() error  %s", diff)
			}
			if diff := cmp.Diff(got, tt.want.ref); diff != "" {
				t.Errorf("base.find() %s", diff)
			}
		})
	}
}

func Test_baseResourceReconciler_newObjectMeta(t *testing.T) {
	tests := map[string]struct {
		fields *v1alpha1.GitLab
		args   []string
		want   metav1.ObjectMeta
	}{
		"Default": {
			fields: &v1alpha1.GitLab{},
			want: metav1.ObjectMeta{
				Labels:          map[string]string{"app": ""},
				OwnerReferences: []metav1.OwnerReference{(&v1alpha1.GitLab{}).ToOwnerReference()},
			},
		},
		"WithValues": {
			fields: &v1alpha1.GitLab{
				ObjectMeta: testMeta,
			},
			want: metav1.ObjectMeta{
				Namespace:       testNamespace,
				Name:            testName,
				Labels:          map[string]string{"app": testName},
				OwnerReferences: []metav1.OwnerReference{(&v1alpha1.GitLab{ObjectMeta: testMeta}).ToOwnerReference()},
			},
		},
		"WithSuffixes": {
			args: []string{"foo"},
			fields: &v1alpha1.GitLab{
				ObjectMeta: testMeta,
			},
			want: metav1.ObjectMeta{
				Namespace:       testNamespace,
				Name:            testName + "-foo",
				Labels:          map[string]string{"app": testName},
				OwnerReferences: []metav1.OwnerReference{(&v1alpha1.GitLab{ObjectMeta: testMeta}).ToOwnerReference()},
			},
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			r := &baseResourceReconciler{GitLab: tt.fields}
			got := r.newObjectMeta(tt.args...)
			if diff := cmp.Diff(got, tt.want); diff != "" {
				t.Errorf("base.newObjectMeta()\n%s", diff)
			}
		})
	}
}

func Test_baseResourceReconciler_newNamespacedName(t *testing.T) {
	tests := map[string]struct {
		fields *v1alpha1.GitLab
		want   types.NamespacedName
	}{
		"Default": {
			fields: &v1alpha1.GitLab{},
			want:   types.NamespacedName{},
		},
		"WithValues": {
			fields: &v1alpha1.GitLab{ObjectMeta: testMeta},
			want:   types.NamespacedName{Namespace: testNamespace, Name: testName},
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			r := &baseResourceReconciler{
				GitLab: tt.fields,
			}
			got := r.newNamespacedName()
			if diff := cmp.Diff(got, tt.want); diff != "" {
				t.Errorf("base.newNamespacedName() %s", diff)
			}
		})
	}
}

func Test_newBaseComponentReconciler(t *testing.T) {
	type args struct {
		gitlab *v1alpha1.GitLab
		client client.Client
	}
	tests := map[string]struct {
		args args
		want *baseResourceReconciler
	}{
		"Default": {
			args: args{
				gitlab: &v1alpha1.GitLab{},
			},
			want: &baseResourceReconciler{
				GitLab: &v1alpha1.GitLab{},
			},
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			got := newBaseComponentReconciler(tt.args.gitlab, tt.args.client)
			if diff := cmp.Diff(got, tt.want, cmpopts.IgnoreUnexported(baseResourceReconciler{})); diff != "" {
				t.Errorf("newBaseComponentReconciler() %s", diff)
			}
		})
	}
}

func Test_gitLabReconciler_fail(t *testing.T) {
	ctx := context.TODO()
	testError := errors.New("test-error")

	type fields struct {
		GitLab *v1alpha1.GitLab
		client client.Client
	}
	type args struct {
		reason string
		msg    string
	}
	type want struct {
		err error
		cds *xpcorev1alpha1.ConditionedStatus
	}
	tests := map[string]struct {
		fields fields
		args   args
		want   want
	}{
		"Default": {
			fields: fields{
				GitLab: &v1alpha1.GitLab{},
				client: &test.MockClient{
					MockStatusUpdate: func(ctx context.Context, obj runtime.Object) error { return nil },
				},
			},
			args: args{reason: "foo", msg: "bar"},
			want: want{
				cds: test.NewConditionedStatusBuilder().WithFailedCondition("foo", "bar").Build(),
			},
		},
		"StatusUpdateFailure": {
			fields: fields{
				GitLab: &v1alpha1.GitLab{},
				client: &test.MockClient{
					MockStatusUpdate: func(ctx context.Context, obj runtime.Object) error { return testError },
				},
			},
			args: args{reason: "foo", msg: "bar"},
			want: want{
				cds: test.NewConditionedStatusBuilder().WithFailedCondition("foo", "bar").Build(),
				err: testError,
			},
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			r := &gitLabReconciler{
				GitLab: tt.fields.GitLab,
				client: tt.fields.client,
			}
			if diff := cmp.Diff(r.fail(ctx, tt.args.reason, tt.args.msg), tt.want.err, cmpErrors); diff != "" {
				t.Errorf("gitLabReconciler.fail() error %s", diff)
			}
			if diff := cmp.Diff(&r.GitLab.Status.ConditionedStatus, tt.want.cds,
				cmp.Comparer(util.EqualConditionedStatus)); diff != "" {
				t.Errorf("gitLabReconciler.fail() conditions %s", diff)
			}
		})
	}
}

func Test_gitLabReconciler_pending(t *testing.T) {
	ctx := context.TODO()

	testError := errors.New("test-error")

	type fields struct {
		GitLab *v1alpha1.GitLab
		client client.Client
	}
	type args struct {
		reason string
		msg    string
	}
	type want struct {
		err error
		cds []xpcorev1alpha1.Condition
	}
	tests := map[string]struct {
		fields fields
		args   args
		want   want
	}{
		"Default": {
			fields: fields{
				GitLab: &v1alpha1.GitLab{},
				client: &test.MockClient{
					MockStatusUpdate: func(ctx context.Context, obj runtime.Object) error { return nil },
				},
			},
			args: args{reason: "foo", msg: "bar"},
			want: want{
				cds: []xpcorev1alpha1.Condition{
					{
						Type:    xpcorev1alpha1.Pending,
						Reason:  "foo",
						Message: "bar",
						Status:  corev1.ConditionTrue,
					},
				},
			},
		},
		"StatusUpdateFailure": {
			fields: fields{
				GitLab: &v1alpha1.GitLab{},
				client: &test.MockClient{
					MockStatusUpdate: func(ctx context.Context, obj runtime.Object) error { return testError },
				},
			},
			args: args{reason: "foo", msg: "bar"},
			want: want{
				cds: []xpcorev1alpha1.Condition{
					{
						Type:    xpcorev1alpha1.Pending,
						Reason:  "foo",
						Message: "bar",
						Status:  corev1.ConditionTrue,
					},
				},
				err: testError,
			},
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			r := &gitLabReconciler{
				GitLab: tt.fields.GitLab,
				client: tt.fields.client,
			}
			if diff := cmp.Diff(r.pending(ctx, tt.args.reason, tt.args.msg), tt.want.err, cmpErrors); diff != "" {
				t.Errorf("gitLabReconciler.pending() error %s", diff)
			}
			if diff := cmp.Diff(r.GitLab.Status.Conditions, tt.want.cds,
				cmp.Comparer(util.EqualConditionedStatus)); diff != "" {
				t.Errorf("gitLabReconciler.pending() conditions %s", diff)
			}
		})
	}
}

func Test_gitLabReconciler_update(t *testing.T) {
	ctx := context.TODO()
	testError := errors.New("test-error")
	tests := map[string]struct {
		client  client.Client
		wantErr error
	}{
		"Successful": {
			client: &test.MockClient{
				MockStatusUpdate: func(ctx context.Context, obj runtime.Object) error { return nil },
			},
		},
		"Failure": {
			client: &test.MockClient{
				MockStatusUpdate: func(ctx context.Context, obj runtime.Object) error { return testError },
			},
			wantErr: testError,
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			r := &gitLabReconciler{
				client: tt.client,
			}
			if diff := cmp.Diff(r.update(ctx), tt.wantErr, cmpErrors); diff != "" {
				t.Errorf("gitLabReconciler.update() error %s", diff)
			}
		})
	}
}

//func Test_gitLabReconciler_reconcile(t *testing.T) {
//	type fields struct {
//		GitLab         *v1alpha1.GitLab
//		client         client.Client
//		resourceClaims []resourceReconciler
//	}
//	type args struct {
//		ctx context.Context
//	}
//	tests := map[string]struct {
//		fields  fields
//		args    args
//		want    reconcile.Result
//		wantErr bool
//	}{
//		// TODO: Add test cases.
//	}
//	for name, tt := range tests {
//		t.Run(name, func(t *testing.T) {
//			r := &gitLabReconciler{
//				GitLab:         tt.fields.GitLab,
//				client:         tt.fields.client,
//				resourceClaims: tt.fields.resourceClaims,
//			}
//			got, err := r.reconcile(tt.args.ctx)
//			if (err != nil) != tt.wantErr {
//				t.Errorf("gitLabReconciler.reconcile() error = %v, wantErr %v", err, tt.wantErr)
//				return
//			}
//			if diff := cmp.Diff(got, tt.want); diff != "" {
//				t.Errorf("gitLabReconciler.reconcile() %s", diff)
//			}
//		})
//	}
//}
//
//func Test_gitLabReconciler_reconcileClaims(t *testing.T) {
//	type fields struct {
//		GitLab         *v1alpha1.GitLab
//		client         client.Client
//		resourceClaims []resourceReconciler
//	}
//	type args struct {
//		ctx    context.Context
//		claims []resourceReconciler
//	}
//	tests := map[string]struct {
//		fields  fields
//		args    args
//		want    reconcile.Result
//		wantErr bool
//	}{
//		// TODO: Add test cases.
//	}
//	for name, tt := range tests {
//		t.Run(name, func(t *testing.T) {
//			r := &gitLabReconciler{
//				GitLab:         tt.fields.GitLab,
//				client:         tt.fields.client,
//				resourceClaims: tt.fields.resourceClaims,
//			}
//			got, err := r.reconcileClaims(tt.args.ctx, tt.args.claims)
//			if (err != nil) != tt.wantErr {
//				t.Errorf("gitLabReconciler.reconcileClaims() error = %v, wantErr %v", err, tt.wantErr)
//				return
//			}
//			if diff := cmp.Diff(got, tt.want); diff != "" {
//				t.Errorf("gitLabReconciler.reconcileClaims() %s", diff)
//			}
//		})
//	}
//}
//
//func Test_gitLabReconciler_reconcileApplication(t *testing.T) {
//	type fields struct {
//		GitLab         *v1alpha1.GitLab
//		client         client.Client
//		resourceClaims []resourceReconciler
//	}
//	type args struct {
//		ctx    context.Context
//		claims []resourceReconciler
//	}
//	tests := map[string]struct {
//		name    string
//		fields  fields
//		args    args
//		want    reconcile.Result
//		wantErr bool
//	}{
//		// TODO: Add test cases.
//	}
//	for name, tt := range tests {
//		t.Run(name, func(t *testing.T) {
//			r := &gitLabReconciler{
//				GitLab:         tt.fields.GitLab,
//				client:         tt.fields.client,
//				resourceClaims: tt.fields.resourceClaims,
//			}
//			got, err := r.reconcileApplication(tt.args.ctx, tt.args.claims)
//			if (err != nil) != tt.wantErr {
//				t.Errorf("gitLabReconciler.reconcileApplication() error = %v, wantErr %v", err, tt.wantErr)
//				return
//			}
//			if diff := cmp.Diff(got, tt.want); diff != "" {
//				t.Errorf("gitLabReconciler.reconcileApplication() %s", diff)
//			}
//		})
//	}
//}
//
//func Test_newGitLabReconciler(t *testing.T) {
//	type args struct {
//		gitlab *v1alpha1.GitLab
//		client client.Client
//	}
//	tests := map[string]struct {
//		name string
//		args args
//		want *gitLabReconciler
//	}{
//		// TODO: Add test cases.
//	}
//	for name, tt := range tests {
//		t.Run(name, func(t *testing.T) {
//			got := newGitLabReconciler(tt.args.gitlab, tt.args.client)
//			if diff := cmp.Diff(got, tt.want); diff != "" {
//				t.Errorf("newGitLabReconciler() %s", diff)
//			}
//		})
//	}
//}
//
//func Test_gitLabReconcilerMill_newReconciler(t *testing.T) {
//	type args struct {
//		gitlab *v1alpha1.GitLab
//		client client.Client
//	}
//	tests := map[string]struct {
//		name string
//		m    *gitLabReconcilerMill
//		args args
//		want reconciler
//	}{
//		// TODO: Add test cases.
//	}
//	for name, tt := range tests {
//		t.Run(name, func(t *testing.T) {
//			m := &gitLabReconcilerMill{}
//			got := m.newReconciler(tt.args.gitlab, tt.args.client)
//			if diff := cmp.Diff(got, tt.want); diff != "" {
//				t.Errorf("gitLabReconcilerMill.newReconciler() %s", diff)
//			}
//		})
//	}
//}

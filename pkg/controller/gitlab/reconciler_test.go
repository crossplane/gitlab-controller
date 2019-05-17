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
	xpworkloadv1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/workload/v1alpha1"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/helm/pkg/chartutil"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/crossplaneio/gitlab-controller/pkg/apis/controller/v1alpha1"
	"github.com/crossplaneio/gitlab-controller/pkg/controller/gitlab/application"
	"github.com/crossplaneio/gitlab-controller/pkg/controller/gitlab/resource/helm"
	"github.com/crossplaneio/gitlab-controller/pkg/test"
)

var _ componentsReconciler = &applicationReconciler{}

const testResource = "test-resource"

func newMockHelmValuesFn(err error) helmValuesFunction {
	return func(_ chartutil.Values, _ *corev1.Secret, _, _ string) error { return err }
}

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

func (m *mockResourceClassFinder) find(ctx context.Context, provider corev1.ObjectReference,
	resource string) (*corev1.ObjectReference, error) {
	return m.mockFind(ctx, provider, resource)
}

var _ resourceClassFinder = &mockResourceClassFinder{}

// mockResourceClaimsReconciler
type mockComponentsReconciler struct {
	mockReconcile func(context.Context, []resourceReconciler) (reconcile.Result, error)
}

func (m *mockComponentsReconciler) reconcile(ctx context.Context, rs []resourceReconciler) (reconcile.Result, error) {
	return m.mockReconcile(ctx, rs)
}

var _ componentsReconciler = &mockComponentsReconciler{}

// mockResourceReconciler
type mockResourceReconciler struct {
	mockReconcile                    func(context.Context) error
	mockIsReady                      func() bool
	mockIsFailed                     func() bool
	mockGetClaimKind                 func() string
	mockGetClaimRef                  func() *corev1.ObjectReference
	mockGetClaimConnectionSecretName func() string
	mockGetClaimConnectionSecret     func(context.Context) (*corev1.Secret, error)
	mockGetHelmValues                func(context.Context, chartutil.Values, string) error
}

func (m *mockResourceReconciler) reconcile(ctx context.Context) error {
	return m.mockReconcile(ctx)
}
func (m *mockResourceReconciler) isReady() bool {
	return m.mockIsReady()
}
func (m *mockResourceReconciler) isFailed() bool {
	return m.mockIsFailed()
}
func (m *mockResourceReconciler) getClaimKind() string {
	return m.mockGetClaimKind()
}
func (m *mockResourceReconciler) getClaimRef() *corev1.ObjectReference {
	return m.mockGetClaimRef()
}
func (m *mockResourceReconciler) getClaimConnectionSecretName() string {
	return m.mockGetClaimConnectionSecretName()
}
func (m *mockResourceReconciler) getClaimConnectionSecret(ctx context.Context) (*corev1.Secret, error) {
	return m.mockGetClaimConnectionSecret(ctx)
}
func (m *mockResourceReconciler) getHelmValues(ctx context.Context, v chartutil.Values, secretPrefix string) error {
	return m.mockGetHelmValues(ctx, v, secretPrefix)
}

var _ resourceReconciler = &mockResourceReconciler{}

// resourceClaimStatusBuilder
type resourceClaimStatusBuilder struct {
	*xpcorev1alpha1.ResourceClaimStatus
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
func newResourceClaimStatusBuilder() *resourceClaimStatusBuilder {
	return &resourceClaimStatusBuilder{ResourceClaimStatus: &xpcorev1alpha1.ResourceClaimStatus{}}
}

// gitlabBuilder
type gitlabBuilder struct {
	*v1alpha1.GitLab
}

func (b *gitlabBuilder) build() *v1alpha1.GitLab { return b.GitLab }
func (b *gitlabBuilder) withMeta(meta metav1.ObjectMeta) *gitlabBuilder {
	b.ObjectMeta = meta
	return b
}
func (b *gitlabBuilder) withSpecClusterRef(ref *corev1.ObjectReference) *gitlabBuilder {
	b.GitLab.Spec.ClusterRef = ref
	return b
}
func (b *gitlabBuilder) withSpecDomain(domain string) *gitlabBuilder {
	b.GitLab.Spec.Domain = domain
	return b
}
func newGitLabBuilder() *gitlabBuilder {
	return &gitlabBuilder{GitLab: &v1alpha1.GitLab{}}
}

//
type statusBuilder struct {
	*xpcorev1alpha1.ConditionedStatus
}

func (b *statusBuilder) build() *xpcorev1alpha1.ConditionedStatus { return b.ConditionedStatus }
func (b *statusBuilder) withFailed(r, m string) *statusBuilder {
	b.SetFailed(r, m)
	return b
}

func newStatusBuilder() *statusBuilder {
	return &statusBuilder{ConditionedStatus: &xpcorev1alpha1.ConditionedStatus{}}
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
			reconciler: &baseResourceReconciler{status: newResourceClaimStatusBuilder().
				withFailedStatus("foo", "bar").build()},
			want: false,
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
			reconciler: &baseResourceReconciler{status: newResourceClaimStatusBuilder().
				withFailedStatus("foo", "bar").build()},
			want: true,
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
				resource: testResource,
			},
			want: want{
				err: errors.Wrapf(testError, errorFmtFailedToListResourceClasses, testNamespace, testProviderName, testResource),
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
				resource: testResource,
			},
			want: want{
				err: errors.Errorf(errorFmtResourceClassNotFound, testNamespace, testProviderName, testResource),
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
				resource: testResource,
			},
			want: want{
				err: errors.Errorf(errorFmtResourceClassNotFound, testNamespace, testProviderName, testResource),
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
				resource: testResource,
			},
			want: want{
				err: errors.Errorf(errorFmtResourceClassNotFound, testNamespace, testProviderName, testResource),
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
			args: args{provider: newProviderRef(testNamespace, testProviderName), resource: testResource},
			want: want{
				err: errors.Errorf(errorFmtResourceClassNotFound, testNamespace, testProviderName, testResource),
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
											annotationResource: testResource,
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
				resource: testResource,
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
											annotationResource: testResource,
										},
									},
									ProviderRef: corev1.LocalObjectReference{Name: testProviderName},
								},
								{
									ObjectMeta: metav1.ObjectMeta{
										Namespace: testNamespace,
										Name:      "thingTwo",
										Annotations: map[string]string{
											annotationResource: testResource,
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
				resource: testResource,
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

func Test_baseResourceReconciler_loadHelmValues(t *testing.T) {
	testError := errors.New("test-error")
	testSecretKey := types.NamespacedName{Namespace: testNamespace, Name: "test-secret"}

	type fields struct {
		GitLab *v1alpha1.GitLab
		client client.Client
		status *xpcorev1alpha1.ResourceClaimStatus
	}
	type args struct {
		ctx          context.Context
		values       chartutil.Values
		function     helmValuesFunction
		secretPrefix string
	}
	tests := map[string]struct {
		fields fields
		args   args
		want   error
	}{
		"FailureNilStatus": {
			fields: fields{
				GitLab: newGitLabBuilder().build(),
			},
			args: args{
				ctx: context.TODO(),
			},
			want: errors.New(errorResourceStatusIsNotFound),
		},
		"FailureRetrievingConnectionSecret": {
			fields: fields{
				GitLab: newGitLabBuilder().withMeta(testMeta).build(),
				client: &test.MockClient{
					MockGet: func(ctx context.Context, key client.ObjectKey, obj runtime.Object) error {
						return testError
					},
				},
				status: &xpcorev1alpha1.ResourceClaimStatus{
					CredentialsSecretRef: corev1.LocalObjectReference{Name: testSecretKey.Name},
				},
			},
			args: args{
				ctx: context.TODO(),
			},
			want: errors.Wrapf(testError, errorFmtFailedToRetrieveConnectionSecret, testSecretKey),
		},
		"Successful": {
			fields: fields{
				GitLab: newGitLabBuilder().withMeta(testMeta).build(),
				client: &test.MockClient{
					MockGet: func(ctx context.Context, key client.ObjectKey, obj runtime.Object) error {
						if _, ok := obj.(*corev1.Secret); !ok {
							t.Errorf("baseResourceReconciler.loadHelmValues() unexpected type %T", obj)
						}
						if key != testSecretKey {
							t.Errorf("baseResourceReconciler.loadHelmValues() unexpected key value %s", key)
						}
						return nil
					},
				},
				status: &xpcorev1alpha1.ResourceClaimStatus{
					CredentialsSecretRef: corev1.LocalObjectReference{Name: testSecretKey.Name},
				},
			},
			args: args{
				ctx:      context.TODO(),
				function: newMockHelmValuesFn(nil),
			},
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			r := newBaseResourceReconciler(tt.fields.GitLab, tt.fields.client, "foo")
			r.status = tt.fields.status
			if diff := cmp.Diff(r.loadHelmValues(tt.args.ctx, tt.args.values, tt.args.function, tt.args.secretPrefix), tt.want, cmpErrors); diff != "" {
				t.Errorf("baseResourceReconciler.loadHelmValues() error %s", diff)
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
			got := newBaseResourceReconciler(tt.args.gitlab, tt.args.client, "foo")
			if diff := cmp.Diff(got, tt.want, cmpopts.IgnoreUnexported(baseResourceReconciler{})); diff != "" {
				t.Errorf("newBaseResourceReconciler() %s", diff)
			}
		})
	}
}

func Test_handler_fail(t *testing.T) {
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
			r := &handle{
				GitLab: tt.fields.GitLab,
				client: tt.fields.client,
			}
			if diff := cmp.Diff(r.fail(ctx, tt.args.reason, tt.args.msg), tt.want.err, cmpErrors); diff != "" {
				t.Errorf("gitLabReconciler.fail() error %s", diff)
			}
			if diff := cmp.Diff(&r.GitLab.Status.ConditionedStatus, tt.want.cds,
				cmp.Comparer(test.EqualConditionedStatus)); diff != "" {
				t.Errorf("gitLabReconciler.fail() conditions %s", diff)
			}
		})
	}
}

func Test_handler_pending(t *testing.T) {
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
			r := &handle{
				GitLab: tt.fields.GitLab,
				client: tt.fields.client,
			}
			if diff := cmp.Diff(r.pending(ctx, tt.args.reason, tt.args.msg), tt.want.err, cmpErrors); diff != "" {
				t.Errorf("gitLabReconciler.pending() error %s", diff)
			}
			if diff := cmp.Diff(r.GitLab.Status.Conditions, tt.want.cds,
				cmp.Comparer(test.EqualConditionedStatus)); diff != "" {
				t.Errorf("gitLabReconciler.pending() conditions %s", diff)
			}
		})
	}
}

func Test_handler_update(t *testing.T) {
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
			r := &handle{
				client: tt.client,
			}
			if diff := cmp.Diff(r.update(ctx), tt.wantErr, cmpErrors); diff != "" {
				t.Errorf("gitLabReconciler.update() error %s", diff)
			}
		})
	}
}

func Test_resourceClaimsReconciler_reconcile(t *testing.T) {
	ctx := context.TODO()
	testError := errors.New("test-error")
	type fields struct {
		handle *handle
	}
	type args struct {
		claims []resourceReconciler
	}
	type want struct {
		err    error
		res    reconcile.Result
		status *xpcorev1alpha1.ConditionedStatus
	}
	tests := map[string]struct {
		fields fields
		args   args
		want   want
	}{
		"Empty": {
			fields: fields{handle: &handle{}},
			args:   args{},
			want:   want{res: reconcileSuccess},
		},
		"ReconcileError": {
			fields: fields{
				handle: &handle{
					GitLab: newGitLabBuilder().build(),
					client: test.NewMockClient(),
				},
			},
			args: args{claims: []resourceReconciler{
				&mockResourceReconciler{
					mockReconcile: func(ctx context.Context) error { return testError },
				},
			}},
			want: want{
				res:    reconcileFailure,
				status: newStatusBuilder().withFailed(reasonResourceProcessingFailure, testError.Error()).build(),
			},
		},
		"HasFailure": {
			fields: fields{
				handle: &handle{
					GitLab: newGitLabBuilder().build(),
					client: test.NewMockClient(),
				},
			},
			args: args{claims: []resourceReconciler{
				&mockResourceReconciler{
					mockReconcile:    func(ctx context.Context) error { return nil },
					mockGetClaimKind: func() string { return "test-failed-resource" },
					mockIsFailed:     func() bool { return true },
				},
			}},
			want: want{
				res:    reconcileFailure,
				status: newStatusBuilder().withFailed(reasonHasFailedResources, "test-failed-resource").build(),
			},
		},
		"HasFailureAndPending": {
			fields: fields{
				handle: &handle{
					GitLab: newGitLabBuilder().build(),
					client: test.NewMockClient(),
				},
			},
			args: args{claims: []resourceReconciler{
				&mockResourceReconciler{
					mockReconcile:    func(ctx context.Context) error { return nil },
					mockGetClaimKind: func() string { return "test-pending-resource" },
					mockIsFailed:     func() bool { return true },
				},
				&mockResourceReconciler{
					mockReconcile:    func(ctx context.Context) error { return nil },
					mockGetClaimKind: func() string { return "test-pending-resource" },
					mockIsFailed:     func() bool { return false },
					mockIsReady:      func() bool { return false },
				},
			}},
			want: want{
				res:    reconcileFailure,
				status: newStatusBuilder().withFailed(reasonHasFailedResources, "test-failed-resource").build(),
			},
		},
		"HasPendingOnly": {
			fields: fields{
				handle: &handle{
					GitLab: newGitLabBuilder().build(),
					client: test.NewMockClient(),
				},
			},
			args: args{claims: []resourceReconciler{
				&mockResourceReconciler{
					mockReconcile:    func(ctx context.Context) error { return nil },
					mockGetClaimKind: func() string { return "test-pending-resource" },
					mockIsFailed:     func() bool { return false },
					mockIsReady:      func() bool { return false },
				},
			}},
			want: want{
				res:    reconcileWait,
				status: newStatusBuilder().withFailed(reasonHasPendingResources, "test-pending-resource").build(),
			},
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			r := &resourceClaimsReconciler{
				handle: tt.fields.handle,
			}
			got, err := r.reconcile(ctx, tt.args.claims)
			if diff := cmp.Diff(err, tt.want.err, cmpErrors); diff != "" {
				t.Errorf("resourceClaimsReconciler.reconcile() error %s", diff)
				return
			}
			if diff := cmp.Diff(got, tt.want.res); diff != "" {
				t.Errorf("resourceClaimsReconciler.reconcile() %s", diff)
			}
		})
	}
}

type mockChartRenderer struct {
	r   helm.Resources
	err error
}

func (r *mockChartRenderer) render(name string, o ...helm.Option) (helm.Resources, error) {
	return r.r, r.err
}

type mockAppCreator struct {
	app *xpworkloadv1alpha1.KubernetesApplication
}

func (p *mockAppCreator) create(_ string, _ application.Templates, _ ...application.Option) *xpworkloadv1alpha1.KubernetesApplication {
	return p.app
}

func Test_applicationReconciler_reconcile(t *testing.T) {
	testError := errors.New("test-error")
	ctrl := &v1alpha1.GitLab{ObjectMeta: metav1.ObjectMeta{UID: types.UID("coolUID")}}
	ref := metav1.NewControllerRef(ctrl, v1alpha1.GitLabGroupVersionKind)
	secretName := "coolSecret"

	type fields struct {
		handle      *handle
		chartURL    string
		chart       chartRenderer
		application applicationCreator
	}
	type args struct {
		ctx       context.Context
		resources []resourceReconciler
	}
	type want struct {
		result reconcile.Result
		err    error
	}
	tests := map[string]struct {
		fields fields
		args   args
		want   want
	}{
		"HelmValuesFailure": {
			fields: fields{
				handle: &handle{
					GitLab: newGitLabBuilder().build(),
					client: &test.MockClient{
						MockStatusUpdate: func(ctx context.Context, obj runtime.Object) error {
							gl, ok := obj.(*v1alpha1.GitLab)
							if !ok {
								t.Errorf("applicationReconciler.reconcile() unexpected type %T", obj)
							}
							if gl.Status.State != xpcorev1alpha1.Failed {
								t.Errorf("applicationReconciler.reconcile() unexpected status.state %s", gl.Status.State)
							}
							return nil
						},
					},
				},
			},
			args: args{
				ctx: context.TODO(),
				resources: []resourceReconciler{
					&mockResourceReconciler{
						mockGetClaimKind:                 func() string { return testResource },
						mockIsReady:                      func() bool { return true },
						mockGetClaimConnectionSecretName: func() string { return secretName },
						mockGetHelmValues:                func(context.Context, chartutil.Values, string) error { return testError },
					},
				},
			},
			want: want{result: reconcileFailure},
		},
		"ProduceResourcesFailure": {
			fields: fields{
				handle: &handle{
					GitLab: newGitLabBuilder().build(),
					client: &test.MockClient{
						MockGet: test.NewMockGetFn(nil),
						MockStatusUpdate: func(ctx context.Context, obj runtime.Object) error {
							gl, ok := obj.(*v1alpha1.GitLab)
							if !ok {
								t.Errorf("applicationReconciler.reconcile() unexpected type %T", obj)
							}
							if gl.Status.State != xpcorev1alpha1.Failed {
								t.Errorf("applicationReconciler.reconcile() unexpected status.state %s", gl.Status.State)
							}
							return nil
						},
					},
				},
				chart: &mockChartRenderer{err: testError},
			},
			args: args{
				ctx: context.TODO(),
				resources: []resourceReconciler{
					&mockResourceReconciler{
						mockGetClaimKind:                 func() string { return testResource },
						mockIsReady:                      func() bool { return true },
						mockGetClaimConnectionSecretName: func() string { return secretName },
						mockGetHelmValues:                func(context.Context, chartutil.Values, string) error { return nil },
					},
				},
			},
			want: want{result: reconcileFailure},
		},
		"CreateOrUpdateAppFailure": {
			fields: fields{
				handle: &handle{
					GitLab: newGitLabBuilder().build(),
					client: &test.MockClient{
						MockGet: func(ctx context.Context, key client.ObjectKey, obj runtime.Object) error {
							evil := &v1alpha1.GitLab{ObjectMeta: metav1.ObjectMeta{UID: types.UID("evilUID")}}
							ref := metav1.NewControllerRef(evil, v1alpha1.GitLabGroupVersionKind)
							*obj.(*xpworkloadv1alpha1.KubernetesApplication) = xpworkloadv1alpha1.KubernetesApplication{
								ObjectMeta: metav1.ObjectMeta{OwnerReferences: []metav1.OwnerReference{*ref}},
							}

							return nil
						},
						MockStatusUpdate: func(ctx context.Context, obj runtime.Object) error {
							gl, ok := obj.(*v1alpha1.GitLab)
							if !ok {
								t.Errorf("applicationReconciler.reconcile() unxpected type %T", obj)
							}
							if gl.Status.State != xpcorev1alpha1.Failed {
								t.Errorf("applicationReconciler.reconcile() unxpected status.state %s", gl.Status.State)
							}
							return nil
						},
					},
				},
				chart: &mockChartRenderer{},
				application: &mockAppCreator{app: &xpworkloadv1alpha1.KubernetesApplication{
					ObjectMeta: metav1.ObjectMeta{OwnerReferences: []metav1.OwnerReference{*ref}},
				}},
			},
			args: args{
				ctx: context.TODO(),
				resources: []resourceReconciler{
					&mockResourceReconciler{
						mockGetClaimKind:                 func() string { return testResource },
						mockIsReady:                      func() bool { return true },
						mockGetClaimConnectionSecretName: func() string { return secretName },
						mockGetHelmValues:                func(context.Context, chartutil.Values, string) error { return nil },
					},
				},
			},
			want: want{result: reconcileFailure},
		},
		"Successful": {
			fields: fields{
				handle: &handle{
					GitLab: newGitLabBuilder().build(),
					client: test.NewMockClient(),
				},
				chart: &mockChartRenderer{},
				application: &mockAppCreator{app: &xpworkloadv1alpha1.KubernetesApplication{
					ObjectMeta: metav1.ObjectMeta{OwnerReferences: []metav1.OwnerReference{*ref}},
				}},
			},
			args: args{
				ctx: context.TODO(),
				resources: []resourceReconciler{
					&mockResourceReconciler{
						mockGetClaimKind:                 func() string { return testResource },
						mockIsReady:                      func() bool { return true },
						mockGetClaimConnectionSecretName: func() string { return secretName },
						mockGetHelmValues:                func(context.Context, chartutil.Values, string) error { return nil },
					},
				},
			},
			want: want{result: reconcileSuccess},
		},
		"Unnecessary": {
			fields: fields{
				handle: &handle{
					GitLab: newGitLabBuilder().build(),
					client: &test.MockClient{
						MockGet: func(_ context.Context, _ client.ObjectKey, obj runtime.Object) error {
							*obj.(*xpworkloadv1alpha1.KubernetesApplication) = xpworkloadv1alpha1.KubernetesApplication{
								ObjectMeta: metav1.ObjectMeta{
									Annotations: map[string]string{
										annotationGitlabChartURL: gitlabChartURL,
										annotationGitlabHash:     hash(newGitLabBuilder().build(), defaultValues(newGitLabBuilder().build())),
									},
								},
							}
							return nil
						},
						MockStatusUpdate: func(_ context.Context, obj runtime.Object) error {
							gl, ok := obj.(*v1alpha1.GitLab)
							if !ok {
								t.Errorf("applicationReconciler.reconcile() unexpected type %T", obj)
							}
							if gl.Status.State != xpcorev1alpha1.Ready {
								t.Errorf("applicationReconciler.reconcile() unexpected status.state %s", gl.Status.State)
							}
							return nil
						},
					},
				},
				chartURL:    gitlabChartURL,
				chart:       &mockChartRenderer{},
				application: &mockAppCreator{},
			},
			args: args{ctx: context.TODO()},
			want: want{result: reconcileSuccess},
		},
		"NeedsReconcileFailure": {
			fields: fields{
				handle: &handle{
					GitLab: newGitLabBuilder().build(),
					client: &test.MockClient{
						MockGet: test.NewMockGetFn(testError),
						MockStatusUpdate: func(_ context.Context, obj runtime.Object) error {
							gl, ok := obj.(*v1alpha1.GitLab)
							if !ok {
								t.Errorf("applicationReconciler.reconcile() unexpected type %T", obj)
							}
							if gl.Status.State != xpcorev1alpha1.Failed {
								t.Errorf("applicationReconciler.reconcile() unexpected status.state %s", gl.Status.State)
							}
							return nil
						},
					},
				},
				chart:       &mockChartRenderer{},
				application: &mockAppCreator{},
			},
			args: args{ctx: context.TODO()},
			want: want{result: reconcileFailure},
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			a := &applicationReconciler{
				handle:      tt.fields.handle,
				chartURL:    tt.fields.chartURL,
				chart:       tt.fields.chart,
				application: tt.fields.application,
			}
			result, err := a.reconcile(tt.args.ctx, tt.args.resources)
			if diff := cmp.Diff(tt.want.err, err, cmpErrors); diff != "" {
				t.Errorf("applicationReconciler.reconcile(): -want error, +got error: %s", diff)
				return
			}
			if diff := cmp.Diff(tt.want.result, result); diff != "" {
				t.Errorf("applicationReconciler.reconcile(): -want result, +got result: %s", diff)
			}
		})
	}
}

func Test_gitLabReconciler_reconcile(t *testing.T) {
	ctx := context.TODO()
	testDomain := "foo.bar"
	testError := errors.New("test-error")
	type fields struct {
		gitlab           *v1alpha1.GitLab
		claimsReconciler componentsReconciler
		appsReconciler   componentsReconciler
	}
	type want struct {
		res reconcile.Result
		err error
	}
	tests := map[string]struct {
		fields fields
		want   want
	}{
		"ReconcileResourceClaimsWithError": {
			fields: fields{
				gitlab: newGitLabBuilder().withMeta(testMeta).withSpecDomain(testDomain).build(),
				claimsReconciler: &mockComponentsReconciler{
					mockReconcile: func(ctx context.Context, reconcilers []resourceReconciler) (reconcile.Result, error) {
						return reconcileFailure, testError
					},
				},
			},
			want: want{
				res: reconcileFailure,
				err: testError,
			},
		},
		"ReconcileResourceClaimsWithFailedStatus": {
			fields: fields{
				gitlab: newGitLabBuilder().withMeta(testMeta).withSpecDomain(testDomain).build(),
				claimsReconciler: &mockComponentsReconciler{
					mockReconcile: func(ctx context.Context, reconcilers []resourceReconciler) (reconcile.Result, error) {
						return reconcileFailure, nil
					},
				},
			},
			want: want{
				res: reconcileFailure,
				err: nil,
			},
		},
		"ReconcileResourceClaimsWithPendingStatus": {
			fields: fields{
				gitlab: newGitLabBuilder().withMeta(testMeta).withSpecDomain(testDomain).build(),
				claimsReconciler: &mockComponentsReconciler{
					mockReconcile: func(ctx context.Context, reconcilers []resourceReconciler) (reconcile.Result, error) {
						return reconcileWait, nil
					},
				},
			},
			want: want{
				res: reconcileWait,
				err: nil,
			},
		},
		"FailureReconcilingApplications": {
			fields: fields{
				gitlab: newGitLabBuilder().withMeta(testMeta).withSpecDomain(testDomain).build(),
				claimsReconciler: &mockComponentsReconciler{
					mockReconcile: func(ctx context.Context, reconcilers []resourceReconciler) (reconcile.Result, error) {
						return reconcileSuccess, nil
					},
				},
				appsReconciler: &mockComponentsReconciler{
					mockReconcile: func(ctx context.Context, reconcilers []resourceReconciler) (reconcile.Result, error) {
						return reconcileFailure, testError
					},
				},
			},
			want: want{
				res: reconcileFailure,
				err: testError,
			},
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			r := &gitLabReconciler{
				handle: &handle{
					GitLab: tt.fields.gitlab,
				},
				resourceClaimsReconciler: tt.fields.claimsReconciler,
				applicationReconciler:    tt.fields.appsReconciler,
			}
			got, err := r.reconcile(ctx)
			if diff := cmp.Diff(err, tt.want.err, cmpErrors); diff != "" {
				t.Errorf("gitLabReconciler.reconcile() error  %s", diff)
				return
			}
			if diff := cmp.Diff(got, tt.want.res); diff != "" {
				t.Errorf("gitLabReconciler.reconcile() %s", diff)
			}
			if tt.fields.gitlab.Status.Endpoint == "" {
				t.Errorf("gitLabReconciler.reconcile() endpoint is emtpy")
			}
		})
	}
}

func Test_newGitLabReconciler(t *testing.T) {
	type args struct {
		gitlab *v1alpha1.GitLab
		client client.Client
	}
	tests := map[string]struct {
		args args
	}{
		"Default": {
			args: args{
				gitlab: newGitLabBuilder().build(),
				client: test.NewMockClient(),
			},
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			newGitLabReconciler(tt.args.gitlab, tt.args.client)
		})
	}
}

func TestHasSameController(t *testing.T) {
	ctrl := &v1alpha1.GitLab{ObjectMeta: metav1.ObjectMeta{Name: "coolController"}}
	ref := metav1.NewControllerRef(ctrl, v1alpha1.GitLabGroupVersionKind)

	cases := []struct {
		name string
		a    metav1.Object
		b    metav1.Object
		want bool
	}{
		{
			name: "SameController",
			a: &v1alpha1.GitLab{
				ObjectMeta: metav1.ObjectMeta{
					OwnerReferences: []metav1.OwnerReference{*ref},
				},
			},
			b: &v1alpha1.GitLab{
				ObjectMeta: metav1.ObjectMeta{
					OwnerReferences: []metav1.OwnerReference{*ref},
				},
			},
			want: true,
		},
		{
			name: "AHasNoController",
			a:    &v1alpha1.GitLab{},
			b: &v1alpha1.GitLab{
				ObjectMeta: metav1.ObjectMeta{
					OwnerReferences: []metav1.OwnerReference{*ref},
				},
			},
			want: false,
		},
		{
			name: "BHasNoController",
			a: &v1alpha1.GitLab{
				ObjectMeta: metav1.ObjectMeta{
					OwnerReferences: []metav1.OwnerReference{*ref},
				},
			},
			b:    &v1alpha1.GitLab{},
			want: false,
		},
		{
			name: "ControllersDiffer",
			a: &v1alpha1.GitLab{
				ObjectMeta: metav1.ObjectMeta{
					OwnerReferences: []metav1.OwnerReference{
						{
							Controller: func() *bool {
								t := true
								return &t
							}(),
							UID: "imdifferent",
						},
					},
				},
			},
			b: &v1alpha1.GitLab{
				ObjectMeta: metav1.ObjectMeta{
					OwnerReferences: []metav1.OwnerReference{*ref},
				},
			},
			want: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := hasSameController(tc.a, tc.b)

			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("hasSameController(...): -want, +got: %s", diff)
			}
		})
	}
}

func TestGetControllerName(t *testing.T) {
	name := "coolController"
	ctrl := &v1alpha1.GitLab{ObjectMeta: metav1.ObjectMeta{Name: name}}
	ref := metav1.NewControllerRef(ctrl, v1alpha1.GitLabGroupVersionKind)

	cases := []struct {
		name string
		obj  metav1.Object
		want string
	}{
		{
			name: "HasController",
			obj: &v1alpha1.GitLab{
				ObjectMeta: metav1.ObjectMeta{
					OwnerReferences: []metav1.OwnerReference{*ref},
				},
			},
			want: name,
		},
		{
			name: "HasNoController",
			obj:  &v1alpha1.GitLab{},
			want: "",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := getControllerName(tc.obj)

			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("getControllerName(...): -want, +got: %s", diff)
			}
		})
	}

}

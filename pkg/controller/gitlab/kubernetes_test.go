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

	xpcomputev1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/compute/v1alpha1"
	xpcorev1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/core/v1alpha1"
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

func Test_kubernetesReconciler_reconcile(t *testing.T) {
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
		"SuccessfulWithClusterRef": {
			fields: fields{
				base: &baseResourceReconciler{
					GitLab: newGitLabBuilder().withSpecClusterRef(&corev1.ObjectReference{Name: testName, Namespace: testNamespace}).build(),
					client: &test.MockClient{
						MockGet: func(ctx context.Context, key client.ObjectKey, obj runtime.Object) error {
							o, ok := obj.(*xpcomputev1alpha1.KubernetesCluster)
							if !ok {
								t.Errorf("kubernetesReconciler.reconcile() unepxected type %T", obj)
							}
							if diff := cmp.Diff(key, testKey); diff != "" {
								t.Errorf("kubernetesReconciler.reconcile() unepxected key %s", diff)
							}
							o.Status.SetCreating()
							return nil
						},
					},
				},
			},
			want: want{
				status: newResourceClaimStatusBuilder().withCreatingStatus().build(),
			},
		},
		"FailureWithClusterRef": {
			fields: fields{
				base: &baseResourceReconciler{
					GitLab: newGitLabBuilder().withSpecClusterRef(&corev1.ObjectReference{Name: testName, Namespace: testNamespace}).build(),
					client: &test.MockClient{
						MockGet: func(ctx context.Context, key client.ObjectKey, obj runtime.Object) error {
							_, ok := obj.(*xpcomputev1alpha1.KubernetesCluster)
							if !ok {
								t.Errorf("kubernetesReconciler.reconcile() unepxected type %T", obj)
							}
							if diff := cmp.Diff(key, testKey); diff != "" {
								t.Errorf("kubernetesReconciler.reconcile() unepxected key %s", diff)
							}
							return testError
						},
					},
				},
			},
			want: want{err: errors.Wrapf(testError, errorFmtFailedToRetrieveInstance, kubernetesClaimKind, testKey)},
		},
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
			want: want{
				err: errors.Wrapf(testError, errorFmtFailedToFindResourceClass, kubernetesClaimKind, newGitLabBuilder().build().GetProviderRef()),
			},
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
			want: want{err: errors.Wrapf(testError, errorFmtFailedToCreate, kubernetesClaimKind, testKey)},
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
			want: want{err: errors.Wrapf(testError, errorFmtFailedToRetrieveInstance, kubernetesClaimKind, testKey)},
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
							o, ok := obj.(*xpcomputev1alpha1.KubernetesCluster)
							if !ok {
								return errors.Errorf("kubernetesReconciler.reconcile() type: %T", obj)
							}
							o.Status = *newResourceClaimStatusBuilder().withReadyStatus().build()
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
			r := &kubernetesReconciler{
				baseResourceReconciler: tt.fields.base,
				resourceClassFinder:    tt.fields.finder,
			}
			if diff := cmp.Diff(r.reconcile(ctx), tt.want.err, cmpErrors); diff != "" {
				t.Errorf("kubernetesReconciler.reconcile() error %s", diff)
			}
			if diff := cmp.Diff(r.status, tt.want.status, cmp.Comparer(util.EqualConditionedStatus)); diff != "" {
				t.Errorf("kubernetesReconciler.reconcile() status %s", diff)
			}
		})
	}
}

func Test_kubernetesReconciler_getClaimKind(t *testing.T) {
	r := &kubernetesReconciler{}
	if diff := cmp.Diff(r.getClaimKind(), kubernetesClaimKind); diff != "" {
		t.Errorf("kubernetesReconciler.getClaimKind() %s", diff)
	}
}

func Test_newKubernetesReconciler(t *testing.T) {
	gitlab := &v1alpha1.GitLab{}
	clnt := test.NewMockClient()

	r := newKubernetesReconciler(gitlab, clnt)
	if diff := cmp.Diff(r.GitLab, gitlab); diff != "" {
		t.Errorf("newRedisReconciler() GitLab %s", diff)
	}
}

func Test_kubernetesReconciler_getHelmValues(t *testing.T) {
	type fields struct {
		baseResourceReconciler *baseResourceReconciler
		resourceClassFinder    resourceClassFinder
	}
	type args struct {
		ctx    context.Context
		values map[string]string
	}
	tests := map[string]struct {
		fields fields
		args   args
	}{
		"Default": {
			fields: fields{
				baseResourceReconciler: newBaseResourceReconciler(newGitLabBuilder().build(), test.NewMockClient(), "foo"),
			},
			args: args{
				ctx:    context.TODO(),
				values: make(map[string]string),
			},
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			r := &kubernetesReconciler{
				baseResourceReconciler: tt.fields.baseResourceReconciler,
				resourceClassFinder:    tt.fields.resourceClassFinder,
			}
			if err := r.getHelmValues(tt.args.ctx, tt.args.values); err != nil {
				t.Errorf("kubernetesReconciler.getHelmValues() error = %v", err)
			}
		})
	}
}

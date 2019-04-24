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
	"strings"
	"time"

	xpcorev1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/core/v1alpha1"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/crossplaneio/gitlab-controller/pkg/apis/controller/v1alpha1"
	"github.com/crossplaneio/gitlab-controller/pkg/logging"
)

const (
	requeueAfterOnSuccess = 1 * time.Minute
	requeueAfterOnWait    = 20 * time.Second

	postgresEngineVersion = "9.6"
	redisEngineVersion    = "3.2"

	resourceAnnotationKey = "resource"

	errorFmtFailedToListResourceClasses        = "failed to list resource classes: [%s/%s, %s]"
	errorFmtResourceClassNotFound              = "resource class not found for provider: [%s/%s, %s]"
	errorFmtNotSupportedProvider               = "not supported provider: %s"
	errorFmtFailedToRetrieveConnectionSecret   = "failed to retrieve connection secret: %s"
	errorFmtFailedToUpdateConnectionSecret     = "failed to update connection secret: %s"
	errorFmtFailedToUpdateConnectionSecretData = "failed to update connection secret data: %s"
	errorFmtFailedToFindResourceClass          = "failed to find %s resource class: %+v"
	errorFmtFailedToRetrieveInstance           = "failed to retrieve %s instance: %s"
	errorFmtFailedToCreate                     = "failed to create %s: %s"

	errorResourceStatusIsNotFound = "resource status is not found"

	reasonResourceProcessingFailure = "fail to process resource"
	reasonHasFailedResources        = "has failed resourceClaims"
	reasonHasPendingResources       = "has pending resourceClaims"
)

var (
	reconcileSuccess = reconcile.Result{RequeueAfter: requeueAfterOnSuccess}
	reconcileWait    = reconcile.Result{RequeueAfter: requeueAfterOnWait}
	reconcileFailure = reconcile.Result{Requeue: true}
)

// resourceReconciler interface provides abstract operations supported by all component reconciles
type resourceReconciler interface {
	// reconcile a given component
	reconcile(context.Context) error
	// isReady() when given component status is ready
	isReady() bool
	// isFailed() when given component status is failed
	isFailed() bool
	// getClaimKind returns a component kind/type so we can loosely identify it in the status list
	getClaimKind() string
	// getClaimConnectionSecret
	getClaimConnectionSecret(context.Context) (*corev1.Secret, error)
}

// resourceClassFinder interface
type resourceClassFinder interface {
	find(ctx context.Context, provider corev1.ObjectReference, resource string) (*corev1.ObjectReference, error)
}

// base provides base operations needed during typical component reconciliation
type baseResourceReconciler struct {
	*v1alpha1.GitLab
	client client.Client
	status *xpcorev1alpha1.ResourceClaimStatus
}

// isReady when ready
func (r *baseResourceReconciler) isReady() bool {
	return r.status != nil && r.status.IsReady()
}

// isFailed when failed
func (r *baseResourceReconciler) isFailed() bool {
	return r.status != nil && r.status.IsFailed()
}

func (r *baseResourceReconciler) getClaimConnectionSecret(ctx context.Context) (*corev1.Secret, error) {
	secret := &corev1.Secret{}
	key := types.NamespacedName{Namespace: r.GetNamespace(), Name: r.status.CredentialsSecretRef.Name}
	if err := r.client.Get(ctx, key, secret); err != nil {
		return nil, errors.Wrapf(err, errorFmtFailedToRetrieveConnectionSecret, key)
	}
	return secret, nil
}

// find returns resource class object reference base on provider and resource values
// Note: if provider is not found, nil value is returned w/out error
func (r *baseResourceReconciler) find(ctx context.Context, provider corev1.ObjectReference, resource string) (*corev1.ObjectReference, error) {
	rcs := &xpcorev1alpha1.ResourceClassList{}
	opts := &client.ListOptions{Namespace: provider.Namespace}
	if err := r.client.List(ctx, opts, rcs); err != nil {
		return nil, errors.Wrapf(err, errorFmtFailedToListResourceClasses, provider.Namespace, provider.Name, resource)
	}
	for _, rc := range rcs.Items {
		if rc.ProviderRef.Name == provider.Name && rc.Annotations[resourceAnnotationKey] == resource {
			return &corev1.ObjectReference{
				Namespace: rc.Namespace,
				Name:      rc.Name,
			}, nil
		}
	}
	return nil, errors.Errorf(errorFmtResourceClassNotFound, provider.Namespace, provider.Name, resource)
}

// newObjectMeta helper function to create a meta object for all components
func (r *baseResourceReconciler) newObjectMeta(nameSuffixes ...string) metav1.ObjectMeta {
	return metav1.ObjectMeta{
		Namespace:       r.GetNamespace(),
		Name:            strings.Join(append(append([]string{}, r.GetName()), nameSuffixes...), "-"),
		Labels:          map[string]string{"app": r.GetName()},
		OwnerReferences: []metav1.OwnerReference{r.ToOwnerReference()},
	}
}

// newKey helper function
func (r *baseResourceReconciler) newNamespacedName(nameSuffixes ...string) types.NamespacedName {
	return types.NamespacedName{
		Namespace: r.GetNamespace(),
		Name:      strings.Join(append(append([]string{}, r.GetName()), nameSuffixes...), "-"),
	}
}

// newBaseComponentReconciler returns a new instance of the component reconciler
func newBaseComponentReconciler(gitlab *v1alpha1.GitLab, client client.Client) *baseResourceReconciler {
	return &baseResourceReconciler{
		GitLab: gitlab,
		client: client,
	}
}

// gitLabReconciler
type gitLabReconciler struct {
	*v1alpha1.GitLab
	client         client.Client
	resourceClaims []resourceReconciler
}

const (
//reasonConnectionSecretRetrievalFailure = "failed to retrieve claim connection secret"
//reasonFailedToConvertConnectionSecret  = "failed to convert connection secret to template"
)

// fail helper function combines setting resource failed condition and updating status
func (r *gitLabReconciler) fail(ctx context.Context, reason, msg string) error {
	log.Info("reconciliation failure", "reason", reason, "msg", msg)
	r.SetFailed(reason, msg)
	return r.update(ctx)
}

func (r *gitLabReconciler) pending(ctx context.Context, reason, msg string) error {
	r.SetPending(reason, msg)
	return r.update(ctx)
}

func (r *gitLabReconciler) update(ctx context.Context) error {
	return r.client.Status().Update(ctx, r.GitLab)
}

func (r *gitLabReconciler) reconcile(ctx context.Context) (reconcile.Result, error) {
	r.SetEndpoint(r.GetEndpoint())

	log.V(logging.Debug).Info("reconciling resource claims")
	res, err := r.reconcileClaims(ctx, r.resourceClaims)
	if res != reconcileSuccess {
		log.Error(err, "claim reconciliation failed")
		return res, err
	}

	log.V(logging.Debug).Info("reconciling applications")
	return r.reconcileApplication(ctx, r.resourceClaims)
}

func (r *gitLabReconciler) reconcileClaims(ctx context.Context, claims []resourceReconciler) (reconcile.Result, error) {
	var failed []string
	var pending []string

	// reconcile GitLab resourceClaims
	for _, res := range claims {
		if err := res.reconcile(ctx); err != nil {
			return reconcileFailure, r.fail(ctx, reasonResourceProcessingFailure, err.Error())
		}
		if res.isFailed() {
			failed = append(failed, res.getClaimKind())
		} else if !res.isReady() {
			pending = append(pending, res.getClaimKind())
		}
	}
	if len(failed) > 0 {
		return reconcileFailure, r.fail(ctx, reasonHasFailedResources, strings.Join(failed, ","))
	}
	if len(pending) > 0 {
		return reconcileWait, r.pending(ctx, reasonHasPendingResources, strings.Join(pending, ","))
	}

	return reconcileSuccess, nil
}

func (r *gitLabReconciler) reconcileApplication(ctx context.Context, resources []resourceReconciler) (reconcile.Result, error) {
	for _, rs := range resources {
		log.V(logging.Debug).Info(rs.getClaimKind(), "ready", rs.isReady())
	}
	r.SetReady()
	return reconcileSuccess, r.update(ctx)
}

func newGitLabReconciler(gitlab *v1alpha1.GitLab, client client.Client) *gitLabReconciler {
	return &gitLabReconciler{
		GitLab: gitlab,
		client: client,
		resourceClaims: []resourceReconciler{
			newKubernetesReconciler(gitlab, client),
			newPostgresReconciler(gitlab, client),
			newRedisReconciler(gitlab, client),
			newBucketReconciler(gitlab, client, "artifacts"),
			newBucketReconciler(gitlab, client, "backups"),
			newBucketReconciler(gitlab, client, "backups-tmp"),
			newBucketReconciler(gitlab, client, "externaldiffs"),
			newBucketReconciler(gitlab, client, "lfs"),
			newBucketReconciler(gitlab, client, "packages"),
			newBucketReconciler(gitlab, client, "pseudonymizer"),
			newBucketReconciler(gitlab, client, "registry"),
			newBucketReconciler(gitlab, client, "uploads"),
		},
	}
}

var _ reconciler = &gitLabReconciler{}

// reconciler
type reconciler interface {
	reconcile(context.Context) (reconcile.Result, error)
}

// reconcilerMill
type reconcilerMill interface {
	newReconciler(*v1alpha1.GitLab, client.Client) reconciler
}

// gitLabReconcilerMill
type gitLabReconcilerMill struct{}

func (m *gitLabReconcilerMill) newReconciler(gitlab *v1alpha1.GitLab, client client.Client) reconciler {
	return newGitLabReconciler(gitlab, client)
}

var _ reconcilerMill = &gitLabReconcilerMill{}

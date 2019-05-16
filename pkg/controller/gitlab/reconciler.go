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
	"strings"
	"time"

	xpcorev1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/core/v1alpha1"
	xpworkloadv1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/workload/v1alpha1"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/helm/pkg/chartutil"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/crossplaneio/gitlab-controller/pkg/apis/controller/v1alpha1"
	"github.com/crossplaneio/gitlab-controller/pkg/controller/gitlab/application"
	"github.com/crossplaneio/gitlab-controller/pkg/controller/gitlab/resource/helm"
	"github.com/crossplaneio/gitlab-controller/pkg/crud"
	"github.com/crossplaneio/gitlab-controller/pkg/logging"
)

const (
	requeueAfterOnSuccess = 1 * time.Minute
	requeueAfterOnWait    = 20 * time.Second

	postgresEngineVersion = "9.6"
	redisEngineVersion    = "3.2"

	resourceAnnotationKey = "resource"

	// delimiter to use when creating composite names for object metadata
	errorFmtFailedToListResourceClasses        = "failed to list resource classes: [%s/%s, %s]"
	errorFmtResourceClassNotFound              = "resource class not found for provider: [%s/%s, %s]"
	errorFmtNotSupportedProvider               = "not supported provider: %s"
	errorFmtFailedToRetrieveConnectionSecret   = "failed to retrieve connection secret: %s"
	errorFmtFailedToUpdateConnectionSecret     = "failed to update connection secret: %s"
	errorFmtFailedToUpdateConnectionSecretData = "failed to update connection secret data: %s"
	errorFmtFailedToFindResourceClass          = "failed to find %s resource class: %+v"
	errorFmtFailedToRetrieveInstance           = "failed to retrieve %s instance: %s"
	errorFmtFailedToCreate                     = "failed to create %s: %s"
	errorFmtFailedToMergeHelmValues            = "failed to produce Helm values for %s"

	errorFailedToRenderHelmChart  = "failed to render Helm chart"
	errorResourceStatusIsNotFound = "resource status is not found"

	reasonResourceProcessingFailure  = "fail to process resource"
	reasonHasFailedResources         = "has failed resourceClaims"
	reasonHasPendingResources        = "has pending resourceClaims"
	reasonFailedToGenerateHelmValues = "failed to generate Helm values"
	reasonProducingResources         = "cannot produce resources"
	reasonSyncingApplication         = "cannot sync application"

	gitlabChartURL = "https://gitlab-charts.s3.amazonaws.com/gitlab-1.8.4.tgz"

	valuesKeyGlobal      = "global"
	valuesKeyAppConfig   = "appConfig"
	valuesKeyGitlab      = "gitlab"
	valuesKeyCertmanager = "certmanager-issuer"
	valuesKeyMinio       = "minio"
	valuesKeyRedis       = "redis"
	valuesKeyPostgres    = "postgresql" // Postgres is configured at .postgres
	valuesKeyPSQL        = "psql"       // ...and at .global.psql
	valuesKeyPrometheus  = "prometheus"
	valuesKeyRunner      = "gitlab-runner"
)

var (
	reconcileSuccess = reconcile.Result{RequeueAfter: requeueAfterOnSuccess}
	reconcileWait    = reconcile.Result{RequeueAfter: requeueAfterOnWait}
	reconcileFailure = reconcile.Result{Requeue: true}
)

// resourceReconciler interface provides abstract operations supported by all component reconciles
type resourceReconciler interface {
	// reconcile a given component
	reconcile(ctx context.Context) error
	// isReady() when given component status is ready
	isReady() bool
	// isFailed() when given component status is failed
	isFailed() bool
	// getClaimKind returns a component kind/type so we can loosely identify it in the status list
	getClaimKind() string
	// getClaimConnectionSecretName returns the claim's connection secret name.
	getClaimConnectionSecretName() string
	// getClaimConnectionSecret
	getClaimConnectionSecret(ctx context.Context) (*corev1.Secret, error)
	// getHelmValues populates the supplied Values with this resource's values.
	getHelmValues(ctx context.Context, dst chartutil.Values, secretPrefix string) error
}

// resourceClassFinder interface
type resourceClassFinder interface {
	find(ctx context.Context, provider corev1.ObjectReference, resource string) (*corev1.ObjectReference, error)
}

// base provides base operations needed during typical component reconciliation
type baseResourceReconciler struct {
	*v1alpha1.GitLab
	client        client.Client
	status        *xpcorev1alpha1.ResourceClaimStatus
	componentName string
}

// isReady when ready
func (r *baseResourceReconciler) isReady() bool {
	return r.status != nil && r.status.IsReady()
}

// isFailed when failed
func (r *baseResourceReconciler) isFailed() bool {
	return r.status != nil && r.status.IsFailed()
}

func (r *baseResourceReconciler) getClaimConnectionSecretName() string {
	return r.status.CredentialsSecretRef.Name
}

func (r *baseResourceReconciler) getClaimConnectionSecret(ctx context.Context) (*corev1.Secret, error) {
	secret := &corev1.Secret{}
	key := types.NamespacedName{Namespace: r.GetNamespace(), Name: r.getClaimConnectionSecretName()}
	if err := r.client.Get(ctx, key, secret); err != nil {
		return nil, errors.Wrapf(err, errorFmtFailedToRetrieveConnectionSecret, key)
	}
	return secret, nil
}

// find returns resource class object reference base on provider and resource values
// Note: if provider is not found, nil value is returned w/out error
func (r *baseResourceReconciler) find(ctx context.Context, provider corev1.ObjectReference,
	resource string) (*corev1.ObjectReference, error) {
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
		Name:            strings.Join(append([]string{r.GetName()}, nameSuffixes...), "-"),
		Labels:          map[string]string{"app": r.GetName()},
		OwnerReferences: []metav1.OwnerReference{r.ToOwnerReference()},
	}
}

func (r *baseResourceReconciler) loadHelmValues(ctx context.Context, dst chartutil.Values, fn helmValuesFunction,
	secretPrefix string) error {
	if r.status == nil {
		return errors.New(errorResourceStatusIsNotFound)
	}

	s := &corev1.Secret{}
	key := types.NamespacedName{Namespace: r.GetNamespace(), Name: r.status.CredentialsSecretRef.Name}
	if err := r.client.Get(ctx, key, s); err != nil {
		return errors.Wrapf(err, errorFmtFailedToRetrieveConnectionSecret, key)
	}

	return errors.Wrapf(fn(dst, s, r.componentName, secretPrefix), errorFmtFailedToMergeHelmValues, r.GetName())
}

// newBaseResourceReconciler returns a new instance of the component reconciler
func newBaseResourceReconciler(gitlab *v1alpha1.GitLab, client client.Client, name string) *baseResourceReconciler {
	return &baseResourceReconciler{
		GitLab:        gitlab,
		client:        client,
		componentName: name,
	}
}

// handle
type handle struct {
	*v1alpha1.GitLab
	client client.Client
}

// fail helper function combines setting resource failed condition and updating status
func (h *handle) fail(ctx context.Context, reason, msg string) error {
	log.Info("reconciliation failure", "reason", reason, "msg", msg)
	h.SetFailed(reason, msg)
	return h.update(ctx)
}

func (h *handle) pending(ctx context.Context, reason, msg string) error {
	h.SetPending(reason, msg)
	return h.update(ctx)
}

func (h *handle) update(ctx context.Context) error {
	return h.client.Status().Update(ctx, h.GitLab)
}

// component reconciler
type componentsReconciler interface {
	reconcile(context.Context, []resourceReconciler) (reconcile.Result, error)
}

//
type resourceClaimsReconciler struct {
	*handle
}

var _ componentsReconciler = &resourceClaimsReconciler{}

func (r *resourceClaimsReconciler) reconcile(ctx context.Context, claims []resourceReconciler) (reconcile.Result, error) {
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

type resourceProducer interface {
	produce(v chartutil.Values) ([]*unstructured.Unstructured, error)
}

type helmResourceProducer struct {
	chartURL string
}

func (p *helmResourceProducer) produce(v chartutil.Values) ([]*unstructured.Unstructured, error) {
	templates, err := helm.Render(p.chartURL, helm.WithValues(v))
	return templates, errors.Wrap(err, errorFailedToRenderHelmChart)
}

type applicationProducer interface {
	produce(ctrl *v1alpha1.GitLab, resources []*unstructured.Unstructured, m application.SecretMap) *xpworkloadv1alpha1.KubernetesApplication
}

type helmApplicationProducer struct{}

func (p *helmApplicationProducer) produce(ctrl *v1alpha1.GitLab, resources []*unstructured.Unstructured, m application.SecretMap) *xpworkloadv1alpha1.KubernetesApplication {
	// TODO(negz): Provide a cluster selector?
	// TODO(negz): Override template namespaces, if necessary?
	return application.New(ctrl.GetName(), resources,
		application.WithSecretMap(m),
		application.WithNamespace(ctrl.GetNamespace()),
		application.WithLabels(ctrl.GetLabels()),
		application.WithControllerReference(metav1.NewControllerRef(ctrl, v1alpha1.GitLabGroupVersionKind)))
}

type applicationReconciler struct {
	*handle

	resources   resourceProducer
	application applicationProducer
}

func (a *applicationReconciler) getHelmValues(ctx context.Context, rr []resourceReconciler,
	secretPrefix string) (chartutil.Values, error) {
	v := chartutil.Values{
		valuesKeyGlobal: chartutil.Values{
			valuesKeyMinio: chartutil.Values{"enabled": false},
			"hosts":        chartutil.Values{"domain": a.Spec.Domain},
		},
		valuesKeyGitlab: chartutil.Values{
			"unicorn": chartutil.Values{
				"helmTests": chartutil.Values{"enabled": false},
			},
		},
		valuesKeyRunner: chartutil.Values{
			"runners": chartutil.Values{
				// Disable the runner cache.
				"cache": chartutil.Values{},
			},
		},
		valuesKeyPostgres:    chartutil.Values{"install": false},
		valuesKeyRedis:       chartutil.Values{"enabled": false},
		valuesKeyPrometheus:  chartutil.Values{"install": false},
		valuesKeyCertmanager: chartutil.Values{"email": a.Spec.Email},
	}

	for _, claim := range rr {
		if err := claim.getHelmValues(ctx, v, secretPrefix); err != nil {
			return nil, errors.Wrapf(err, errorFmtFailedToMergeHelmValues, v1alpha1.GitLabKind)
		}
	}
	return v, nil
}

func (a *applicationReconciler) getConnectionSecretNames(rr []resourceReconciler) []string {
	names := make([]string, len(rr))
	for i := range rr {
		names[i] = rr[i].getClaimConnectionSecretName()
	}
	return names
}

func (a *applicationReconciler) reconcile(ctx context.Context, rr []resourceReconciler) (reconcile.Result, error) {
	// This is a hack. KubernetesApplications expect connection secrets to be
	// associated with the KubernetesApplicationResource templates that consume
	// them. A secret named 'secretname' in the Crossplane API server will be
	// propagated to the KubernetesCluster where the KubernetesApplication runs
	// as 'templatenamespace/KARname-secretname'.
	//
	// We use Helm to render our KubernetesApplication. The Gitlab chart must be
	// told where to load secrets in order to render the KubernetesApplication,
	// so we have a chicken-and-egg problem; we can't render an app until we
	// know what our connection secrets will be named and we can't know what our
	// connection secrets will be named until we render the app. We work around
	// this by associating all of our connection secrets with this empty secret,
	// allowing us to reliably predict propagated secret names.
	s := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "v1",
		"kind":       "Secret",
		"metadata":   map[string]interface{}{"name": "crossplane"},
	}}
	secretPrefix := fmt.Sprintf("%s-secret-%s-", a.GetName(), s.GetName())
	secretNames := a.getConnectionSecretNames(rr)
	secretMap := application.SecretMap{}
	secretMap.Add(s, secretNames...)

	v, err := a.getHelmValues(ctx, rr, secretPrefix)
	if err != nil {
		return reconcileFailure, a.fail(ctx, reasonFailedToGenerateHelmValues, err.Error())
	}

	// TODO(negz): Don't download the chart on every reconcile loop.
	resources, err := a.resources.produce(v)
	if err != nil {
		return reconcileFailure, a.fail(ctx, reasonProducingResources, err.Error())
	}
	resources = append([]*unstructured.Unstructured{s}, resources...)

	app := a.application.produce(a.handle.GitLab, resources, secretMap)

	remote := app.DeepCopy()
	if err := crud.CreateOrUpdate(ctx, a.client, remote, func() error {
		// Inside this anonymous function remote could either be unchanged (if
		// it does not exist in the API server) or updated to reflect its
		// current state according to the API server.

		if !hasSameController(remote, app) {
			return errors.Errorf("%s %s exists and is not controlled by %s %s",
				xpworkloadv1alpha1.KubernetesApplicationKind, remote.GetName(),
				v1alpha1.GitLabKind, getControllerName(app))
		}

		remote.SetLabels(app.GetLabels())
		remote.Spec = *app.Spec.DeepCopy()

		return nil
	}); err != nil {
		return reconcileFailure, a.fail(ctx, reasonSyncingApplication, err.Error())
	}

	// TODO(negz): Derive readiness from application state.
	a.SetReady()
	return reconcileSuccess, a.update(ctx)
}

func hasSameController(a, b metav1.Object) bool {
	ac := metav1.GetControllerOf(a)
	bc := metav1.GetControllerOf(b)

	// We do not consider two objects without any controller to have
	// the same controller.
	if ac == nil || bc == nil {
		return false
	}

	return ac.UID == bc.UID
}

func getControllerName(obj metav1.Object) string {
	c := metav1.GetControllerOf(obj)
	if c == nil {
		return ""
	}

	return c.Name
}

// gitLabReconciler
type gitLabReconciler struct {
	*handle
	resourceClaims           []resourceReconciler
	resourceClaimsReconciler componentsReconciler
	applicationReconciler    componentsReconciler
}

func (r *gitLabReconciler) reconcile(ctx context.Context) (reconcile.Result, error) {
	r.SetEndpoint(r.GetEndpoint())

	log.V(logging.Debug).Info("reconciling resource claims")
	res, err := r.resourceClaimsReconciler.reconcile(ctx, r.resourceClaims)
	if err != nil {
		log.Error(err, "claim reconciliation failed")
		return res, err
	}

	switch res {
	case reconcileFailure:
		log.Info("one or more claims have failed status", "reconcile action", "rerun")
		return res, err
	case reconcileWait:
		log.Info("one or more claims have pending status", "reconcile action", "wait")
		return res, err
	}

	log.V(logging.Debug).Info("reconciling applications")
	return r.applicationReconciler.reconcile(ctx, r.resourceClaims)
}

func newGitLabReconciler(gitlab *v1alpha1.GitLab, client client.Client) *gitLabReconciler {
	h := &handle{
		GitLab: gitlab,
		client: client,
	}
	return &gitLabReconciler{
		handle: h,
		resourceClaims: []resourceReconciler{
			newKubernetesReconciler(gitlab, client),
			newPostgresReconciler(gitlab, client),
			newRedisReconciler(gitlab, client),
			newBucketReconciler(gitlab, client, "artifacts", bucketConnectionHelmValues),
			newBucketReconciler(gitlab, client, "backups", bucketBackupsHelmValues),
			newBucketReconciler(gitlab, client, "backups-tmp", bucketBackupsTempHelmValues),
			newBucketReconciler(gitlab, client, "externaldiffs", bucketConnectionHelmValues),
			newBucketReconciler(gitlab, client, "lfs", bucketConnectionHelmValues),
			newBucketReconciler(gitlab, client, "packages", bucketConnectionHelmValues),
			newBucketReconciler(gitlab, client, "pseudonymizer", bucketConnectionHelmValues),
			newBucketReconciler(gitlab, client, "registry", bucketConnectionHelmValues),
			newBucketReconciler(gitlab, client, "uploads", bucketConnectionHelmValues),
		},
		resourceClaimsReconciler: &resourceClaimsReconciler{handle: h},
		applicationReconciler: &applicationReconciler{
			handle:      h,
			resources:   &helmResourceProducer{chartURL: gitlabChartURL},
			application: &helmApplicationProducer{},
		},
	}
}

type helmValuesFunction func(dst chartutil.Values, s *corev1.Secret, propagatedSecretPrefix, bucketName string) error

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

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
	"hash/fnv"
	"strings"
	"time"

	xpawsv1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/aws/v1alpha1"
	xpcorev1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/core/v1alpha1"
	xpgcpv1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/gcp/v1alpha1"
	xpworkloadv1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/workload/v1alpha1"
	"github.com/davecgh/go-spew/spew"
	"github.com/pkg/errors"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/json"
	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/helm/pkg/chartutil"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/crossplaneio/gitlab-controller/pkg/apis/controller/v1alpha1"
	"github.com/crossplaneio/gitlab-controller/pkg/controller/gitlab/application"
	"github.com/crossplaneio/gitlab-controller/pkg/controller/gitlab/resource/helm"
	"github.com/crossplaneio/gitlab-controller/pkg/controller/gitlab/values"
	"github.com/crossplaneio/gitlab-controller/pkg/crud"
	"github.com/crossplaneio/gitlab-controller/pkg/logging"
)

const (
	requeueAfterOnSuccess = 1 * time.Minute
	requeueAfterOnWait    = 20 * time.Second

	postgresEngineVersion = "9.6"
	redisEngineVersion    = "3.2"

	// TODO(negz): Prefix this annotation.
	annotationResource   = "resource"
	annotationGitlabHash = v1alpha1.Group + "/hash"
	annotationChartURL   = v1alpha1.Group + "/charturl"

	// delimiter to use when creating composite names for object metadata
	errorFmtFailedToListResourceClasses        = "failed to list resource classes: [%s/%s, %s]"
	errorFmtResourceClassNotFound              = "resource class not found for provider: [%s/%s, %s]"
	errorFmtNotSupportedProvider               = "not supported provider: %s"
	errorFmtFailedToRetrieveConnectionSecret   = "failed to retrieve connection secret: %s"
	errorFmtFailedToUpdateConnectionSecret     = "failed to update connection secret: %s"
	errorFmtFailedToUpdateConnectionSecretData = "failed to update connection secret data: %s"
	errorFmtFailedToFindResourceClass          = "failed to find %s resource class: %+v"
	errorFmtFailedToRetrieveInstance           = "failed to retrieve %s instance: %s"
	errorFmtFailedToGet                        = "failed to get %s"
	errorFmtFailedToCreate                     = "failed to create %s: %s"
	errorFmtFailedToMergeHelmValues            = "failed to produce Helm values for %s"
	errorFmtCannotGetProvider                  = "cannot get provider %s"

	errorResourceStatusIsNotFound          = "resource status is not found"
	errorFailedToDetermineReconcileNeed    = "failed to determine whether reconcile was needed"
	errorProducingExternalDNSResources     = "cannot produce resources"
	errorSyncingExternalDNS                = "cannot sync External DNS"
	errorFailedToGenerateExternalDNSValues = "cannot generate External DNS values"
	errorUnsupportedProvider               = "unsupported provider"

	reasonResourceProcessingFailure  = "fail to process resource"
	reasonHasFailedResources         = "has failed resourceClaims"
	reasonHasPendingResources        = "has pending resourceClaims"
	reasonFailedToGenerateHelmValues = "failed to generate Helm values"
	reasonProducingResources         = "cannot produce resources"
	reasonSyncingGitlab              = "cannot sync Gitlab application"
	reasonSyncingExternalDNS         = "cannot sync External DNS application"

	gitlabChartURL       = "https://gitlab-charts.s3.amazonaws.com/gitlab-1.8.4.tgz"
	externalDNSChartURL  = "https://kubernetes-charts.storage.googleapis.com/external-dns-1.7.5.tgz"
	externalDNSAppSuffix = "-externaldns"

	unicornResourceSuffix = "-deployment-gitlab-unicorn"

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
	// getClaimRef returns a reference to this reconciler's resource claim.
	getClaimRef() *corev1.ObjectReference
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
		if rc.ProviderRef.Name == provider.Name && rc.Annotations[annotationResource] == resource {
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

type chartRenderer interface {
	render(chartURL string, o ...helm.Option) (helm.Resources, error)
}

type chartRendererFn func(chartURL string, o ...helm.Option) (helm.Resources, error)

func (fn chartRendererFn) render(chartURL string, o ...helm.Option) (helm.Resources, error) {
	return fn(chartURL, o...)
}

type applicationCreator interface {
	create(name string, ts application.Templates, o ...application.Option) *xpworkloadv1alpha1.KubernetesApplication
}

type applicationCreatorFn func(name string, ts application.Templates, o ...application.Option) *xpworkloadv1alpha1.KubernetesApplication

func (fn applicationCreatorFn) create(name string, ts application.Templates, o ...application.Option) *xpworkloadv1alpha1.KubernetesApplication {
	return fn(name, ts, o...)
}

type applicationReconciler struct {
	*handle

	gitlabChartURL      string
	externalDNSChartURL string

	chart       chartRenderer
	application applicationCreator
}

func defaultValues(gl *v1alpha1.GitLab) chartutil.Values {
	return chartutil.Values{
		valuesKeyGlobal: chartutil.Values{
			valuesKeyMinio: chartutil.Values{"enabled": false},
			"hosts": chartutil.Values{
				"domain":     gl.Spec.Domain,
				"hostSuffix": gl.Spec.HostSuffix,
			},
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
		valuesKeyCertmanager: chartutil.Values{"email": gl.Spec.Email},
	}
}

func (a *applicationReconciler) getConnectionSecretNames(rr []resourceReconciler) []string {
	names := make([]string, len(rr))
	for i := range rr {
		names[i] = rr[i].getClaimConnectionSecretName()
	}
	return names
}

// TODO(negz): Handle collisions. A collision would prevent us from updating our
// managed KubernetesApplication.
func hash(g *v1alpha1.GitLab, v chartutil.Values) string {
	h := fnv.New32a()
	s := &spew.ConfigState{
		Indent:         " ",
		SortKeys:       true,
		DisableMethods: true,
		SpewKeys:       true,
	}

	// Hash writes never return an error.
	fmts := "%#v"
	s.Fprintf(h, fmts, g.GetUID())         // nolint:errcheck
	s.Fprintf(h, fmts, g.GetNamespace())   // nolint:errcheck
	s.Fprintf(h, fmts, g.GetName())        // nolint:errcheck
	s.Fprintf(h, fmts, g.GetAnnotations()) // nolint:errcheck
	s.Fprintf(h, fmts, g.GetLabels())      // nolint:errcheck
	s.Fprintf(h, fmts, g.Spec)             // nolint:errcheck
	s.Fprintf(h, fmts, v)                  // nolint:errcheck

	return rand.SafeEncodeString(fmt.Sprint(h.Sum32()))
}

func (a *applicationReconciler) needsReconcile(ctx context.Context, v chartutil.Values) (bool, error) {
	n := types.NamespacedName{Namespace: a.GetNamespace(), Name: a.GetName()}
	existing := &xpworkloadv1alpha1.KubernetesApplication{}
	if err := a.client.Get(ctx, n, existing); err != nil {
		if kerrors.IsNotFound(err) {
			return true, nil
		}
		return false, errors.Wrapf(err, errorFmtFailedToGet, n)
	}

	if existing.GetAnnotations()[annotationChartURL] != a.gitlabChartURL {
		return true, nil
	}

	return existing.GetAnnotations()[annotationGitlabHash] != hash(a.GitLab, v), nil
}

func (a *applicationReconciler) clusterRef(rr []resourceReconciler) *corev1.ObjectReference {
	for _, claim := range rr {
		if claim.getClaimKind() == kubernetesClaimKind {
			return claim.getClaimRef()
		}
	}
	return nil
}

// TODO(negz): Burn this code to the ground.
// nolint:gocyclo
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

	values := defaultValues(a.GitLab)
	for _, claim := range rr {
		if err := claim.getHelmValues(ctx, values, secretPrefix); err != nil {
			return reconcileFailure, a.fail(ctx, reasonFailedToGenerateHelmValues, err.Error())
		}
	}

	necessary, err := a.needsReconcile(ctx, values)
	if err != nil {
		return reconcileFailure, a.fail(ctx, errorFailedToDetermineReconcileNeed, err.Error())
	}
	if !necessary {
		log.V(logging.Debug).Info("skipping no-op reconciliation",
			"kind", xpworkloadv1alpha1.KubernetesApplicationKind,
			"namespace", a.GetNamespace(),
			"name", a.GetName())
		if a.isReady(ctx) {
			a.SetReady()
			return reconcileSuccess, a.update(ctx)
		}
		return reconcileWait, a.update(ctx)
	}

	resources, err := a.chart.render(a.gitlabChartURL, helm.WithValues(values))
	if err != nil {
		return reconcileFailure, a.fail(ctx, reasonProducingResources, err.Error())
	}
	resources = append(helm.Resources{s}, resources...)

	// TODO(negz): Override template namespaces, if necessary?
	// TODO(negz): Provide a cluster selector instead of a cluster reference.
	cluster := a.clusterRef(rr)
	app := a.application.create(a.GetName(), application.Templates(resources),
		application.WithSecretMap(secretMap),
		application.WithNamespace(a.GetNamespace()),
		application.WithCluster(cluster),
		application.WithControllerReference(metav1.NewControllerRef(a, v1alpha1.GitLabGroupVersionKind)),
		application.WithLabels(a.GetLabels()),
		application.WithAnnotations(map[string]string{
			annotationChartURL:   a.gitlabChartURL,
			annotationGitlabHash: hash(a.GitLab, values),
		}))

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
		return reconcileFailure, a.fail(ctx, reasonSyncingGitlab, err.Error())
	}

	if err := a.reconcileExternalDNS(ctx, cluster); err != nil {
		return reconcileFailure, a.fail(ctx, reasonSyncingExternalDNS, err.Error())
	}

	if a.isReady(ctx) {
		a.SetReady()
		return reconcileSuccess, a.update(ctx)
	}
	return reconcileWait, a.update(ctx)
}

// TODO(negz): Consider more factors than whether the unicorn deployment is
// available, i.e. whether the ingress controller and external DNS deployments
// are ready.
func (a *applicationReconciler) isReady(ctx context.Context) bool {
	unicorn := a.GetName() + unicornResourceSuffix
	r := &xpworkloadv1alpha1.KubernetesApplicationResource{}
	n := types.NamespacedName{Namespace: a.GetNamespace(), Name: unicorn}
	if err := a.client.Get(ctx, n, r); err != nil {
		log.V(logging.Debug).Info("KubernetesApplicationResource not yet ready", "name", unicorn, "error", err)
		return false
	}

	if r.Status.Remote == nil {
		return false
	}

	s := &appsv1.DeploymentStatus{}
	if err := json.Unmarshal(r.Status.Remote.Raw, s); err != nil {
		log.V(logging.Debug).Info("KubernetesApplicationResource not yet ready", "name", unicorn, "error", err)
		return false
	}

	return s.AvailableReplicas > 0
}

func (a *applicationReconciler) externalDNSValues(ctx context.Context, provider corev1.ObjectReference, dst chartutil.Values) error {
	switch provider.APIVersion {
	case xpgcpv1alpha1.APIVersion:
		p := &xpgcpv1alpha1.Provider{}
		n := types.NamespacedName{Namespace: provider.Namespace, Name: provider.Name}
		if err := a.client.Get(ctx, n, p); err != nil {
			return errors.Wrapf(err, errorFmtCannotGetProvider, n)
		}
		return values.Merge(dst, chartutil.Values{
			"provider": "google",
			"google":   chartutil.Values{"project": p.Spec.ProjectID},
		})
	case xpawsv1alpha1.APIVersion:
		return values.Merge(dst, chartutil.Values{"provider": "aws"})
	default:
		return errors.New(errorUnsupportedProvider)
	}
}

func (a *applicationReconciler) reconcileExternalDNS(ctx context.Context, cluster *corev1.ObjectReference) error {
	values := chartutil.Values{
		"rbac":          chartutil.Values{"create": true},
		"txtOwnerId":    a.GetUID(),
		"domainFilters": []string{a.Spec.Domain},
	}
	if err := a.externalDNSValues(ctx, a.GetProviderRef(), values); err != nil {
		return errors.Wrap(err, errorFailedToGenerateExternalDNSValues)
	}

	resources, err := a.chart.render(a.externalDNSChartURL, helm.WithValues(values))
	if err != nil {
		return errors.Wrap(err, errorProducingExternalDNSResources)
	}

	// TODO(negz): Override template namespaces, if necessary?
	// TODO(negz): Provide a cluster selector instead of a cluster reference.
	app := a.application.create(a.GetName()+externalDNSAppSuffix, application.Templates(resources),
		application.WithNamespace(a.GetNamespace()),
		application.WithCluster(cluster),
		application.WithControllerReference(metav1.NewControllerRef(a, v1alpha1.GitLabGroupVersionKind)),
		application.WithLabels(a.GetLabels()),
		application.WithAnnotations(map[string]string{
			annotationChartURL:   a.externalDNSChartURL,
			annotationGitlabHash: hash(a.GitLab, values),
		}))

	remote := app.DeepCopy()
	return errors.Wrap(crud.CreateOrUpdate(ctx, a.client, remote, func() error {
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
	}), errorSyncingExternalDNS)
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
			handle: h,

			gitlabChartURL:      gitlabChartURL,
			externalDNSChartURL: externalDNSChartURL,

			chart:       chartRendererFn(helm.Render),
			application: applicationCreatorFn(application.New),
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

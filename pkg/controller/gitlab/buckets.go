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

	xpcorev1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/core/v1alpha1"
	xpstoragev1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/storage/v1alpha1"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplaneio/gitlab-controller/pkg/apis/controller/v1alpha1"
)

const (
	bucketClaimKind = "bucket"

	errorMsgEmptyConnectionSecret = "empty connection secret"
	errorFmtFailedToParse         = "failed to parse %s"
	errorFmtFailedToSave          = "failed to save %s"

	errorFailedToCreateConnectionData = "failed to create connection data"
	errorFailedToCreateConfigData     = "failed to create s3cmd config data"
)

// secret updater
type secretUpdater interface {
	update(*corev1.Secret) error
}

// connectionKey is a secret key for connection data
const connectionKey = "connection"

// configKey is a secret key for s3cmd config data
const configKey = "config"

// secretDataCreator interface to be implemented by a specific provider
type secretDataCreator interface {
	create(*corev1.Secret) error
}

// secretTransformer interface defines operation of transforming connection secret data
type secretTransformer interface {
	transform(context.Context) error
}

// gitLabSecretTransformer
type gitLabSecretTransformer struct {
	*baseResourceReconciler
	secretUpdaters map[string]secretUpdater
}

// newGitLabSecretTransformer returns new instance of secret transformer with supported provider/updater map
func newGitLabSecretTransformer(base *baseResourceReconciler) *gitLabSecretTransformer {
	return &gitLabSecretTransformer{
		baseResourceReconciler: base,
		secretUpdaters: map[string]secretUpdater{
			"aws.crossplane.io/v1alpha1": newAWSSecretUpdater(),
			"gcp.crossplane.io/v1alpha1": newGCPSecretUpdater(),
		},
	}
}

// transform GitLab bucket secret
func (t *gitLabSecretTransformer) transform(ctx context.Context) error {
	if t.status == nil {
		return errors.New(errorResourceStatusIsNotFound)
	}

	s := &corev1.Secret{}
	key := types.NamespacedName{Namespace: t.GetNamespace(), Name: t.status.CredentialsSecretRef.Name}
	if err := t.client.Get(ctx, key, s); err != nil {
		return errors.Wrapf(err, errorFmtFailedToRetrieveConnectionSecret, key)
	}

	providerAPIVersion := t.GetProviderRef().APIVersion
	updater, found := t.secretUpdaters[providerAPIVersion]
	if !found {
		return errors.Errorf(errorFmtNotSupportedProvider, providerAPIVersion)
	}

	if err := updater.update(s); err != nil {
		return errors.Wrapf(err, errorFmtFailedToUpdateConnectionSecretData, key)
	}

	return errors.Wrapf(t.client.Update(ctx, s), errorFmtFailedToUpdateConnectionSecret, key)
}

// bucketReconciler
type bucketReconciler struct {
	*baseResourceReconciler
	resourceClassFinder resourceClassFinder
	secretTransformer   secretTransformer
	helmValuesFunction  helmValuesFunction
}

func newBucketReconciler(gitlab *v1alpha1.GitLab, client client.Client, name string, helmValuesFunction helmValuesFunction) *bucketReconciler {
	base := newBaseResourceReconciler(gitlab, client, name)
	return &bucketReconciler{
		baseResourceReconciler: base,
		resourceClassFinder:    base,
		secretTransformer:      newGitLabSecretTransformer(base),
		helmValuesFunction:     helmValuesFunction,
	}
}

func (r *bucketReconciler) reconcile(ctx context.Context) error {
	ref, err := r.resourceClassFinder.find(ctx, r.GetProviderRef(), xpstoragev1alpha1.BucketKindAPIVersion)
	if err != nil {
		return errors.Wrapf(err, errorFmtFailedToFindResourceClass, r.getClaimKind(), r.GetProviderRef())
	}

	meta := r.newObjectMeta(r.componentName)

	bucket := &xpstoragev1alpha1.Bucket{
		ObjectMeta: meta,
		Spec: xpstoragev1alpha1.BucketSpec{
			ClassRef: ref,
			Name:     strings.Join([]string{meta.Name, "%s"}, "-"),
		},
	}
	key := r.newNamespacedName(r.componentName)

	if err := r.client.Get(ctx, key, bucket); err != nil {
		if kerrors.IsNotFound(err) {
			return errors.Wrapf(r.client.Create(ctx, bucket), errorFmtFailedToCreate, r.getClaimKind(), key)
		}
		return errors.Wrapf(err, errorFmtFailedToRetrieveInstance, r.getClaimKind(), key)
	}

	r.status = &bucket.Status
	if bucket.Status.IsReady() {
		return r.secretTransformer.transform(ctx)
	}
	return nil
}

func (r *bucketReconciler) getClaimKind() string {
	return bucketClaimKind + "-" + r.componentName
}

func (r *bucketReconciler) getHelmValues(ctx context.Context, values map[string]string) error {
	return r.baseResourceReconciler.loadHelmValues(ctx, values, r.helmValuesFunction)
}

const (
	helmValueBucketFmt           = "global.appConfig.%s.bucket"
	helmValueConnectionSecretFmt = "global.appConfig.%s.connection.secret"
	helmValueConnectionKeyFmt    = "global.appConfig.%s.connection.key"
	helmValueTaskRunnerSecret    = "gitlab.task-runner.backups.objectStorage.config.secret"
	helmValueTaskRunnerKey       = "gitlab.task-runner.backups.objectStorage.config.key"
	helmValueBackupsTempBucket   = "global.appConfig.backups.tmpBucket"
)

// bucketConnectionHelmValues map of helm set key/value paris
// https://docs.gitlab.com/charts/advanced/external-object-storage/index.html#lfs-artifacts-uploads-packages-external-diffs-pseudonymizer
func bucketConnectionHelmValues(values map[string]string, name string, secret *corev1.Secret) {
	values[fmt.Sprintf(helmValueBucketFmt, name)] = string(secret.Data[xpcorev1alpha1.ResourceCredentialsSecretEndpointKey])
	values[fmt.Sprintf(helmValueConnectionSecretFmt, name)] = secret.GetName()
	values[fmt.Sprintf(helmValueConnectionKeyFmt, name)] = connectionKey
}

// bucketBackupsHelmValues map of helm set key/value paris
// https://docs.gitlab.com/charts/advanced/external-object-storage/index.html#backups
func bucketBackupsHelmValues(values map[string]string, name string, secret *corev1.Secret) {
	values[fmt.Sprintf(helmValueBucketFmt, name)] = string(secret.Data[xpcorev1alpha1.ResourceCredentialsSecretEndpointKey])
	values[helmValueTaskRunnerSecret] = secret.GetName()
	values[helmValueTaskRunnerKey] = configKey
}

// bucketBackupsTempHelmValues map of helm set key/value paris
// https://docs.gitlab.com/charts/advanced/external-object-storage/index.html#backups
func bucketBackupsTempHelmValues(values map[string]string, name string, _ *corev1.Secret) {
	values[helmValueBackupsTempBucket] = name
}

var _ resourceReconciler = &bucketReconciler{}

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

	xpcorev1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/core/v1alpha1"
	xpstoragev1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/storage/v1alpha1"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplaneio/gitlab-controller/pkg/apis/controller/v1alpha1"
)

const (
	postgresqlClaimKind = "postgresql"
)

// postgresReconciler
type postgresReconciler struct {
	*baseResourceReconciler
	resourceClassFinder resourceClassFinder
}

func (r *postgresReconciler) reconcile(ctx context.Context) error {
	ref, err := r.resourceClassFinder.find(ctx, r.GetProviderRef(), xpstoragev1alpha1.PostgreSQLInstanceKindAPIVersion)
	if err != nil {
		return errors.Wrapf(err, errorFmtFailedToFindResourceClass, r.getClaimKind(), r.GetProviderRef())
	}

	pg := &xpstoragev1alpha1.PostgreSQLInstance{
		ObjectMeta: r.newObjectMeta(),
		Spec: xpstoragev1alpha1.PostgreSQLInstanceSpec{
			ClassRef:      ref,
			EngineVersion: postgresEngineVersion,
		},
	}
	key := r.newNamespacedName()

	if err := r.client.Get(ctx, key, pg); err != nil {
		if kerrors.IsNotFound(err) {
			return errors.Wrapf(r.client.Create(ctx, pg), errorFmtFailedToCreate, r.getClaimKind(), key)
		}
		return errors.Wrapf(err, errorFmtFailedToRetrieveInstance, r.getClaimKind(), key)
	}

	r.status = &pg.Status
	return nil
}

func (r *postgresReconciler) getClaimKind() string {
	return postgresqlClaimKind
}

func (r *postgresReconciler) getHelmValues(ctx context.Context, values map[string]string) error {
	return r.loadHelmValues(ctx, values, postgresHelmValues)
}

const (
	helmPostgresComponentName = "psql"
	helmValuePsqlHostKey      = "global." + helmPostgresComponentName + "psql.host"
	helmValuePsqlDatabaseKey  = "global." + helmPostgresComponentName + ".database"
	helmValuePsqlUsernameKey  = "global." + helmPostgresComponentName + ".username"
)

func postgresHelmValues(values map[string]string, _ string, secret *corev1.Secret) {
	values[helmValuePsqlHostKey] = string(secret.Data[xpcorev1alpha1.ResourceCredentialsSecretEndpointKey])
	values[helmValuePsqlUsernameKey] = string(secret.Data[xpcorev1alpha1.ResourceCredentialsSecretUserKey])
	values[helmValuePsqlDatabaseKey] = "postgres"
}

var _ resourceReconciler = &postgresReconciler{}

func newPostgresReconciler(gitlab *v1alpha1.GitLab, client client.Client) *postgresReconciler {
	base := newBaseResourceReconciler(gitlab, client, helmPostgresComponentName)
	return &postgresReconciler{
		baseResourceReconciler: base,
		resourceClassFinder:    base,
	}
}

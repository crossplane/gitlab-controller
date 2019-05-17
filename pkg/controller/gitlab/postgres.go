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
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/helm/pkg/chartutil"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplaneio/gitlab-controller/pkg/apis/controller/v1alpha1"
	"github.com/crossplaneio/gitlab-controller/pkg/controller/gitlab/values"
)

const (
	postgresqlClaimKind     = "postgresql"
	defaultPostgresDatabase = "postgres"
)

// postgresReconciler
type postgresReconciler struct {
	*baseResourceReconciler
	resourceClassFinder resourceClassFinder
	ref                 *corev1.ObjectReference
}

func (r *postgresReconciler) reconcile(ctx context.Context) error {
	ref, err := r.resourceClassFinder.find(ctx, r.GetProviderRef(), xpstoragev1alpha1.PostgreSQLInstanceKindAPIVersion)
	if err != nil {
		return errors.Wrapf(err, errorFmtFailedToFindResourceClass, r.getClaimKind(), r.GetProviderRef())
	}

	// TODO(negz): Set connection secret override to something unique.
	pg := &xpstoragev1alpha1.PostgreSQLInstance{
		ObjectMeta: r.newObjectMeta(xpstoragev1alpha1.PostgreSQLInstanceKind),
		Spec: xpstoragev1alpha1.PostgreSQLInstanceSpec{
			ClassRef:      ref,
			EngineVersion: postgresEngineVersion,
		},
	}
	key := types.NamespacedName{Namespace: pg.GetNamespace(), Name: pg.GetName()}

	if err := r.client.Get(ctx, key, pg); err != nil {
		if kerrors.IsNotFound(err) {
			return errors.Wrapf(r.client.Create(ctx, pg), errorFmtFailedToCreate, r.getClaimKind(), key)
		}
		return errors.Wrapf(err, errorFmtFailedToRetrieveInstance, r.getClaimKind(), key)
	}

	r.status = &pg.Status
	r.ref = &corev1.ObjectReference{
		Kind:       pg.GetObjectKind().GroupVersionKind().Kind,
		APIVersion: pg.GetObjectKind().GroupVersionKind().Version,
		Namespace:  pg.GetNamespace(),
		Name:       pg.GetName(),
		UID:        pg.GetUID(),
	}
	return nil
}

func (r *postgresReconciler) getClaimKind() string {
	return postgresqlClaimKind
}

func (r *postgresReconciler) getClaimRef() *corev1.ObjectReference {
	return r.ref
}

func (r *postgresReconciler) getHelmValues(ctx context.Context, dst chartutil.Values, secretPrefix string) error {
	return r.loadHelmValues(ctx, dst, postgresHelmValues, secretPrefix)
}

func postgresHelmValues(dst chartutil.Values, s *corev1.Secret, _, secretPrefix string) error {
	return values.Merge(dst, chartutil.Values{
		valuesKeyGlobal: chartutil.Values{
			valuesKeyPSQL: chartutil.Values{
				"host":     string(s.Data[xpcorev1alpha1.ResourceCredentialsSecretEndpointKey]),
				"username": string(s.Data[xpcorev1alpha1.ResourceCredentialsSecretUserKey]),
				"database": defaultPostgresDatabase,
				"password": chartutil.Values{
					"secret": secretPrefix + s.GetName(),
					"key":    xpcorev1alpha1.ResourceCredentialsSecretPasswordKey,
				},
			},
		},
	})
}

func newPostgresReconciler(gitlab *v1alpha1.GitLab, client client.Client) *postgresReconciler {
	base := newBaseResourceReconciler(gitlab, client, postgresqlClaimKind)
	return &postgresReconciler{
		baseResourceReconciler: base,
		resourceClassFinder:    base,
	}
}

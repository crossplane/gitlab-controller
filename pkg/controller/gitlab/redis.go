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

	xpcachev1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/cache/v1alpha1"
	xpcorev1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/core/v1alpha1"
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
	redisClaimKind = "redis"
)

// redisReconciler
type redisReconciler struct {
	*baseResourceReconciler
	resourceClassFinder resourceClassFinder
}

func (r *redisReconciler) reconcile(ctx context.Context) error {
	ref, err := r.resourceClassFinder.find(ctx, r.GetProviderRef(), xpcachev1alpha1.RedisClusterKindAPIVersion)
	if err != nil {
		return errors.Wrapf(err, errorFmtFailedToFindResourceClass, r.getClaimKind(), r.GetProviderRef())
	}

	// TODO(negz): Set connection secret override to something unique.
	red := &xpcachev1alpha1.RedisCluster{
		ObjectMeta: r.newObjectMeta(xpcachev1alpha1.RedisClusterKind),
		Spec: xpcachev1alpha1.RedisClusterSpec{
			ClassRef:      ref,
			EngineVersion: redisEngineVersion,
		},
	}
	key := types.NamespacedName{Namespace: red.GetNamespace(), Name: red.GetName()}

	if err := r.client.Get(ctx, key, red); err != nil {
		if kerrors.IsNotFound(err) {
			return errors.Wrapf(r.client.Create(ctx, red), errorFmtFailedToCreate, r.getClaimKind(), key)
		}
		return errors.Wrapf(err, errorFmtFailedToRetrieveInstance, r.getClaimKind(), key)
	}

	r.status = &red.Status
	return nil
}

func (r *redisReconciler) getClaimKind() string {
	return redisClaimKind
}

func (r *redisReconciler) getHelmValues(ctx context.Context, dst chartutil.Values, secretPrefix string) error {
	return r.loadHelmValues(ctx, dst, redisHelmValues, secretPrefix)
}

func redisHelmValues(dst chartutil.Values, s *corev1.Secret, _, _ string) error {
	return values.Merge(dst, chartutil.Values{
		valuesKeyGlobal: chartutil.Values{
			valuesKeyRedis: chartutil.Values{
				"password": chartutil.Values{"enabled": false},
				"host":     string(s.Data[xpcorev1alpha1.ResourceCredentialsSecretEndpointKey]),
			},
		},
	})
}

func newRedisReconciler(gitlab *v1alpha1.GitLab, client client.Client) *redisReconciler {
	base := newBaseResourceReconciler(gitlab, client, redisClaimKind)
	return &redisReconciler{
		baseResourceReconciler: base,
		resourceClassFinder:    base,
	}
}

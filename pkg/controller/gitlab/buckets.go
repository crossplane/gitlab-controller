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

	xpstoragev1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/storage/v1alpha1"
	"github.com/pkg/errors"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplaneio/gitlab-controller/pkg/apis/controller/v1alpha1"
)

const (
	bucketClaimKind = "bucket"
)

// bucketReconciler
type bucketReconciler struct {
	*baseResourceReconciler
	resourceClassFinder resourceClassFinder
	bucketName          string
}

func (r *bucketReconciler) reconcile(ctx context.Context) error {
	ref, err := r.resourceClassFinder.find(ctx, r.GetProviderRef(), xpstoragev1alpha1.BucketKindAPIVersion)
	if err != nil {
		return errors.Wrapf(err, errorFmtFailedToFindResourceClass, r.getClaimKind(), r.GetProviderRef())
	}

	meta := r.newObjectMeta(r.bucketName)

	bucket := &xpstoragev1alpha1.Bucket{
		ObjectMeta: meta,
		Spec: xpstoragev1alpha1.BucketSpec{
			ClassRef: ref,
			Name:     strings.Join([]string{meta.Name, "%s"}, "-"),
		},
	}
	key := r.newNamespacedName(r.bucketName)

	if err := r.client.Get(ctx, key, bucket); err != nil {
		if kerrors.IsNotFound(err) {
			return errors.Wrapf(r.client.Create(ctx, bucket), errorFmtFailedToCreate, r.getClaimKind(), key)
		}
		return errors.Wrapf(err, errorFmtFailedToRetrieveInstance, r.getClaimKind(), key)
	}

	r.status = &bucket.Status
	return nil
}

func (r *bucketReconciler) getClaimKind() string {
	return bucketClaimKind + "-" + r.bucketName
}

var _ resourceReconciler = &bucketReconciler{}

func newBucketReconciler(gitlab *v1alpha1.GitLab, client client.Client, bucketName string) *bucketReconciler {
	base := newBaseComponentReconciler(gitlab, client)
	return &bucketReconciler{
		baseResourceReconciler: base,
		resourceClassFinder:    base,
		bucketName:             bucketName,
	}
}

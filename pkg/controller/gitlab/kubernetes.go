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

	xpcomputev1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/compute/v1alpha1"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/helm/pkg/chartutil"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplaneio/gitlab-controller/pkg/apis/controller/v1alpha1"
)

const (
	kubernetesClaimKind = "kubernetes"
)

// kubernetesReconciler
type kubernetesReconciler struct {
	*baseResourceReconciler
	resourceClassFinder resourceClassFinder
	ref                 *corev1.ObjectReference
}

func (r *kubernetesReconciler) reconcile(ctx context.Context) error {
	// TODO(negz): Set connection secret override to something unique.
	cluster := &xpcomputev1alpha1.KubernetesCluster{
		ObjectMeta: r.newObjectMeta(xpcomputev1alpha1.KubernetesClusterKind),
	}

	// Check if spec contains cluster reference
	if ref := r.GetClusterRef(); ref != nil {
		// Use Cluster reference to retrieve the existing Kubernetes cluster
		key := types.NamespacedName{Namespace: ref.Namespace, Name: ref.Name}
		if err := r.client.Get(ctx, key, cluster); err != nil {
			return errors.Wrapf(err, errorFmtFailedToRetrieveInstance, r.getClaimKind(), key)
		}

		r.status = &cluster.Status
		r.ref = &corev1.ObjectReference{
			Kind:       cluster.GetObjectKind().GroupVersionKind().Kind,
			APIVersion: cluster.GetObjectKind().GroupVersionKind().Version,
			Namespace:  cluster.GetNamespace(),
			Name:       cluster.GetName(),
			UID:        cluster.GetUID(),
		}
		return nil
	}

	// Find and use provider reference to create new Kubernetes cluster
	classRef, err := r.resourceClassFinder.find(ctx, r.GetProviderRef(), xpcomputev1alpha1.KubernetesClusterKindAPIVersion)
	if err != nil {
		return errors.Wrapf(err, errorFmtFailedToFindResourceClass, r.getClaimKind(), r.GetProviderRef())
	}
	cluster.Spec.ClassRef = classRef
	key := types.NamespacedName{Namespace: cluster.GetNamespace(), Name: cluster.GetName()}

	if err := r.client.Get(ctx, key, cluster); err != nil {
		if kerrors.IsNotFound(err) {
			return errors.Wrapf(r.client.Create(ctx, cluster), errorFmtFailedToCreate, r.getClaimKind(), key)
		}
		return errors.Wrapf(err, errorFmtFailedToRetrieveInstance, r.getClaimKind(), key)
	}

	r.status = &cluster.Status
	r.ref = &corev1.ObjectReference{
		Kind:       cluster.GetObjectKind().GroupVersionKind().Kind,
		APIVersion: cluster.GetObjectKind().GroupVersionKind().Version,
		Namespace:  cluster.GetNamespace(),
		Name:       cluster.GetName(),
		UID:        cluster.GetUID(),
	}
	return nil
}

func (r *kubernetesReconciler) getClaimKind() string {
	return kubernetesClaimKind
}

func (r *kubernetesReconciler) getClaimRef() *corev1.ObjectReference {
	return r.ref
}

func (r *kubernetesReconciler) getHelmValues(_ context.Context, _ chartutil.Values, _ string) error {
	return nil
}

func newKubernetesReconciler(gitlab *v1alpha1.GitLab, client client.Client) *kubernetesReconciler {
	base := newBaseResourceReconciler(gitlab, client, "")
	return &kubernetesReconciler{
		baseResourceReconciler: base,
		resourceClassFinder:    base,
	}
}

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
	"time"

	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/crossplaneio/gitlab-controller/pkg/apis/controller/v1alpha1"
	"github.com/crossplaneio/gitlab-controller/pkg/logging"
)

const (
	controllerName   = "gitlab-controller"
	reconcileTimeout = 5 * time.Minute
)

var log = logging.Logger.WithName("controller." + controllerName)

// Reconciler reconciles a GitLab object
type Reconciler struct {
	client.Client
	scheme *runtime.Scheme
	mill   reconcilerMill
}

// Reconcile reads that state of the cluster for a GitLab object and makes changes based on the state read
// and what is in the GitLab.Spec
// Automatically generate RBAC rules to allow the Controller to read and write Deployments
// +kubebuilder:rbac:groups=apps,resourceClaims=deployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps,resourceClaims=deployments/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=controller.gitlab.io,resourceClaims=gitlabs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=controller.gitlab.io,resourceClaims=gitlabs/status,verbs=get;update;patch
func (r *Reconciler) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	log.V(logging.Debug).Info("reconciling", "request", request)

	ctx, cancel := context.WithTimeout(context.Background(), reconcileTimeout)
	defer cancel()

	// Fetch the GitLab instance
	instance := &v1alpha1.GitLab{}
	err := r.Get(ctx, request.NamespacedName, instance)
	if err != nil {
		if kerrors.IsNotFound(err) {
			// Object not found, return.  Created objects are automatically garbage collected.
			// For additional cleanup logic use finalizers.
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return reconcile.Result{}, err
	}

	return r.mill.newReconciler(instance, r.Client).reconcile(ctx)
}

var _ reconcile.Reconciler = &Reconciler{}

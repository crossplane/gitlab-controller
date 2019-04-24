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

	"github.com/pkg/errors"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	"github.com/crossplaneio/gitlab-controller/pkg/apis/controller/v1alpha1"
	"github.com/crossplaneio/gitlab-controller/pkg/logging"
)

const (
	controllerName      = "gitlab-controller"
	reconcileTimeout    = 5 * time.Minute
	requeueAfterSuccess = 1 * time.Minute
)

var (
	log = logging.Logger.WithName("controller." + controllerName)
)

// Add creates a new GitLab Controller and adds it to the Manager with default RBAC. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	r := &Reconciler{Client: mgr.GetClient()}

	// Create a new controller
	c, err := controller.New("gitlab-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to GitLab
	return c.Watch(&source.Kind{Type: &v1alpha1.GitLab{}}, &handler.EnqueueRequestForObject{})
}

var _ reconcile.Reconciler = &Reconciler{}

// Reconciler reconciles a GitLab object
type Reconciler struct {
	client.Client
}

// Reconcile reads that state of the cluster for a GitLab object and makes changes based on the state read
// and what is in the GitLab.Spec
// Automatically generate RBAC rules to allow the Controller to read and write Deployments
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps,resources=deployments/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=controller.gitlab.io,resources=gitlabs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=controller.gitlab.io,resources=gitlabs/status,verbs=get;update;patch
func (r *Reconciler) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	log.V(logging.Debug).Info("reconciling", "kind", v1alpha1.GitLabKind, "request", request)

	ctx, cancel := context.WithTimeout(context.Background(), reconcileTimeout)
	defer cancel()

	// Fetch the GitLab instance
	instance := &v1alpha1.GitLab{}
	err := r.Get(context.TODO(), request.NamespacedName, instance)
	if err != nil {
		if kerrors.IsNotFound(err) {
			// Object not found, return.  Created objects are automatically garbage collected.
			// For additional cleanup logic use finalizers.
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return reconcile.Result{}, err
	}

	// TODO: implement GitLab controller reconciler logic and set appropriate status
	instance.SetReady()
	instance.Status.Endpoint = instance.GetEndpoint()

	return reconcile.Result{RequeueAfter: requeueAfterSuccess}, errors.Wrapf(r.Status().Update(ctx, instance), "failed to update object status")
}

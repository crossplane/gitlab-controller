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
	xpcachev1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/cache/v1alpha1"
	xpcomputev1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/compute/v1alpha1"
	xpcorev1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/core/v1alpha1"
	xpstoragev1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/storage/v1alpha1"
	xpworkloadv1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/workload/v1alpha1"

	"github.com/pkg/errors"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/source"

	"github.com/crossplaneio/gitlab-controller/pkg/apis/controller/v1alpha1"
)

// Add creates a new GitLab Controller and adds it to the Manager with default RBAC. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	r := &Reconciler{Client: mgr.GetClient(), mill: &gitLabReconcilerMill{}, scheme: mgr.GetScheme()}

	// Create a new controller
	c, err := controller.New(controllerName, mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to GitLab
	if err := c.Watch(&source.Kind{Type: &v1alpha1.GitLab{}}, &handler.EnqueueRequestForObject{}); err != nil {
		return errors.Wrapf(err, "cannot watch for %s", v1alpha1.GitLabKindAPIVersion)
	}

	_ = xpcorev1alpha1.SchemeBuilder.AddToScheme(mgr.GetScheme())
	_ = xpcachev1alpha1.SchemeBuilder.AddToScheme(mgr.GetScheme())
	_ = xpcomputev1alpha1.SchemeBuilder.AddToScheme(mgr.GetScheme())
	_ = xpstoragev1alpha1.SchemeBuilder.AddToScheme(mgr.GetScheme())
	_ = xpworkloadv1alpha1.SchemeBuilder.AddToScheme(mgr.GetScheme())

	// Watch for changes to crossplane KubernetesApplication
	return errors.Wrapf(c.Watch(&source.Kind{Type: &xpworkloadv1alpha1.KubernetesApplication{}},
		&handler.EnqueueRequestForOwner{
			IsController: true,
			OwnerType:    &v1alpha1.GitLab{},
		}), "cannot watch for %s", xpworkloadv1alpha1.KubernetesApplicationKindAPIVersion)
}

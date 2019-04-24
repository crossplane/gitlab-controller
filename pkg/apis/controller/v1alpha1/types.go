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

package v1alpha1

import (
	"fmt"
	"strconv"

	"github.com/crossplaneio/crossplane/pkg/util"

	xpcorev1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/core/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ApplicationName gitlab
const ApplicationName = "gitlab"

// GitLabSpec defines the desired state of GitLab
type GitLabSpec struct {
	// Domain which will contain records to resolve gitlab, registry, and minio (if enabled) to the appropriate IP
	Domain string `json:"domain"`
	// HostSuffix appended to domain records, i.e. `gitlab-suffix.domain`, default - no suffix
	HostSuffix string `json:"hostSuffix,omitempty"`
	// Protocol http or https, default - https
	Protocol string `json:"protocol,omitempty"`
	// Port gitlab service port, default - none
	Port uint `json:"port,omitempty"`
	// Email address to register TLS certificates
	Email string `json:"email"`

	// ProviderRef cloud provider reference
	ProviderRef corev1.ObjectReference `json:"providerRef"`
	// ReclaimPolicy controls application cleanup
	ReclaimPolicy xpcorev1alpha1.ReclaimPolicy `json:"reclaimPolicy,omitempty"`

	// ClusterSelector label based
	ClusterSelector *metav1.LabelSelector `json:"clusterSelector,omitempty"`

	// ClusterNamespace
	ClusterNamespace string `json:"clusterNamespace,omitempty"`
}

// GitLabStatus defines the observed state of GitLab
type GitLabStatus struct {
	xpcorev1alpha1.ConditionedStatus

	// Endpoint for GitLab service
	Endpoint string `json:"endpoint,omitempty"`

	// State of the GitLab service - string representation of the latest active condition
	// This is provided for convenience for displaying status gitlab
	State xpcorev1alpha1.ConditionType `json:"state,omitempty"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// GitLab is the Schema for the gitlabs API
// +k8s:openapi-gen=true
// +groupName=gitlab
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="STATE",type="string",JSONPath=".status.state"
// +kubebuilder:printcolumn:name="ENDPOINT",type="string",JSONPath=".status.endpoint"
type GitLab struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   GitLabSpec   `json:"spec,omitempty"`
	Status GitLabStatus `json:"status,omitempty"`
}

// GetApplicationName returns "gitlab[-suffix]" if host suffix is provided
func (in *GitLab) GetApplicationName() string {
	if sfx := in.Spec.HostSuffix; sfx != "" {
		return ApplicationName + "-" + sfx
	}
	return ApplicationName
}

// GetClusterNamespace spec property
func (in *GitLab) GetClusterNamespace() string {
	if in.Spec.ClusterNamespace == "" {
		return "default"
	}
	return in.Spec.ClusterNamespace
}

// GetClusterSelector spec property
func (in *GitLab) GetClusterSelector() *metav1.LabelSelector {
	return in.Spec.ClusterSelector
}

// GetEndpoint returns a gitlab service endpoint
func (in *GitLab) GetEndpoint() string {
	f := func(a, b string) string {
		if a == "" {
			return b
		}
		return a
	}
	protocol := f(in.Spec.Protocol, "https")
	suffix := f(in.Spec.HostSuffix, "")
	if suffix != "" {
		suffix = "-" + suffix
	}
	port := ""
	if in.Spec.Port != 0 {
		port = ":" + strconv.Itoa(int(in.Spec.Port))
	}

	return fmt.Sprintf("%s://gitlab%s.%s%s", protocol, suffix, in.Spec.Domain, port)
}

// GetProviderRef spec property
func (in *GitLab) GetProviderRef() corev1.ObjectReference {
	return in.Spec.ProviderRef
}

// IsReclaimDelete tests reclaim delete policy
func (in *GitLab) IsReclaimDelete() bool {
	return in.Spec.ReclaimPolicy == xpcorev1alpha1.ReclaimDelete
}

// SetCreating a convenience method to set object status
func (in *GitLab) SetCreating() {
	in.Status.SetCreating()
	in.Status.State = xpcorev1alpha1.Creating
}

// SetDeleting a convenience method to set object status
func (in *GitLab) SetDeleting() {
	in.Status.SetDeleting()
	in.Status.State = xpcorev1alpha1.Deleting
}

// SetFailed a convenience method to set object status
func (in *GitLab) SetFailed(reason, msg string) {
	in.Status.SetFailed(reason, msg)
	in.Status.State = xpcorev1alpha1.Failed
}

// SetPending status with reason and message
func (in *GitLab) SetPending(reason, msg string) {
	in.Status.UnsetAllConditions()
	in.Status.SetCondition(xpcorev1alpha1.NewCondition(xpcorev1alpha1.Pending, reason, msg))
	in.Status.State = xpcorev1alpha1.Pending
}

// SetReady a convenience method to set object status
func (in *GitLab) SetReady() {
	if !in.Status.IsReady() {
		in.Status.UnsetAllConditions()
		in.Status.SetReady()
	}
	in.Status.State = xpcorev1alpha1.Ready
}

// SetEndpoint for this GitLab instance
func (in *GitLab) SetEndpoint(ep string) {
	in.Status.Endpoint = ep
}

// ToOwnerReference converts this object metadata to a new owner reference
func (in *GitLab) ToOwnerReference() metav1.OwnerReference {
	return metav1.OwnerReference{
		APIVersion: util.IfEmptyString(in.APIVersion, APIVersion),
		Kind:       util.IfEmptyString(in.Kind, GitLabKind),
		Name:       in.Name,
		UID:        in.UID,
	}
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// GitLabList contains a list of GitLab
type GitLabList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []GitLab `json:"items"`
}

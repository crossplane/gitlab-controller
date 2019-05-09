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
	"testing"

	xpcorev1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/core/v1alpha1"
	"github.com/google/go-cmp/cmp"
	"github.com/onsi/gomega"
	"golang.org/x/net/context"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplaneio/gitlab-controller/pkg/util"

	"github.com/crossplaneio/gitlab-controller/pkg/test"
)

const (
	testNamespace = "default"
	testName      = "test-gitlab"
)

var (
	c client.Client

	testKey = types.NamespacedName{
		Name:      testName,
		Namespace: testNamespace,
	}

	testMeta = metav1.ObjectMeta{
		Name:      testName,
		Namespace: testNamespace,
	}
)

func TestMain(m *testing.M) {
	t := test.NewEnv(testNamespace, SchemeBuilder.SchemeBuilder, test.CRDs())
	c = t.StartClient()
	t.StopAndExit(m.Run())
}

func TestStorageGitLab(t *testing.T) {
	ctx := context.TODO()
	created := &GitLab{ObjectMeta: testMeta}
	g := gomega.NewGomegaWithT(t)

	// Test Create
	fetched := &GitLab{}
	g.Expect(c.Create(ctx, created)).NotTo(gomega.HaveOccurred())

	g.Expect(c.Get(ctx, testKey, fetched)).NotTo(gomega.HaveOccurred())
	g.Expect(fetched).To(gomega.Equal(created))

	// Test Updating the Labels
	updated := fetched.DeepCopy()
	updated.Labels = map[string]string{"hello": "world"}
	g.Expect(c.Update(ctx, updated)).NotTo(gomega.HaveOccurred())

	g.Expect(c.Get(ctx, testKey, fetched)).NotTo(gomega.HaveOccurred())
	g.Expect(fetched).To(gomega.Equal(updated))

	// Test Delete
	g.Expect(c.Delete(ctx, fetched)).NotTo(gomega.HaveOccurred())
	g.Expect(c.Get(ctx, testKey, fetched)).To(gomega.HaveOccurred())
}

const (
	readyType    = xpcorev1alpha1.Ready
	creatingType = xpcorev1alpha1.Creating
	deletingType = xpcorev1alpha1.Deleting
	failedType   = xpcorev1alpha1.Failed
	pendingType  = xpcorev1alpha1.Pending
)

var (
	ready    = xpcorev1alpha1.NewCondition(readyType, "", "")
	creating = xpcorev1alpha1.NewCondition(creatingType, "", "")
	deleting = xpcorev1alpha1.NewCondition(deletingType, "", "")
	failed   = xpcorev1alpha1.NewCondition(failedType, "foo", "bar")
	pending  = xpcorev1alpha1.NewCondition(pendingType, "fooz", "barz")
)

type gl GitLab

func (gl *gl) withStatusCondition(c xpcorev1alpha1.Condition) *gl {
	gl.Status.SetCondition(c)
	return gl
}

func (gl *gl) withStatusState(s xpcorev1alpha1.Condition) *gl {
	gl.Status.State = s.Type
	return gl
}

type setTestCase struct {
	gitlab GitLab
	want   GitLabStatus
}

func setTestCases(cA, cB xpcorev1alpha1.Condition) map[string]setTestCase {
	spec := GitLabSpec{}

	return map[string]setTestCase{
		"NoStatus": {
			gitlab: GitLab{
				ObjectMeta: testMeta,
				Spec:       spec,
				Status:     GitLabStatus{},
			},
			want: (&gl{}).withStatusCondition(cA).withStatusState(cA).Status,
		},
		"SameStatus": {
			gitlab: GitLab{
				ObjectMeta: testMeta,
				Spec:       spec,
				Status:     (&gl{}).withStatusCondition(cA).withStatusState(cA).Status,
			},
			want: (&gl{}).withStatusCondition(cA).withStatusState(cA).Status,
		},
		"DifferentStatus": {
			gitlab: GitLab{
				ObjectMeta: testMeta,
				Spec:       spec,
				Status:     (&gl{}).withStatusCondition(cB).withStatusState(cB).Status,
			},
			want: (&gl{}).withStatusCondition(cB).withStatusCondition(cA).withStatusState(cA).Status,
		},
	}
}

func TestGitLabSetReady(t *testing.T) {
	for name, tt := range setTestCases(ready, failed) {
		t.Run(name, func(t *testing.T) {
			tt.gitlab.SetReady()
			if name == "DifferentStatus" {
				tt.want.UnsetCondition(xpcorev1alpha1.Failed)
			}
			if diff := cmp.Diff(tt.gitlab.Status, tt.want, cmp.Comparer(util.EqualConditionedStatus)); diff != "" {
				t.Errorf("SetReady() %s", diff)
			}
		})
	}
}

func TestGitLabSetCreating(t *testing.T) {
	for name, tt := range setTestCases(creating, failed) {
		t.Run(name, func(t *testing.T) {
			tt.gitlab.SetCreating()
			if diff := cmp.Diff(tt.gitlab.Status, tt.want); diff != "" {
				t.Errorf("SetCreating() %s", diff)
			}
		})
	}
}

func TestGitLabSetDeleting(t *testing.T) {
	for name, tt := range setTestCases(deleting, failed) {
		t.Run(name, func(t *testing.T) {
			tt.gitlab.SetDeleting()
			if diff := cmp.Diff(tt.gitlab.Status, tt.want); diff != "" {
				t.Errorf("SetDeleting() %s", diff)
			}
		})
	}
}

func TestGitLabSetFailed(t *testing.T) {
	for name, tt := range setTestCases(failed, pending) {
		t.Run(name, func(t *testing.T) {
			tt.gitlab.SetFailed("foo", "bar")
			if diff := cmp.Diff(tt.gitlab.Status, tt.want); diff != "" {
				t.Errorf("SetFailed() %s", diff)
			}
		})
	}
}

func TestGitLabSetPending(t *testing.T) {
	for name, tt := range setTestCases(pending, failed) {
		t.Run(name, func(t *testing.T) {
			tt.gitlab.SetPending("fooz", "barz")
			if name == "DifferentStatus" {
				tt.want.UnsetCondition(xpcorev1alpha1.Failed)
			}
			if diff := cmp.Diff(tt.gitlab.Status, tt.want, cmp.Comparer(util.EqualConditionedStatus)); diff != "" {
				t.Errorf("SetPending() %s", diff)
			}
		})
	}
}

func TestGitLabGetEndpoint(t *testing.T) {
	domain := "test.io"
	tests := []struct {
		name     string
		suffix   string
		protocol string
		port     uint
		want     string
	}{
		{name: "Defaults", suffix: "", want: "https://gitlab.test.io"},
		{name: "Protocol", protocol: "file", want: "file://gitlab.test.io"},
		{name: "Suffix", suffix: "foo", want: "https://gitlab-foo.test.io"},
		{name: "Port", port: 9999, want: "https://gitlab.test.io:9999"},
		{name: "AllOfTheAbove", protocol: "http", suffix: "demo", port: 8080, want: "http://gitlab-demo.test.io:8080"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gl := &GitLab{
				ObjectMeta: testMeta,
				Spec:       GitLabSpec{Domain: domain, HostSuffix: tt.suffix, Protocol: tt.protocol, Port: tt.port},
			}
			got := gl.GetEndpoint()
			if diff := cmp.Diff(got, tt.want); diff != "" {
				t.Errorf("GitLab.GetEndpoint() = %v, want %v\n%s", got, tt.want, diff)
			}
		})
	}
}

func TestGitLab_GetApplicationName(t *testing.T) {
	tests := map[string]struct {
		spec GitLabSpec
		want string
	}{
		"NoSuffix":   {spec: GitLabSpec{}, want: ApplicationName},
		"WithSuffix": {spec: GitLabSpec{HostSuffix: "foo"}, want: ApplicationName + "-foo"},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			gl := &GitLab{
				Spec: tt.spec,
			}
			if got := gl.GetApplicationName(); got != tt.want {
				t.Errorf("GitLab.GetApplicationName() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGitLab_GetClusterNamespace(t *testing.T) {
	tests := map[string]struct {
		spec GitLabSpec
		want string
	}{
		"Default": {spec: GitLabSpec{}, want: "default"},
		"Custom":  {spec: GitLabSpec{ClusterNamespace: "foo"}, want: "foo"},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			gl := &GitLab{
				Spec: tt.spec,
			}
			if got := gl.GetClusterNamespace(); got != tt.want {
				t.Errorf("GitLab.GetClusterNamespace() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGitLab_GetClusterSelector(t *testing.T) {
	tests := map[string]struct {
		spec GitLabSpec
		want *metav1.LabelSelector
	}{
		"NoSelector": {spec: GitLabSpec{}, want: nil},
		"Custom":     {spec: GitLabSpec{ClusterSelector: &metav1.LabelSelector{}}, want: &metav1.LabelSelector{}},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			gl := &GitLab{
				Spec: tt.spec,
			}
			got := gl.GetClusterSelector()
			if diff := cmp.Diff(got, tt.want); diff != "" {
				t.Errorf("GitLab.GetClusterSelector() %s", diff)
			}
		})
	}
}

func TestGitLab_GetProviderRef(t *testing.T) {
	tests := map[string]struct {
		spec GitLabSpec
		want corev1.ObjectReference
	}{
		"Default": {spec: GitLabSpec{}, want: corev1.ObjectReference{}},
		"Empty":   {spec: GitLabSpec{ProviderRef: corev1.ObjectReference{}}, want: corev1.ObjectReference{}},
		"Custom": {
			spec: GitLabSpec{ProviderRef: corev1.ObjectReference{Name: "foo", Namespace: "bar"}},
			want: corev1.ObjectReference{Namespace: "bar", Name: "foo"},
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			gl := &GitLab{
				Spec: tt.spec,
			}
			if got := gl.GetProviderRef(); got != tt.want {
				t.Errorf("GitLab.GetProviderRef() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGitLab_IsReclaimDelete(t *testing.T) {
	tests := map[string]struct {
		spec GitLabSpec
		want bool
	}{
		"Default": {spec: GitLabSpec{}, want: false},
		"Retain":  {spec: GitLabSpec{ReclaimPolicy: xpcorev1alpha1.ReclaimRetain}, want: false},
		"Delete":  {spec: GitLabSpec{ReclaimPolicy: xpcorev1alpha1.ReclaimDelete}, want: true},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			gl := &GitLab{
				Spec: tt.spec,
			}
			if got := gl.IsReclaimDelete(); got != tt.want {
				t.Errorf("GitLab.IsReclaimDelete() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGitLab_SetEndpoint(t *testing.T) {
	type fields struct {
		endpoint string
		status   GitLabStatus
	}
	tests := map[string]struct {
		fields fields
		want   string
	}{
		"Default":   {fields: fields{endpoint: "", status: GitLabStatus{}}, want: ""},
		"SetNew":    {fields: fields{endpoint: "foo", status: GitLabStatus{}}, want: "foo"},
		"Overwrite": {fields: fields{endpoint: "foo", status: GitLabStatus{Endpoint: "bar"}}, want: "foo"},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			gl := &GitLab{
				Status: tt.fields.status,
			}
			gl.SetEndpoint(tt.fields.endpoint)
			if diff := cmp.Diff(gl.Status.Endpoint, tt.want); diff != "" {
				t.Errorf("GitLab.SetEndpoint() %s", diff)
			}
		})
	}
}

func TestGitLab_ToOwnerReference(t *testing.T) {
	type fields struct {
		TypeMeta   metav1.TypeMeta
		ObjectMeta metav1.ObjectMeta
	}
	tests := map[string]struct {
		fields fields
		want   metav1.OwnerReference
	}{
		"Default": {
			fields: fields{},
			want: metav1.OwnerReference{
				APIVersion: APIVersion,
				Kind:       GitLabKind,
			},
		},
		"Custom": {
			fields: fields{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "foo",
				},
				ObjectMeta: testMeta,
			},
			want: metav1.OwnerReference{
				APIVersion: "foo",
				Kind:       GitLabKind,
				Name:       testName,
			},
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			gl := &GitLab{
				TypeMeta:   tt.fields.TypeMeta,
				ObjectMeta: tt.fields.ObjectMeta,
			}
			if diff := cmp.Diff(gl.ToOwnerReference(), tt.want); diff != "" {
				t.Errorf("GitLab.ToOwnerReference() = %s", diff)
			}
		})
	}
}

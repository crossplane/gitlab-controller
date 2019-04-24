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
	"github.com/go-test/deep"
	"github.com/onsi/gomega"
	"golang.org/x/net/context"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

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
	created := &GitLab{ObjectMeta: testMeta}
	g := gomega.NewGomegaWithT(t)

	// Test Create
	fetched := &GitLab{}
	g.Expect(c.Create(context.TODO(), created)).NotTo(gomega.HaveOccurred())

	g.Expect(c.Get(context.TODO(), testKey, fetched)).NotTo(gomega.HaveOccurred())
	g.Expect(fetched).To(gomega.Equal(created))

	// Test Updating the Labels
	updated := fetched.DeepCopy()
	updated.Labels = map[string]string{"hello": "world"}
	g.Expect(c.Update(context.TODO(), updated)).NotTo(gomega.HaveOccurred())

	g.Expect(c.Get(context.TODO(), testKey, fetched)).NotTo(gomega.HaveOccurred())
	g.Expect(fetched).To(gomega.Equal(updated))

	// Test Delete
	g.Expect(c.Delete(context.TODO(), fetched)).NotTo(gomega.HaveOccurred())
	g.Expect(c.Get(context.TODO(), testKey, fetched)).To(gomega.HaveOccurred())
}

const (
	readyType    = xpcorev1alpha1.Ready
	creatingType = xpcorev1alpha1.Creating
	deletingType = xpcorev1alpha1.Deleting
	failedType   = xpcorev1alpha1.Failed
)

var (
	ready    = xpcorev1alpha1.NewCondition(readyType, "", "")
	creating = xpcorev1alpha1.NewCondition(creatingType, "", "")
	deleting = xpcorev1alpha1.NewCondition(deletingType, "", "")
	failed   = xpcorev1alpha1.NewCondition(failedType, "foo", "bar")
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
	name   string
	gitlab GitLab
	want   GitLabStatus
}

func setTestCases(cA, cB xpcorev1alpha1.Condition) []setTestCase {
	spec := GitLabSpec{}

	return []setTestCase{
		{
			name: "NoStatus",
			gitlab: GitLab{
				ObjectMeta: testMeta,
				Spec:       spec,
				Status:     GitLabStatus{},
			},
			want: (&gl{}).withStatusCondition(cA).withStatusState(cA).Status,
		},
		{
			name: "SameStatus",
			gitlab: GitLab{
				ObjectMeta: testMeta,
				Spec:       spec,
				Status:     (&gl{}).withStatusCondition(cA).withStatusState(cA).Status,
			},
			want: (&gl{}).withStatusCondition(cA).withStatusState(cA).Status,
		},
		{
			name: "DifferentStatus",
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
	for _, tt := range setTestCases(ready, failed) {
		t.Run(tt.name, func(t *testing.T) {
			tt.gitlab.SetReady()
			if diff := deep.Equal(tt.gitlab.Status, tt.want); diff != nil {
				t.Errorf("SetReady() = %v, want %v\n%s", tt.gitlab.Status, tt.want, diff)
			}
		})
	}
}

func TestGitLabSetCreating(t *testing.T) {
	for _, tt := range setTestCases(creating, failed) {
		t.Run(tt.name, func(t *testing.T) {
			tt.gitlab.SetCreating()
			if diff := deep.Equal(tt.gitlab.Status, tt.want); diff != nil {
				t.Errorf("SetCreating() = %v, want %v\n%s", tt.gitlab.Status, tt.want, diff)
			}
		})
	}
}

func TestGitLabSetDeleting(t *testing.T) {
	for _, tt := range setTestCases(deleting, failed) {
		t.Run(tt.name, func(t *testing.T) {
			tt.gitlab.SetDeleting()
			if diff := deep.Equal(tt.gitlab.Status, tt.want); diff != nil {
				t.Errorf("SetDeleting() = %v, want %v\n%s", tt.gitlab.Status, tt.want, diff)
			}
		})
	}
}

func TestGitLabSetFailed(t *testing.T) {
	for _, tt := range setTestCases(failed, creating) {
		t.Run(tt.name, func(t *testing.T) {
			tt.gitlab.SetFailed("foo", "bar")
			if diff := deep.Equal(tt.gitlab.Status, tt.want); diff != nil {
				t.Errorf("SetFailed() = %v, want %v\n%s", tt.gitlab.Status, tt.want, diff)
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
			if diff := deep.Equal(got, tt.want); diff != nil {
				t.Errorf("GitLab.GetEndpoint() = %v, want %v\n%s", got, tt.want, diff)
			}
		})
	}
}

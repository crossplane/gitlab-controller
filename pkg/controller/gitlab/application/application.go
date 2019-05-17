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

package application

import (
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	xpworkloadv1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/workload/v1alpha1"
)

// A SecretMap associates resources with the secrets they depend upon.
type SecretMap map[corev1.ObjectReference][]string

// Add secret dependencies.
func (s SecretMap) Add(u *unstructured.Unstructured, secretNames ...string) {
	ref := s.ref(u)
	if _, ok := s[ref]; !ok {
		s[ref] = make([]string, 0, len(secretNames))
	}
	s[ref] = append(s[ref], secretNames...)
}

// Get secret dependencies.
func (s SecretMap) Get(u *unstructured.Unstructured) []string {
	return s[s.ref(u)]
}

func (s SecretMap) ref(u *unstructured.Unstructured) corev1.ObjectReference {
	return corev1.ObjectReference{
		APIVersion: u.GetAPIVersion(),
		Kind:       u.GetKind(),
		Namespace:  u.GetNamespace(),
		Name:       u.GetName(),
	}
}

type options struct {
	namespace   string
	labels      map[string]string
	annotations map[string]string
	cluster     *metav1.LabelSelector
	owners      []metav1.OwnerReference
	secrets     SecretMap
}

// Templates used to create and manage resources on a remote KubernetesCluster.
type Templates []*unstructured.Unstructured

// Option configures a Producer.
type Option func(*options)

// WithNamespace configures the namespace of the produced application.
func WithNamespace(namespace string) Option {
	return func(o *options) {
		o.namespace = namespace
	}
}

// WithLabels configures the labels of the produced application.
func WithLabels(labels map[string]string) Option {
	return func(o *options) {
		o.labels = labels
	}
}

// WithAnnotations configures the annotations of the produced application.
func WithAnnotations(annotations map[string]string) Option {
	return func(o *options) {
		o.annotations = annotations
	}
}

// WithClusterSelector configures the cluster selector of the produced
// application. Applications will select all clusters by default.
func WithClusterSelector(s *metav1.LabelSelector) Option {
	return func(o *options) {
		o.cluster = s
	}
}

// WithControllerReference configures the controller reference of the produced
// application. The supplied owner reference must not be nil.
func WithControllerReference(c *metav1.OwnerReference) Option {
	return func(o *options) {
		o.owners = []metav1.OwnerReference{*c}
	}
}

// WithSecretMap configures the secret dependencies of the produced applciation.
func WithSecretMap(m SecretMap) Option {
	return func(o *options) {
		o.secrets = m
	}
}

// New returns a KubernetesApplication that will manage resources based on the
// supplied templates.
func New(name string, ts Templates, o ...Option) *xpworkloadv1alpha1.KubernetesApplication {
	opts := &options{
		namespace: corev1.NamespaceDefault,
		cluster:   &metav1.LabelSelector{}, // The empty selector selects all clusters.
	}

	for _, apply := range o {
		apply(opts)
	}

	a := &xpworkloadv1alpha1.KubernetesApplication{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:       opts.namespace,
			Name:            name,
			Labels:          opts.labels,
			Annotations:     opts.annotations,
			OwnerReferences: opts.owners,
		},
		Spec: xpworkloadv1alpha1.KubernetesApplicationSpec{
			ResourceSelector:  &metav1.LabelSelector{MatchLabels: opts.labels},
			ClusterSelector:   opts.cluster,
			ResourceTemplates: make([]xpworkloadv1alpha1.KubernetesApplicationResourceTemplate, len(ts)),
		},
	}

	for i, t := range ts {
		secrets := opts.secrets.Get(t)
		rt := xpworkloadv1alpha1.KubernetesApplicationResourceTemplate{
			ObjectMeta: metav1.ObjectMeta{
				// TODO(negz): Handle the case in which we have templates for
				// two resources with the same kind and name but different API
				// versions. The below format string will result in a name
				// conflict.
				Name:        strings.ToLower(fmt.Sprintf("%s-%s-%s", name, t.GetKind(), t.GetName())),
				Labels:      opts.labels,
				Annotations: opts.annotations,
			},
			Spec: xpworkloadv1alpha1.KubernetesApplicationResourceSpec{
				Template: t,
				Secrets:  make([]corev1.LocalObjectReference, len(secrets)),
			},
		}

		for i, name := range secrets {
			rt.Spec.Secrets[i] = corev1.LocalObjectReference{Name: name}
		}

		a.Spec.ResourceTemplates[i] = rt
	}

	return a
}

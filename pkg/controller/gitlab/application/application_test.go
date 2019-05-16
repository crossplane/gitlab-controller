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
	"testing"

	xpworkloadv1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/workload/v1alpha1"
	"github.com/google/go-cmp/cmp"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

const (
	name      = "coolapp"
	namespace = "coolstuff"
)

var (
	labels          = map[string]string{"cool": "very"}
	annotations     = map[string]string{"cool.io/enabled": "true"}
	clusterSelector = &metav1.LabelSelector{MatchLabels: map[string]string{"isitacluster": "yes"}}
)

var (
	// It doesn't make sense for a KubernetesApplication to own another
	// KubernetesApplication, but we don't really care in the context of this
	// test.
	owner    = &xpworkloadv1alpha1.KubernetesApplication{}
	ownerRef = metav1.NewControllerRef(owner, xpworkloadv1alpha1.KubernetesApplicationGroupVersionKind)

	service = &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "v1",
		"kind":       "Service",
		"metadata":   map[string]interface{}{"name": name},
		"spec": map[string]interface{}{
			"ports": []interface{}{
				map[string]interface{}{
					"name":       "http",
					"port":       80,
					"protocol":   "TCP",
					"targetPort": "http",
				},
			},
			"selector": map[string]interface{}{
				"app.kubernetes.io/instance": name,
				"app.kubernetes.io/name":     "simple",
			},
			"type": "ClusterIP",
		},
	}}

	ingress = &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "extensions/v1beta1",
		"kind":       "Ingress",
		"metadata":   map[string]interface{}{"name": name},
		"spec": map[string]interface{}{
			"rules": []interface{}{
				map[string]interface{}{"host": "chart-example.local"},
			},
		},
	}}

	secrets   = []corev1.LocalObjectReference{{Name: "coolSecret"}, {Name: "coolestSecret"}}
	secretMap = func() SecretMap {
		m := SecretMap{}
		for _, s := range secrets {
			m.Add(service, s.Name)
		}
		return m
	}()
)

func TestProduce(t *testing.T) {
	type args struct {
		name string
		ts   Templates
		opts []Option
	}
	cases := []struct {
		name string
		args args
		want *xpworkloadv1alpha1.KubernetesApplication
	}{
		{
			name: "SimpleApplication",
			args: args{
				name: name,
				ts:   Templates{service, ingress},
				opts: []Option{
					WithNamespace(namespace),
					WithLabels(labels),
					WithAnnotations(annotations),
					WithClusterSelector(clusterSelector),
					WithControllerReference(ownerRef),
				},
			},
			want: &xpworkloadv1alpha1.KubernetesApplication{
				ObjectMeta: metav1.ObjectMeta{
					Namespace:       namespace,
					Name:            name,
					Labels:          labels,
					Annotations:     annotations,
					OwnerReferences: []metav1.OwnerReference{*ownerRef},
				},
				Spec: xpworkloadv1alpha1.KubernetesApplicationSpec{
					ClusterSelector:  clusterSelector,
					ResourceSelector: &metav1.LabelSelector{MatchLabels: labels},
					ResourceTemplates: []xpworkloadv1alpha1.KubernetesApplicationResourceTemplate{
						{
							ObjectMeta: metav1.ObjectMeta{
								Name:        name + "-service-" + name,
								Labels:      labels,
								Annotations: annotations,
							},
							Spec: xpworkloadv1alpha1.KubernetesApplicationResourceSpec{
								Template: service,
								Secrets:  []corev1.LocalObjectReference{},
							},
						},
						{
							ObjectMeta: metav1.ObjectMeta{
								Name:        name + "-ingress-" + name,
								Labels:      labels,
								Annotations: annotations,
							},
							Spec: xpworkloadv1alpha1.KubernetesApplicationResourceSpec{
								Template: ingress,
								Secrets:  []corev1.LocalObjectReference{},
							},
						},
					},
				},
			},
		},
		{
			name: "WithoutOptions",
			args: args{
				name: name,
				ts:   Templates{service},
			},
			want: &xpworkloadv1alpha1.KubernetesApplication{
				ObjectMeta: metav1.ObjectMeta{
					// Note this is set to default.
					Namespace: metav1.NamespaceDefault,
					Name:      name,
				},
				Spec: xpworkloadv1alpha1.KubernetesApplicationSpec{
					ResourceSelector: &metav1.LabelSelector{},
					ClusterSelector:  &metav1.LabelSelector{},
					ResourceTemplates: []xpworkloadv1alpha1.KubernetesApplicationResourceTemplate{{
						ObjectMeta: metav1.ObjectMeta{Name: name + "-service-" + name},
						Spec: xpworkloadv1alpha1.KubernetesApplicationResourceSpec{
							Template: service,
							Secrets:  []corev1.LocalObjectReference{},
						},
					}},
				},
			},
		},
		{
			name: "WithSecrets",
			args: args{
				name: name,
				ts:   Templates{service, ingress},
				opts: []Option{WithSecretMap(secretMap)},
			},
			want: &xpworkloadv1alpha1.KubernetesApplication{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: metav1.NamespaceDefault,
					Name:      name,
				},
				Spec: xpworkloadv1alpha1.KubernetesApplicationSpec{
					ResourceSelector: &metav1.LabelSelector{},
					ClusterSelector:  &metav1.LabelSelector{},
					ResourceTemplates: []xpworkloadv1alpha1.KubernetesApplicationResourceTemplate{
						{
							ObjectMeta: metav1.ObjectMeta{Name: name + "-service-" + name},
							Spec: xpworkloadv1alpha1.KubernetesApplicationResourceSpec{
								Template: service,
								Secrets:  secrets,
							},
						},
						{
							// The ingress has no secrets.
							ObjectMeta: metav1.ObjectMeta{Name: name + "-ingress-" + name},
							Spec: xpworkloadv1alpha1.KubernetesApplicationResourceSpec{
								Template: ingress,
								Secrets:  []corev1.LocalObjectReference{},
							},
						},
					},
				},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := New(tc.args.name, tc.args.ts, tc.args.opts...)

			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("New(...): -want, +got: %s", diff)
			}
		})
	}
}

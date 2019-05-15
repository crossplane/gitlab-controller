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

package helm

import (
	"bytes"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/helm/pkg/chartutil"

	"github.com/crossplaneio/gitlab-controller/pkg/test"
)

const (
	name = "cool"

	chartName = "simple"
	chartDir  = "fixtures/" + chartName
	chartPath = "/" + chartName + "-1.0.0.tgz"
)

func TestProduce(t *testing.T) {
	type want struct {
		resources []*unstructured.Unstructured
		err       error
	}
	cases := []struct {
		name     string
		chartDir string
		opts     []Option
		want     want
	}{
		{
			name:     "IngressEnabled",
			chartDir: chartDir,
			opts: []Option{
				WithValues(chartutil.Values{
					// The ingress is disabled by default.
					"ingress": map[string]bool{"enabled": true},
				}),
			},
			want: want{
				resources: []*unstructured.Unstructured{
					{
						Object: map[string]interface{}{
							"apiVersion": "v1",
							"kind":       "Service",
							"metadata":   map[string]interface{}{"name": chartName},
							"spec": map[string]interface{}{
								"ports": []interface{}{
									map[string]interface{}{
										"name":       "http",
										"port":       int64(80),
										"protocol":   "TCP",
										"targetPort": "http",
									},
								},
								"selector": map[string]interface{}{
									"app.kubernetes.io/instance": chartName,
									"app.kubernetes.io/name":     chartName,
								},
								"type": "ClusterIP",
							},
						},
					},
					{
						Object: map[string]interface{}{
							"apiVersion": "extensions/v1beta1",
							"kind":       "Ingress",
							"metadata":   map[string]interface{}{"name": chartName},
							"spec": map[string]interface{}{
								"rules": []interface{}{
									map[string]interface{}{"host": "chart-example.local"},
								},
							},
						},
					},
				},
			},
		},
		{
			name:     "IngressDisabled",
			chartDir: chartDir,
			// The ingress is disabled by default, so we omit values.
			opts: []Option{WithReleaseName(name)},
			want: want{
				resources: []*unstructured.Unstructured{
					{
						Object: map[string]interface{}{
							"apiVersion": "v1",
							"kind":       "Service",
							"metadata":   map[string]interface{}{"name": name + "-" + chartName},
							"spec": map[string]interface{}{
								"ports": []interface{}{
									map[string]interface{}{
										"name":       "http",
										"port":       int64(80),
										"protocol":   "TCP",
										"targetPort": "http",
									},
								},
								"selector": map[string]interface{}{
									"app.kubernetes.io/instance": name,
									"app.kubernetes.io/name":     chartName,
								},
								"type": "ClusterIP",
							},
						},
					},
				},
			},
		},
		{
			name:     "WithReleaseName",
			chartDir: chartDir,
			opts:     []Option{WithReleaseName(name)},
			want: want{
				resources: []*unstructured.Unstructured{
					{
						Object: map[string]interface{}{
							"apiVersion": "v1",
							"kind":       "Service",
							"metadata":   map[string]interface{}{"name": name + "-" + chartName},
							"spec": map[string]interface{}{
								"ports": []interface{}{
									map[string]interface{}{
										"name":       "http",
										"port":       int64(80),
										"protocol":   "TCP",
										"targetPort": "http",
									},
								},
								"selector": map[string]interface{}{
									"app.kubernetes.io/instance": name,
									"app.kubernetes.io/name":     chartName,
								},
								"type": "ClusterIP",
							},
						},
					},
				},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tarball, err := targz(tc.chartDir)
			if err != nil {
				t.Fatalf("targz(%s): %s", tc.chartDir, err)
			}

			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				tarball.WriteTo(w)
				r.Body.Close()
			}))
			defer ts.Close()

			resources, err := Render(ts.URL+chartPath, tc.opts...)
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Fatalf("Render(...): -want error, +got error: %s", diff)
			}
			if diff := cmp.Diff(tc.want.resources, resources); diff != "" {
				t.Errorf("Render(...): -want resources, +got resources: %s", diff)
			}
		})
	}
}

// targz uses Helm's chart packaging logic to package a chart directory into a
// gzipped tarball. It writes the tarball out to a temporary file because Helm's
// chartutil functions are hardwired to read and write to the OS filesystem.
func targz(chartDir string) (io.WriterTo, error) {
	path, err := filepath.Abs(chartDir)
	if err != nil {
		return nil, errors.Wrapf(err, "cannot determine absolute path of %s", chartDir)
	}

	c, err := chartutil.LoadDir(path)
	if err != nil {
		return nil, errors.Wrapf(err, "cannot load %s", path)
	}

	tmp, err := ioutil.TempDir(os.TempDir(), "chart")
	if err != nil {
		return nil, errors.Wrap(err, "cannot create temp dir")
	}
	defer os.RemoveAll(tmp)

	f, err := chartutil.Save(c, tmp)
	if err != nil {
		return nil, errors.Wrap(err, "cannot save chart tarball")
	}

	tarball, err := os.Open(f)
	if err != nil {
		return nil, errors.Wrap(err, "cannot open chart tarball")
	}
	defer tarball.Close()

	b := &bytes.Buffer{}
	if _, err := b.ReadFrom(tarball); err != nil {
		return nil, errors.Wrap(err, "cannot read chart tarball")
	}

	return b, nil
}

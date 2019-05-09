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
	"fmt"
	"io"
	"net/url"
	"path"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/helm/pkg/chartutil"
	"k8s.io/helm/pkg/getter"
	"k8s.io/helm/pkg/manifest"
	"k8s.io/helm/pkg/proto/hapi/chart"
	"k8s.io/helm/pkg/renderutil"
	"k8s.io/helm/pkg/tiller"
	"k8s.io/helm/pkg/timeconv"
)

const (
	// These are returned as during rendering, but will almost certainly not
	// contain valid YAML or JSON serialized Kubernetes objects.
	// https://github.com/helm/helm/blob/581e6cd/cmd/helm/template.go#L232
	notesFile        = "NOTES.txt"
	ignoreFilePrefix = "_"

	repoCert, repokey, repoCA = "", "", ""

	// Copied from yaml.NewYAMLOrJSONDecoder tests ¯\_(ツ)_/¯
	decodeBuffer = 100
)

type options struct {
	name      string
	namespace string
	values    string
}

// Option configures a ResourceProducer.
type Option func(*options) error

// WithReleaseName configures the Helm release name used to render resources.
func WithReleaseName(name string) Option {
	return func(o *options) error {
		o.name = name
		return nil
	}
}

// WithValues configures the Helm values used to render resources.
func WithValues(v chartutil.Values) Option {
	return func(o *options) error {
		var err error
		o.values, err = v.YAML()
		return errors.Wrap(err, "cannot marshal values to YAML")
	}
}

// Render a Helm chart as a slice of Unstructured objects. The supplied Helm
// chart URL must be an HTTP or HTTPS URL trusted by the system.
func Render(chartURL string, o ...Option) ([]*unstructured.Unstructured, error) {
	opts := &options{
		name:      strings.Split(path.Base(chartURL), "-")[0],
		namespace: corev1.NamespaceDefault,
	}

	for _, apply := range o {
		if err := apply(opts); err != nil {
			return nil, errors.Wrap(err, "cannot apply option")
		}
	}

	u, err := url.Parse(chartURL)
	if err != nil {
		return nil, errors.Wrap(err, "invalid chart URL format")
	}

	g, err := getter.NewHTTPGetter(u.String(), repoCert, repokey, repoCA)
	if err != nil {
		return nil, errors.Wrap(err, "cannot create chart getter")
	}

	tarball, err := g.Get(u.String())
	if err != nil {
		return nil, errors.Wrapf(err, "cannot download Helm chart %s", chartURL)
	}

	c, err := chartutil.LoadArchive(tarball)
	if err != nil {
		return nil, errors.Wrapf(err, "cannot load Helm chart %s", chartURL)
	}

	ro := renderutil.Options{
		ReleaseOptions: chartutil.ReleaseOptions{
			// Namespace can be set here, but it seems to be ignored.
			// https://github.com/helm/helm/issues/3553
			Name: opts.name,
			Time: timeconv.Now(),
		},
		KubeVersion: fmt.Sprintf("%s.%s", chartutil.DefaultKubeVersion.Major, chartutil.DefaultKubeVersion.Minor),
	}

	config := &chart.Config{Raw: opts.values}
	m, err := renderutil.Render(c, config, ro)
	if err != nil {
		return nil, errors.Wrapf(err, "cannot render Helm chart %s", chartURL)
	}

	manifests := tiller.SortByKind(manifest.SplitManifests(m))
	resources, err := parse(manifests)
	return resources, errors.Wrapf(err, "cannot parse rendered Helm chart %s", chartURL)

}

func parse(manifests []manifest.Manifest) ([]*unstructured.Unstructured, error) {
	resources := make([]*unstructured.Unstructured, 0, len(manifests))

	for _, m := range manifests {
		b := filepath.Base(m.Name)
		if b == notesFile || strings.HasPrefix(b, ignoreFilePrefix) {
			// This is a special Helm file.
			continue
		}

		// Each manifest could either be a JSON document, a YAML document, or a
		// stream of YAML documents.
		d := yaml.NewYAMLOrJSONDecoder(strings.NewReader(m.Content), decodeBuffer)
		for {
			u := &unstructured.Unstructured{}
			if err := d.Decode(u); err == io.EOF {
				break
			} else if err != nil {
				return nil, errors.Wrapf(err, "cannot unmarshal Helm manifest %s", m.Name)
			}
			// It's common for the stream of YAML documents produced during
			// chart rendering to contain several empty documents resulting from
			// templates whose entire content can be omitted based on a values
			// derived conditional.
			if len(u.UnstructuredContent()) == 0 {
				continue
			}
			resources = append(resources, u)
		}
	}

	return resources, nil
}

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

package values

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"k8s.io/helm/pkg/chartutil"

	"github.com/crossplaneio/gitlab-controller/pkg/test"
)

func TestMerge(t *testing.T) {
	cases := []struct {
		name    string
		dst     chartutil.Values
		src     chartutil.Values
		depth   int
		want    chartutil.Values
		wantErr error
	}{
		{
			name:  "Simple",
			dst:   chartutil.Values{"global": chartutil.Values{"cool": "very", "rad": "somewhat"}},
			src:   chartutil.Values{"global": chartutil.Values{"rad": "very"}, "local": "cool"},
			depth: defaultMaxDepth,
			want:  chartutil.Values{"global": chartutil.Values{"cool": "very", "rad": "very"}, "local": "cool"},
		},
		{
			name:  "EmptyDest",
			dst:   chartutil.Values{},
			src:   chartutil.Values{"global": chartutil.Values{"cool": "very"}},
			depth: defaultMaxDepth,
			want:  chartutil.Values{"global": chartutil.Values{"cool": "very"}},
		},
		{
			name:  "EmptySource",
			dst:   chartutil.Values{"global": chartutil.Values{"cool": "very"}},
			src:   chartutil.Values{},
			depth: defaultMaxDepth,
			want:  chartutil.Values{"global": chartutil.Values{"cool": "very"}},
		},
		{
			name:    "TooDeep",
			dst:     chartutil.Values{"global": chartutil.Values{"cool": "very", "rad": chartutil.Values{"too": "deep"}}},
			src:     chartutil.Values{"global": chartutil.Values{"cool": "super", "rad": chartutil.Values{"too": "deep"}}},
			want:    chartutil.Values{"global": chartutil.Values{"cool": "very", "rad": chartutil.Values{"too": "deep"}}},
			depth:   2,
			wantErr: ErrMaxDepthExceeded,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gotErr := Merge(tc.dst, tc.src, WithMaxDepth(tc.depth))

			if diff := cmp.Diff(tc.wantErr, gotErr, test.EquateErrors()); diff != "" {
				t.Fatalf("Merge(...): -want error, +got error:\n%s", diff)
			}

			if diff := cmp.Diff(tc.want, tc.dst); diff != "" {
				t.Fatalf("Merge(...): -want != +got:\n%s", diff)
			}
		})
	}
}

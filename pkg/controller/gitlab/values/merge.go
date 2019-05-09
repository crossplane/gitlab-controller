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

// This file is shamelessly adapted from the below package.
// https://github.com/peterbourgon/mergemap/blob/ed50b1a/mergemap.go

import (
	"reflect"

	"github.com/pkg/errors"
	"k8s.io/helm/pkg/chartutil"
)

const defaultMaxDepth = 32

// ErrMaxDepthExceeded is returned when we recurse too deep.
var ErrMaxDepthExceeded = errors.New("exceeded maximum recursion depth while merging Helm values")

// A MergeOption configures merge semantics.
type MergeOption func(*merger)

// WithMaxDepth configures how deep the merge will recurse before returning an
// error.
func WithMaxDepth(max int) MergeOption {
	return func(m *merger) {
		m.depth = max
	}
}

// Merge recursively merges the src and dst maps. Key conflicts are resolved by
// preferring src, or recursively descending, if both src and dst are maps. It
// returns an error if and only if it reaches its max recursion depth.
func Merge(dst, src chartutil.Values, o ...MergeOption) error {
	m := &merger{depth: defaultMaxDepth}
	for _, mo := range o {
		mo(m)
	}
	_, err := m.merge(dst, src)
	return err
}

type merger struct {
	depth int
}

func (m *merger) merge(dst, src chartutil.Values) (chartutil.Values, error) {
	if m.depth == 0 {
		return nil, ErrMaxDepthExceeded
	}

	for key, srcVal := range src {
		if dstVal, ok := dst[key]; ok {
			srcMap, srcMapOk := mapify(srcVal)
			dstMap, dstMapOk := mapify(dstVal)
			if srcMapOk && dstMapOk {
				var err error
				m.depth--
				srcVal, err = m.merge(dstMap, srcMap)
				if err != nil {
					return nil, err
				}
			}
		}
		dst[key] = srcVal
	}
	return dst, nil
}

func mapify(i interface{}) (chartutil.Values, bool) {
	value := reflect.ValueOf(i)
	if value.Kind() == reflect.Map {
		m := chartutil.Values{}
		for _, k := range value.MapKeys() {
			m[k.String()] = value.MapIndex(k).Interface()
		}
		return m, true
	}
	return chartutil.Values{}, false
}

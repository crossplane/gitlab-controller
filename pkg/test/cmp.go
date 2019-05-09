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

package test

import (
	"reflect"

	xpcorev1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/core/v1alpha1"
	"github.com/google/go-cmp/cmp"
)

// TODO(negz): Replace this if a similar option is added to cmpopts per
// https://github.com/google/go-cmp/issues/89

// EquateErrors returns true if the supplied errors are of the same type and
// produce identical strings. This mirrors the error comparison behaviour of
// https://github.com/go-test/deep, which most Crossplane tests targeted before
// we switched to go-cmp.
func EquateErrors() cmp.Option {
	return cmp.Comparer(func(a, b error) bool {
		if a == nil || b == nil {
			return a == nil && b == nil
		}

		av := reflect.ValueOf(a)
		bv := reflect.ValueOf(b)
		if av.Type() != bv.Type() {
			return false
		}

		return a.Error() == b.Error()
	})
}

// TODO(negz): Implement this as an Equal() method on ConditionedStatus.
// per https://godoc.org/github.com/google/go-cmp/cmp

// EqualConditionedStatus checks ConditionedStatus for equality ignoring the
// actual order of the conditions
func EqualConditionedStatus(x, y xpcorev1alpha1.ConditionedStatus) bool {
	if len(x.Conditions) != len(y.Conditions) {
		return false
	}
	for _, xc := range x.Conditions {
		yc := y.Condition(xc.Type)
		if yc == nil || !xc.Equal(*yc) {
			return false
		}
	}
	return true
}

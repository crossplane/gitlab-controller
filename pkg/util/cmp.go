/*
Copyright 2018 The GitLab-Controller Authors.

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

package util

import xpcorev1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/core/v1alpha1"

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

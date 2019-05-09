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

package test

import (
	"testing"

	xpcorev1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/core/v1alpha1"
)

func TestEqualConditionedStatus(t *testing.T) {
	readyFailed := NewConditionedStatusBuilder().WithReadyCondition().WithFailedCondition("foo", "bar").Build()
	failedReady := NewConditionedStatusBuilder().WithFailedCondition("foo", "bar").WithReadyCondition().Build()
	failedUnset := NewConditionedStatusBuilder().WithFailedCondition("foo", "bar").WithReadyCondition().
		WithUnsetCondition(xpcorev1alpha1.Failed).Build()
	type args struct {
		x xpcorev1alpha1.ConditionedStatus
		y xpcorev1alpha1.ConditionedStatus
	}
	tests := map[string]struct {
		args args
		want bool
	}{
		"Default": {
			args: args{x: xpcorev1alpha1.ConditionedStatus{}, y: xpcorev1alpha1.ConditionedStatus{}},
			want: true,
		},
		"DifferentLength": {
			args: args{x: xpcorev1alpha1.ConditionedStatus{}, y: *readyFailed},
			want: false,
		},
		"Same": {
			args: args{x: *readyFailed, y: *readyFailed},
			want: true,
		},
		"SameDifferentOrder": {
			args: args{x: *readyFailed, y: *failedReady},
			want: true,
		},
		"Different": {
			args: args{x: *failedReady, y: *failedUnset},
			want: false,
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			if got := EqualConditionedStatus(tt.args.x, tt.args.y); got != tt.want {
				t.Errorf("EqualConditionedStatus() = %v, want %v", got, tt.want)
			}
		})
	}
}

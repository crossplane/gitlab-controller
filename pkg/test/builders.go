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
	xpcorev1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/core/v1alpha1"
)

// ConditionedStatusBuilder creates new ConditionedStatus
type ConditionedStatusBuilder struct {
	*xpcorev1alpha1.ConditionedStatus
}

// NewConditionedStatusBuilder - new builder
func NewConditionedStatusBuilder() *ConditionedStatusBuilder {
	return &ConditionedStatusBuilder{ConditionedStatus: &xpcorev1alpha1.ConditionedStatus{}}
}

// Build returns new ConditionedStatus
func (b *ConditionedStatusBuilder) Build() *xpcorev1alpha1.ConditionedStatus {
	return b.ConditionedStatus
}

// WithFailedCondition sets and activates failed condition
func (b *ConditionedStatusBuilder) WithFailedCondition(r, m string) *ConditionedStatusBuilder {
	b.SetFailed(r, m)
	return b
}

// WithReadyCondition sets and activates ready condition
func (b *ConditionedStatusBuilder) WithReadyCondition() *ConditionedStatusBuilder {
	b.SetReady()
	return b
}

// WithUnsetCondition deactivates condition of given type
func (b *ConditionedStatusBuilder) WithUnsetCondition(c xpcorev1alpha1.ConditionType) *ConditionedStatusBuilder {
	b.UnsetCondition(c)
	return b
}

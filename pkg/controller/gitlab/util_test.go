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

package gitlab

import (
	"testing"

	"github.com/go-test/deep"
	"github.com/google/go-cmp/cmp"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type invalidObject struct{}

func (i *invalidObject) MarshalJSON() ([]byte, error)   { return nil, errors.New("test-error") }
func (*invalidObject) GetObjectKind() schema.ObjectKind { return schema.EmptyObjectKind }
func (*invalidObject) DeepCopyObject() runtime.Object   { return nil }

var _ runtime.Object = &invalidObject{}

func Test_convert(t *testing.T) {
	type want struct {
		obj *unstructured.Unstructured
		err error
	}
	tests := map[string]struct {
		args runtime.Object
		want want
	}{
		"EmptyConfigmap": {
			args: &corev1.ConfigMap{},
			want: want{
				obj: &unstructured.Unstructured{
					Object: map[string]interface{}{
						"metadata": map[string]interface{}{
							"creationTimestamp": nil,
						},
					},
				},
			},
		},
		"Secret Service": {
			args: &corev1.Secret{
				ObjectMeta: testMeta,
				Data: map[string][]byte{
					"foo": []byte("bar"),
				},
			},
			want: want{
				obj: &unstructured.Unstructured{
					Object: map[string]interface{}{
						"metadata": map[string]interface{}{
							"namespace":         testNamespace,
							"name":              testName,
							"creationTimestamp": nil,
						},
						"data": map[string]interface{}{
							"foo": "YmFy", // YmFy is base64 encoded "bar"
						},
					},
				},
			},
		},
		"Invalid": {
			args: &invalidObject{},
			want: want{
				err: errors.New("test-error"),
			},
		},
	}
	testCaseName := "convert()"
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			got, err := convert(tt.args)
			if diff := deep.Equal(err, tt.want.err); diff != nil {
				t.Errorf("%s error %s", testCaseName, diff)
				return
			}
			if diff := cmp.Diff(got, tt.want.obj); diff != "" {
				t.Errorf("%s %s", testCaseName, diff)
			}
		})
	}
}

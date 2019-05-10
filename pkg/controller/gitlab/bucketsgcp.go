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
	"fmt"

	xpcorev1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/core/v1alpha1"
	"github.com/pkg/errors"
	"gopkg.in/yaml.v2"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/json"
)

// GCP Secret Handlers
const (
	gcpProvider       = "Google"
	gcpS3CmdConfigFmt = `[default]
host_base = storage.googleapis.com
host_bucket = storage.googleapis.com
use_https = True
signature_v2 = True
enable_multipart = False
access_key = %s
secret_key = %s`
)

type gcpSecretConnectionCreator struct{}

func (c *gcpSecretConnectionCreator) create(s *corev1.Secret) error {
	if s == nil || len(s.Data) == 0 {
		return errors.New(errorMsgEmptyConnectionSecret)
	}
	var creds = &struct {
		ProjectID string `json:"project_id"`
		Email     string `json:"client_email"`
	}{}
	data := s.Data[xpcorev1alpha1.ResourceCredentialsTokenKey]
	if err := json.Unmarshal(data, creds); err != nil {
		return errors.Wrapf(err, errorFmtFailedToParse, xpcorev1alpha1.ResourceCredentialsTokenKey)
	}

	connection := &struct {
		Provider string `yaml:"provider"`
		Project  string `yaml:"google_project"`
		Email    string `yaml:"google_client_email"`
		Key      string `yaml:"google_json_key"`
	}{
		Provider: gcpProvider,
		Project:  creds.ProjectID,
		Email:    creds.Email,
		Key:      string(s.Data[xpcorev1alpha1.ResourceCredentialsTokenKey]),
	}

	yamlData, err := yaml.Marshal(connection)
	if err != nil {
		return errors.Wrapf(err, errorFmtFailedToSave, "connection")
	}

	s.Data[connectionKey] = yamlData
	return nil
}

type gcpSecretS3CmdConfigCreator struct{}

func (c *gcpSecretS3CmdConfigCreator) create(s *corev1.Secret) error {
	if s == nil || len(s.Data) == 0 {
		return errors.New(errorMsgEmptyConnectionSecret)
	}

	accessKey := s.Data[xpcorev1alpha1.ResourceCredentialsSecretUserKey]
	secretKey := s.Data[xpcorev1alpha1.ResourceCredentialsSecretPasswordKey]

	s.Data[configKey] = []byte(fmt.Sprintf(gcpS3CmdConfigFmt, accessKey, secretKey))
	return nil
}

type gcpSecretUpdater struct {
	connection secretDataCreator
	config     secretDataCreator
}

func newGCPSecretUpdater() *gcpSecretUpdater {
	return &gcpSecretUpdater{
		connection: &gcpSecretConnectionCreator{},
		config:     &gcpSecretS3CmdConfigCreator{},
	}
}

func (u *gcpSecretUpdater) update(s *corev1.Secret) error {
	if err := u.connection.create(s); err != nil {
		return errors.Wrapf(err, errorFailedToCreateConnectionData)
	}
	return errors.Wrapf(u.config.create(s), errorFailedToCreateConfigData)
}

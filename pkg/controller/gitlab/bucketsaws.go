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
	corev1 "k8s.io/api/core/v1"
)

// AWS Secret Handlers
const (
	awsConnectionFmt = `provider: AWS
region: us-east-1
aws_access_key_id: %s
aws_secret_access_key: %s
`
	awsS3CmdConfigFmt = ` [default]
access_key = %s
secret_key = %s
bucket_location = us-east-1`
)

type awsSecretConnectionCreator struct{}

func (c *awsSecretConnectionCreator) create(s *corev1.Secret) error {
	if s == nil || len(s.Data) == 0 {
		return errors.New(errorMsgEmptyConnectionSecret)
	}

	accessKey := s.Data[xpcorev1alpha1.ResourceCredentialsSecretUserKey]
	secretKey := s.Data[xpcorev1alpha1.ResourceCredentialsSecretPasswordKey]

	s.Data[connectionKey] = []byte(fmt.Sprintf(awsConnectionFmt, accessKey, secretKey))
	return nil
}

type awsSecretS3CmdConfigCreator struct{}

func (c *awsSecretS3CmdConfigCreator) create(s *corev1.Secret) error {
	if s == nil || len(s.Data) == 0 {
		return errors.New(errorMsgEmptyConnectionSecret)
	}

	accessKey := s.Data[xpcorev1alpha1.ResourceCredentialsSecretUserKey]
	secretKey := s.Data[xpcorev1alpha1.ResourceCredentialsSecretPasswordKey]

	s.Data[configKey] = []byte(fmt.Sprintf(awsS3CmdConfigFmt, accessKey, secretKey))
	return nil
}

type awsSecretUpdater struct {
	connection secretDataCreator
	config     secretDataCreator
}

func newAWSSecretUpdater() *awsSecretUpdater {
	return &awsSecretUpdater{
		connection: &awsSecretConnectionCreator{},
		config:     &awsSecretS3CmdConfigCreator{},
	}
}

func (u *awsSecretUpdater) update(s *corev1.Secret) error {
	if err := u.connection.create(s); err != nil {
		return errors.Wrapf(err, errorFailedToCreateConnectionData)
	}
	return errors.Wrapf(u.config.create(s), errorFailedToCreateConfigData)
}

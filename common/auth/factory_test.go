/**
 * Copyright 2021 IBM Corp.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

// Package auth ...
package auth

import (
	"os"
	"path/filepath"
	"testing"

	vpcconfig "github.com/IBM/ibmcloud-volume-file-vpc/file/vpcconfig"
	"github.com/IBM/ibmcloud-volume-interface/config"
	"github.com/IBM/secret-utils-lib/pkg/k8s_utils"
	"github.com/stretchr/testify/assert"
)

func TestNewVPCFileContextCredentialsFactory(t *testing.T) {
	conf := &vpcconfig.VPCFileConfig{
		VPCConfig: &config.VPCProviderConfig{
			Enabled:                    true,
			EndpointURL:                "test-iam-url",
			VPCTimeout:                 "30s",
			IamClientID:                "test-iam_client_id",
			IamClientSecret:            "test-iam_client_secret",
			IKSTokenExchangePrivateURL: "https://us-south.containers.cloud.ibm.com",
		},
	}

	kc, _ := k8s_utils.FakeGetk8sClientSet()
	pwd, _ := os.Getwd()
	file := filepath.Join(pwd, "..", "..", "etc", "libconfig.toml")
	_ = k8s_utils.FakeCreateSecret(kc, "DEFAULT", file)
	_, err := NewVPCContextCredentialsFactory(conf, &kc)
	assert.Nil(t, err)

	//Invalid libconfig file
	kc, _ = k8s_utils.FakeGetk8sClientSet()
	pwd, _ = os.Getwd()
	file = filepath.Join(pwd, "..", "..", "etc", "libconfigxyz.toml")
	_ = k8s_utils.FakeCreateSecret(kc, "DEFAULT", file)
	_, err = NewVPCContextCredentialsFactory(conf, &kc)
	assert.NotNil(t, err)
}

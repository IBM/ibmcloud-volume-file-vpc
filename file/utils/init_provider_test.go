/**
 * Copyright 2025 IBM Corp.
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

package utils

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	vpc_prov "github.com/IBM/ibmcloud-volume-file-vpc/file/provider"
	vpcconfig "github.com/IBM/ibmcloud-volume-file-vpc/file/vpcconfig"
	"github.com/IBM/ibmcloud-volume-interface/config"
	"github.com/IBM/ibmcloud-volume-interface/lib/provider"
	auth "github.com/IBM/ibmcloud-volume-interface/provider/auth"
	"github.com/IBM/ibmcloud-volume-interface/provider/local/fakes"
	"github.com/IBM/secret-utils-lib/pkg/k8s_utils"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

const (
	TestProviderAccessToken = "test-provider-access-token"
	TestIKSAccountID        = "test-iks-account"
)

func TestInitProviders(t *testing.T) {
	logger := zap.NewNop()
	vpcfileconf := &vpcconfig.VPCFileConfig{}
	k8sClient, _ := k8s_utils.FakeGetk8sClientSet()
	pwd, _ := os.Getwd()

	clusterConfPath := filepath.Join(pwd, "..", "..", "test-fixtures", "valid", "cluster_info", "cluster-config.json")
	_ = k8s_utils.FakeCreateCM(k8sClient, clusterConfPath)

	secretConfPath := filepath.Join(pwd, "..", "..", "test-fixtures", "slconfig.toml")
	_ = k8s_utils.FakeCreateSecret(k8sClient, "DEFAULT", secretConfPath)

	// Define test cases
	testCases := []struct {
		name           string
		expectedErrMsg string
		vpcConfig      *config.VPCProviderConfig
		iksConfig      *config.IKSConfig
		client         *k8s_utils.KubernetesClient
	}{
		{
			name:           "Both VPC and IKS providers are enabled",
			client:         &k8sClient,
			expectedErrMsg: "",
			// Setup test environment
			vpcConfig: &config.VPCProviderConfig{
				Enabled:       true,
				VPCVolumeType: "test-vpc-volume-type",
			},
			iksConfig: &config.IKSConfig{
				Enabled:             true,
				IKSFileProviderName: "test-iks-file-provider",
			},
		},
		{
			name:           "VPC provider is enabled",
			client:         &k8sClient,
			expectedErrMsg: "",
			// Setup test environment
			vpcConfig: &config.VPCProviderConfig{
				Enabled:       true,
				VPCVolumeType: "test-vpc-volume-type",
			},
			iksConfig: nil,
		},
		{
			name:   "No providers are enabled",
			client: &k8sClient,
			// Setup test environment
			vpcConfig:      nil,
			iksConfig:      nil,
			expectedErrMsg: "no providers registered",
		},
		{
			name: "pass nill k8s",
			// Setup test environment
			vpcConfig: &config.VPCProviderConfig{
				Enabled:       true,
				VPCVolumeType: "test-vpc-volume-type",
			},
			iksConfig:      nil,
			client:         nil,
			expectedErrMsg: "Description: Error initialising k8s client BackendError: validator: (nil *k8s_utils.KubernetesClient) ",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			vpcfileconf.VPCConfig = tc.vpcConfig
			vpcfileconf.IKSConfig = tc.iksConfig

			_, err := InitProviders(vpcfileconf, tc.client, logger)
			if tc.expectedErrMsg != "" {
				assert.EqualError(t, err, tc.expectedErrMsg)
			} else {
				assert.NoError(t, err)
			}

		})
	}
}

func TestOpenProviderSession(t *testing.T) {
	fakeProvider := &fakes.Provider{}
	ccf := &auth.ContextCredentialsFactory{}

	logger := zap.NewNop()
	k8sClient, _ := k8s_utils.FakeGetk8sClientSet()
	pwd, _ := os.Getwd()

	clusterConfPath := filepath.Join(pwd, "..", "..", "test-fixtures", "valid", "cluster_info", "cluster-config.json")
	_ = k8s_utils.FakeCreateCM(k8sClient, clusterConfPath)

	secretConfPath := filepath.Join(pwd, "..", "..", "test-fixtures", "slconfig.toml")
	_ = k8s_utils.FakeCreateSecret(k8sClient, "DEFAULT", secretConfPath)

	// Define test cases
	testCases := []struct {
		name                 string
		expectedErrMsg       string
		expectedCredenErrMsg string
		sessn                *vpc_prov.VPCSession
	}{
		{
			name: "Open session success",
			sessn: &vpc_prov.VPCSession{
				VPCAccountID: TestIKSAccountID,
				Config: &vpcconfig.VPCFileConfig{
					ServerConfig: &config.ServerConfig{
						DebugTrace: true,
					},
					IKSConfig: &config.IKSConfig{
						Enabled:             true,
						IKSFileProviderName: "test-vpc-volume-type",
					},
				},
				ContextCredentials: provider.ContextCredentials{
					AuthType:     provider.IAMAccessToken,
					Credential:   TestProviderAccessToken,
					IAMAccountID: TestIKSAccountID,
				},
				VolumeType: "vpc-share",
				Provider:   vpc_prov.VPC,
			},
		}, {
			name: "openSession fails",
			sessn: &vpc_prov.VPCSession{
				VPCAccountID: TestIKSAccountID,
				Config: &vpcconfig.VPCFileConfig{
					ServerConfig: &config.ServerConfig{
						DebugTrace: true,
					},
					IKSConfig: &config.IKSConfig{
						Enabled:             true,
						IKSFileProviderName: "test-vpc-volume-type",
					},
				},
				ContextCredentials: provider.ContextCredentials{
					AuthType:     provider.IAMAccessToken,
					Credential:   TestProviderAccessToken,
					IAMAccountID: TestIKSAccountID,
				},
				VolumeType: "vpc-share",
				Provider:   vpc_prov.VPC,
			},
			expectedErrMsg: "openssession fatal error",
		}, {
			name: "ContextCredentialsFactory fails",
			sessn: &vpc_prov.VPCSession{
				VPCAccountID: TestIKSAccountID,
				Config: &vpcconfig.VPCFileConfig{
					ServerConfig: &config.ServerConfig{
						DebugTrace: true,
					},
					IKSConfig: &config.IKSConfig{
						Enabled:             true,
						IKSFileProviderName: "test-vpc-volume-type",
					},
				},
				ContextCredentials: provider.ContextCredentials{
					AuthType:     provider.IAMAccessToken,
					Credential:   TestProviderAccessToken,
					IAMAccountID: TestIKSAccountID,
				},
				VolumeType: "vpc-share",
				Provider:   vpc_prov.VPC,
			},
			expectedCredenErrMsg: "ContextCredentialsFactory fatal error",
		}, {
			name: "ContextCredentialsFactory due to nil IKS config",
			sessn: &vpc_prov.VPCSession{
				VPCAccountID: TestIKSAccountID,
				Config: &vpcconfig.VPCFileConfig{
					ServerConfig: &config.ServerConfig{
						DebugTrace: true,
					},
					IKSConfig: nil,
				},
				ContextCredentials: provider.ContextCredentials{
					AuthType:     provider.IAMAccessToken,
					Credential:   TestProviderAccessToken,
					IAMAccountID: TestIKSAccountID,
				},
				VolumeType: "vpc-share",
				Provider:   vpc_prov.VPC,
			},
			expectedErrMsg: "Insufficient authentication credentials",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.expectedErrMsg != "" {
				//Error case openSession fails
				fakeProvider.OpenSessionReturns(tc.sessn, errors.New(tc.expectedErrMsg))
				fakeProvider.ContextCredentialsFactoryReturns(ccf, nil)
			} else if tc.expectedCredenErrMsg != "" {
				fakeProvider.OpenSessionReturns(tc.sessn, nil)
				fakeProvider.ContextCredentialsFactoryReturns(ccf, errors.New(tc.expectedCredenErrMsg))
			} else {
				fakeProvider.OpenSessionReturns(tc.sessn, nil)
				fakeProvider.ContextCredentialsFactoryReturns(ccf, nil)
			}

			registry, _ := InitProviders(tc.sessn.Config, &k8sClient, logger)
			_, fatal, err := OpenProviderSession(fakeProvider, tc.sessn.Config, registry, "test-vpc-volume-type", logger)

			if tc.expectedErrMsg != "" {
				assert.EqualError(t, err, tc.expectedErrMsg)
				assert.True(t, fatal)
			} else if tc.expectedCredenErrMsg != "" {
				assert.EqualError(t, err, tc.expectedCredenErrMsg)
				assert.True(t, fatal)
			} else {
				assert.NoError(t, err)
				assert.False(t, fatal)
			}

		})
	}
}

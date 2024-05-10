/**
 * Copyright 2024 IBM Corp.
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

// Package provider ...
package provider

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	vpcconfig "github.com/IBM/ibmcloud-volume-file-vpc/file/vpcconfig"
	"github.com/IBM/ibmcloud-volume-interface/config"
	"github.com/IBM/ibmcloud-volume-interface/provider/auth"
	"github.com/IBM/ibmcloud-volume-interface/provider/local"
	"github.com/IBM/secret-utils-lib/pkg/k8s_utils"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

const (
	TestProviderAccountID   = "test-provider-account"
	TestProviderAccessToken = "test-provider-access-token"
	TestIKSAccountID        = "test-iks-account"
	TestZone                = "test-zone"
	IamURL                  = "test-iam-url"
	IamClientID             = "test-iam_client_id"
	IamClientSecret         = "test-iam_client_secret"
	IamAPIKey               = "test-iam_api_key"
	RefreshToken            = "test-refresh_token"
	TestEndpointURL         = "http://some_endpoint"
	TestAPIVersion          = "2019-07-02"
	PrivateContainerAPIURL  = "private.test-iam-url"
)

var _ local.ContextCredentialsFactory = &auth.ContextCredentialsFactory{}

func GetTestLogger(t *testing.T) (logger *zap.Logger, teardown func()) {
	atom := zap.NewAtomicLevel()
	atom.SetLevel(zap.DebugLevel)

	encoderCfg := zap.NewProductionEncoderConfig()
	encoderCfg.TimeKey = "timestamp"
	encoderCfg.EncodeTime = zapcore.ISO8601TimeEncoder

	buf := &bytes.Buffer{}

	logger = zap.New(
		zapcore.NewCore(
			zapcore.NewJSONEncoder(encoderCfg),
			zapcore.AddSync(buf),
			atom,
		),
		zap.AddCaller(),
	)

	teardown = func() {
		err := logger.Sync()
		assert.Nil(t, err)

		if t.Failed() {
			t.Log(buf)
		}
	}

	return
}

func TestNewProvider(t *testing.T) {
	var err error
	conf := &vpcconfig.VPCFileConfig{
		ServerConfig: &config.ServerConfig{
			DebugTrace: true,
		},
		VPCConfig: &config.VPCProviderConfig{
			Enabled:                    false,
			EndpointURL:                TestEndpointURL,
			VPCTimeout:                 "30s",
			IamClientID:                IamClientID,
			IamClientSecret:            IamClientSecret,
			IKSTokenExchangePrivateURL: "token-exchange-private-URL",
		},
		IKSConfig: &config.IKSConfig{
			Enabled:             true,
			IKSFileProviderName: "vpc-file-share",
		},
	}

	logger, teardown := GetTestLogger(t)
	defer teardown()

	kc, _ := k8s_utils.FakeGetk8sClientSet()
	pwd, _ := os.Getwd()
	file := filepath.Join(pwd, "..", "..", "etc", "libconfig.toml")
	err = k8s_utils.FakeCreateSecret(kc, "DEFAULT", file)
	prov, err := NewProvider(conf, &kc, logger)
	assert.Nil(t, err)
	assert.NotNil(t, prov)
}

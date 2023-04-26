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

// Package provider ...
package provider

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/IBM/ibmcloud-volume-file-vpc/common/vpcclient/riaas/fakes"
	volumeServiceFakes "github.com/IBM/ibmcloud-volume-file-vpc/common/vpcclient/vpcfilevolume/fakes"
	vpcconfig "github.com/IBM/ibmcloud-volume-file-vpc/file/vpcconfig"
	"github.com/IBM/ibmcloud-volume-interface/config"
	"github.com/IBM/ibmcloud-volume-interface/lib/provider"
	util "github.com/IBM/ibmcloud-volume-interface/lib/utils"
	"github.com/IBM/ibmcloud-volume-interface/provider/auth"
	"github.com/IBM/ibmcloud-volume-interface/provider/local"
	"github.com/IBM/secret-utils-lib/pkg/k8s_utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
	PrivateRIaaSEndpoint    = "private.test-riaas-url"
	CsrfToken               = "csrf-token"
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
	logger, teardown := GetTestLogger(t)
	defer teardown()

	// gc public endpoint test
	conf := &vpcconfig.VPCFileConfig{

		VPCConfig: &config.VPCProviderConfig{
			Enabled:          true,
			EndpointURL:      TestEndpointURL,
			TokenExchangeURL: IamURL,
			APIKey:           IamClientSecret,
		},
	}

	kc, _ := k8s_utils.FakeGetk8sClientSet()
	pwd, _ := os.Getwd()
	file := filepath.Join(pwd, "..", "..", "etc", "libconfig.toml")
	err = k8s_utils.FakeCreateSecret(kc, "DEFAULT", file)
	prov, err := NewProvider(conf, &kc, logger)
	assert.Nil(t, prov)
	assert.NotNil(t, err)

	// GC private endpoint related test
	conf = &vpcconfig.VPCFileConfig{
		VPCConfig: &config.VPCProviderConfig{
			Enabled:                    true,
			PrivateEndpointURL:         PrivateRIaaSEndpoint,
			IKSTokenExchangePrivateURL: PrivateContainerAPIURL,
			APIKey:                     IamClientSecret,
			IamClientID:                IamClientID,
			IamClientSecret:            IamClientSecret,
		},
	}

	err = k8s_utils.FakeCreateSecret(kc, "DEFAULT", file)
	prov, err = NewProvider(conf, &kc, logger)
	assert.Nil(t, prov)
	assert.NotNil(t, err)

	// gc mix test
	conf = &vpcconfig.VPCFileConfig{
		VPCConfig: &config.VPCProviderConfig{
			Enabled:            true,
			PrivateEndpointURL: PrivateRIaaSEndpoint,
			APIKey:             IamClientSecret,
			G2TokenExchangeURL: IamURL,
			IamClientID:        IamClientID,
			IamClientSecret:    IamClientSecret,
		},
	}

	err = k8s_utils.FakeCreateSecret(kc, "DEFAULT", file)
	prov, err = NewProvider(conf, &kc, logger)
	assert.Nil(t, prov)
	assert.NotNil(t, err)

	// gen2 public endpoint related test
	conf = &vpcconfig.VPCFileConfig{
		VPCConfig: &config.VPCProviderConfig{
			Enabled:            true,
			G2EndpointURL:      TestEndpointURL,
			G2TokenExchangeURL: IamURL,
			G2APIKey:           IamClientSecret,
		},
	}

	err = k8s_utils.FakeCreateSecret(kc, "DEFAULT", file)
	prov, err = NewProvider(conf, &kc, logger)
	assert.Nil(t, prov)
	assert.NotNil(t, err)

	// gen2 private endpoint related test
	conf = &vpcconfig.VPCFileConfig{
		VPCConfig: &config.VPCProviderConfig{
			Enabled:                    true,
			G2EndpointPrivateURL:       PrivateRIaaSEndpoint,
			IKSTokenExchangePrivateURL: PrivateContainerAPIURL,
			G2APIKey:                   IamClientSecret,
			G2TokenExchangeURL:         IamURL,
			IamClientID:                IamClientID,
			IamClientSecret:            IamClientSecret,
		},
	}

	err = k8s_utils.FakeCreateSecret(kc, "DEFAULT", file)
	prov, err = NewProvider(conf, &kc, logger)
	assert.Nil(t, prov)
	assert.NotNil(t, err)

	// gen2 mix test
	conf = &vpcconfig.VPCFileConfig{
		VPCConfig: &config.VPCProviderConfig{
			Enabled:                    true,
			G2EndpointPrivateURL:       PrivateRIaaSEndpoint,
			IKSTokenExchangePrivateURL: PrivateContainerAPIURL,
			G2APIKey:                   IamClientSecret,
			G2TokenExchangeURL:         IamURL,
			IamClientID:                IamClientID,
			IamClientSecret:            IamClientSecret,
		},
	}

	err = k8s_utils.FakeCreateSecret(kc, "DEFAULT", file)
	prov, err = NewProvider(conf, &kc, logger)
	assert.Nil(t, prov)
	assert.NotNil(t, err)
	// TODO
	// zone := "Test Zone"
	// contextCF, _ := prov.ContextCredentialsFactory(&zone)
	// assert.NotNil(t, contextCF)
}

func GetTestProvider(t *testing.T, logger *zap.Logger) (*VPCFileProvider, error) {
	var cp *fakes.RegionalAPIClientProvider
	var uc, sc *fakes.RegionalAPI

	// SetRetryParameters sets the retry logic parameters
	SetRetryParameters(2, 5)

	logger.Info("Getting New test Provider")
	conf := &vpcconfig.VPCFileConfig{
		ServerConfig: &config.ServerConfig{
			DebugTrace: true,
		},
		VPCConfig: &config.VPCProviderConfig{
			Enabled:                    true,
			EndpointURL:                TestEndpointURL,
			VPCTimeout:                 "30s",
			MaxRetryAttempt:            5,
			MaxRetryGap:                10,
			APIVersion:                 TestAPIVersion,
			G2EndpointPrivateURL:       PrivateRIaaSEndpoint,
			IKSTokenExchangePrivateURL: PrivateContainerAPIURL,
			G2APIKey:                   IamClientSecret,
			G2TokenExchangeURL:         IamURL,
			IamClientID:                IamClientID,
			IamClientSecret:            IamClientSecret,
		},
	}

	// p, err := NewProvider(conf, logger)
	// assert.NotNil(t, p)
	// assert.Nil(t, err)

	timeout, _ := time.ParseDuration(conf.VPCConfig.VPCTimeout)

	// Inject a fake RIAAS API client
	cp = &fakes.RegionalAPIClientProvider{}
	uc = &fakes.RegionalAPI{}
	cp.NewReturnsOnCall(0, uc, nil)
	sc = &fakes.RegionalAPI{}
	cp.NewReturnsOnCall(1, sc, nil)

	volumeService := &volumeServiceFakes.FileShareService{}
	uc.FileShareServiceReturns(volumeService)

	httpClient, err := config.GeneralCAHttpClientWithTimeout(timeout)
	if err != nil {
		logger.Error("Failed to prepare HTTP client", util.ZapError(err))
		return nil, err
	}
	assert.NotNil(t, httpClient)

	provider := &VPCFileProvider{
		timeout:        timeout,
		Config:         conf,
		tokenGenerator: &tokenGenerator{config: conf.VPCConfig},
		httpClient:     httpClient,
	}
	assert.NotNil(t, provider)
	assert.Equal(t, provider.timeout, timeout)

	return provider, nil
}

func TestGetTestProvider(t *testing.T) {
	//var err error
	logger, teardown := GetTestLogger(t)
	defer teardown()

	prov, err := GetTestProvider(t, logger)
	assert.NotNil(t, prov)
	assert.Nil(t, err)

	zone := "Test Zone"
	contextCF, _ := prov.ContextCredentialsFactory(&zone)
	assert.Nil(t, contextCF)
	assert.NotNil(t, prov.httpClient)
}

func TestOpenSession(t *testing.T) {
	//var err error
	logger, teardown := GetTestLogger(t)
	defer teardown()

	vpcp, _ := GetTestProvider(t, logger)

	sessn, err := vpcp.OpenSession(context.Background(), provider.ContextCredentials{
		AuthType:     provider.IAMAccessToken,
		Credential:   TestProviderAccessToken,
		IAMAccountID: TestIKSAccountID,
	}, logger)

	require.NoError(t, err)
	assert.NotNil(t, sessn)

	sessn, err = vpcp.OpenSession(context.Background(), provider.ContextCredentials{
		AuthType:     provider.IAMAccessToken,
		IAMAccountID: TestIKSAccountID,
	}, logger)

	require.Error(t, err)
	assert.Nil(t, sessn)

	sessn, err = vpcp.OpenSession(context.Background(), provider.ContextCredentials{
		AuthType:     "WrongType",
		IAMAccountID: TestIKSAccountID,
	}, logger)

	require.Error(t, err)
	assert.Nil(t, sessn)
}

func GetTestOpenSession(t *testing.T, logger *zap.Logger) (sessn *VPCSession, uc, sc *fakes.RegionalAPI, err error) {
	vpcp, err := GetTestProvider(t, logger)

	m := http.NewServeMux()
	s := httptest.NewServer(m)
	assert.NotNil(t, s)

	vpcp.httpClient = http.DefaultClient

	// Inject a fake RIAAS API client
	cp := &fakes.RegionalAPIClientProvider{}
	uc = &fakes.RegionalAPI{}
	cp.NewReturnsOnCall(0, uc, nil)
	sc = &fakes.RegionalAPI{}
	cp.NewReturnsOnCall(1, sc, nil)
	vpcp.ClientProvider = cp

	sessn = &VPCSession{
		VPCAccountID: TestIKSAccountID,
		Config:       vpcp.Config,
		ContextCredentials: provider.ContextCredentials{
			AuthType:     provider.IAMAccessToken,
			Credential:   TestProviderAccessToken,
			IAMAccountID: TestIKSAccountID,
		},
		VolumeType: "vpc-share",
		Provider:   VPC,
		Apiclient:  uc,
		Logger:     logger,
		APIRetry:   NewFlexyRetryDefault(),
	}

	return
}

func TestGetTestOpenSession(t *testing.T) {
	//var err error
	logger, teardown := GetTestLogger(t)
	defer teardown()

	vpcs, uc, sc, err := GetTestOpenSession(t, logger)
	assert.NotNil(t, vpcs)
	assert.NotNil(t, uc)
	assert.NotNil(t, sc)
	assert.Nil(t, err)

	providerDisplayName := vpcs.GetProviderDisplayName()
	assert.Equal(t, providerDisplayName, provider.VolumeProvider("VPC-SHARE"))
	vpcs.Close()

	providerName := vpcs.ProviderName()
	assert.Equal(t, providerName, provider.VolumeProvider("VPC-SHARE"))

	volumeType := vpcs.Type()
	assert.Equal(t, volumeType, provider.VolumeType("vpc-share"))

	volume, _ := vpcs.GetVolume("test volume")
	assert.Nil(t, volume)
}

func TestGetPrivateEndpoint(t *testing.T) {
	logger, teardown := GetTestLogger(t)
	defer teardown()

	// passing public URL
	privateURL := getPrivateEndpoint(logger, "https://us-south.com")
	assert.Equal(t, privateURL, "https://private-us-south.com")

	// passing private URL
	privateURL = getPrivateEndpoint(logger, "https://private-us-south.com")
	assert.Equal(t, privateURL, "https://private-us-south.com")

	//Wrong URL
	privateURL = getPrivateEndpoint(logger, "https")
	assert.Equal(t, privateURL, "")
}

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

// Package iam ...
package iam

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/IBM/ibmcloud-volume-interface/config"
	util "github.com/IBM/ibmcloud-volume-interface/lib/utils"
	"github.com/IBM/ibmcloud-volume-interface/lib/utils/reasoncode"
	"github.com/IBM/ibmcloud-volume-interface/provider/iam"
	sp "github.com/IBM/secret-utils-lib/pkg/secret_provider"
)

var (
	mux              *http.ServeMux
	server           *httptest.Server
	logger           *zap.Logger
	lowPriority      zap.LevelEnablerFunc
	consoleDebugging zapcore.WriteSyncer
)

func TestMain(m *testing.M) {
	// Logging
	lowPriority = zap.LevelEnablerFunc(func(lvl zapcore.Level) bool {
		return lvl < zapcore.ErrorLevel
	})
	consoleDebugging = zapcore.Lock(os.Stdout)
	logger = zap.New(zapcore.NewCore(zapcore.NewJSONEncoder(zap.NewDevelopmentEncoderConfig()), consoleDebugging, lowPriority), zap.AddCaller())

	os.Exit(m.Run())
}

func Test_IKSExchangeRefreshTokenForAccessToken_Success(t *testing.T) {
	logger := zap.New(
		zapcore.NewCore(zapcore.NewJSONEncoder(zap.NewDevelopmentEncoderConfig()), consoleDebugging, lowPriority),
		zap.AddCaller(),
	)
	httpSetup()

	// IAM endpoint
	mux.HandleFunc("/v1/iam/apikey",
		func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(200)
			fmt.Fprint(w, `{"token": "at_success"}`)
		},
	)

	iksAuthConfig := &IksAuthConfiguration{
		PrivateAPIRoute: server.URL,
	}

	var err error
	tes := new(tokenExchangeIKSService)
	tes.httpClient, err = config.GeneralCAHttpClient()
	assert.Nil(t, err)
	tes.iksAuthConfig = iksAuthConfig
	tes.secretprovider = new(sp.FakeSecretProvider)

	r, err := tes.ExchangeRefreshTokenForAccessToken("testrefreshtoken", logger)
	assert.Nil(t, err)
	if assert.NotNil(t, r) {
		assert.Equal(t, (*r).Token, "at_success")
	}
}

func httpSetup() {
	// test server
	mux = http.NewServeMux()
	server = httptest.NewServer(mux)
}

func Test_IKSExchangeRefreshTokenForAccessToken_FailedDuringRequest(t *testing.T) {
	logger := zap.New(
		zapcore.NewCore(zapcore.NewJSONEncoder(zap.NewDevelopmentEncoderConfig()), consoleDebugging, lowPriority),
		zap.AddCaller(),
	)

	httpSetup()
	mux.HandleFunc("/v1/iam/apikey",
		func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
			fmt.Fprint(w, `{"description": "did not work",
				"code": "bad news",
				"type" : "more details",
				"incidentID" : "1000"
				}`)
		},
	)

	iksAuthConfig := &IksAuthConfiguration{
		PrivateAPIRoute: server.URL,
	}

	var err error
	tes := new(tokenExchangeIKSService)
	tes.httpClient, err = config.GeneralCAHttpClient()
	assert.Nil(t, err)
	tes.iksAuthConfig = iksAuthConfig
	tes.secretprovider = new(sp.FakeSecretProvider)

	r, err := tes.ExchangeRefreshTokenForAccessToken("badrefreshtoken", logger)
	assert.Nil(t, r)
	if assert.NotNil(t, err) {
		assert.Equal(t, "IAM token exchange request failed: did not work", err.Error())
		assert.Equal(t, reasoncode.ReasonCode("ErrorFailedTokenExchange"), util.ErrorReasonCode(err))
	}
}

func Test_IKSExchangeRefreshTokenForAccessToken_FailedDuringRequest_no_message(t *testing.T) {
	logger := zap.New(
		zapcore.NewCore(zapcore.NewJSONEncoder(zap.NewDevelopmentEncoderConfig()), consoleDebugging, lowPriority),
		zap.AddCaller(),
	)

	httpSetup()
	mux.HandleFunc("/v1/iam/apikey",
		func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
			fmt.Fprint(w, `{}`)
		},
	)

	iksAuthConfig := &IksAuthConfiguration{
		PrivateAPIRoute: server.URL,
	}

	var err error
	tes := new(tokenExchangeIKSService)
	tes.httpClient, err = config.GeneralCAHttpClient()
	assert.Nil(t, err)
	tes.iksAuthConfig = iksAuthConfig
	tes.secretprovider = new(sp.FakeSecretProvider)

	r, err := tes.ExchangeRefreshTokenForAccessToken("badrefreshtoken", logger)
	assert.Nil(t, r)
	if assert.NotNil(t, err) {
		assert.Equal(t, "Unexpected IAM token exchange response", err.Error())
		assert.Equal(t, reasoncode.ReasonCode("ErrorUnclassified"), util.ErrorReasonCode(err))
	}
}

func Test_IKSExchangeRefreshTokenForAccessToken_FailedWrongApiUrl(t *testing.T) {
	logger := zap.New(
		zapcore.NewCore(zapcore.NewJSONEncoder(zap.NewDevelopmentEncoderConfig()), consoleDebugging, lowPriority),
		zap.AddCaller(),
	)

	httpSetup()

	mux.HandleFunc("/v1/iam/apikey",
		func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
			fmt.Fprint(w, `{}`)
		},
	)

	iksAuthConfig := &IksAuthConfiguration{
		PrivateAPIRoute: "wrongProtocolURL",
	}

	var err error
	tes := new(tokenExchangeIKSService)
	tes.httpClient, err = config.GeneralCAHttpClient()
	assert.Nil(t, err)
	tes.iksAuthConfig = iksAuthConfig
	tes.secretprovider = new(sp.FakeSecretProvider)

	r, err := tes.ExchangeRefreshTokenForAccessToken("testrefreshtoken", logger)
	assert.Nil(t, r)

	if assert.NotNil(t, err) {
		assert.Equal(t, "IAM token exchange request failed", err.Error())
		assert.Equal(t, reasoncode.ReasonCode("ErrorUnclassified"), util.ErrorReasonCode(err))
		assert.Equal(t, []string{"Post \"wrongProtocolURL/v1/iam/apikey\": unsupported protocol scheme \"\""},
			util.ErrorDeepUnwrapString(err))
	}
}

/*
func Test_IKSExchangeRefreshTokenForAccessToken_FailedRequesting_unclassified_error(t *testing.T) {
	logger := zap.New(
		zapcore.NewCore(zapcore.NewJSONEncoder(zap.NewDevelopmentEncoderConfig()), consoleDebugging, lowPriority),
		zap.AddCaller(),
	)

	httpSetup()

	mux.HandleFunc("/v1/iam/apikey",
		func(w http.ResponseWriter, r *http.Request) {
			// Leave response empty
		},
	)

	iksAuthConfig := &iam.AuthConfiguration{
		//PrivateAPIRoute: server.URL,
	}

	tes, err := iam.NewTokenExchangeService(iksAuthConfig)
	assert.NoError(t, err)

	r, err := tes.ExchangeRefreshTokenForAccessToken("badrefreshtoken", logger)
	assert.Nil(t, r)

	if assert.NotNil(t, err) {
		assert.Equal(t, "IAM token exchange request failed", err.Error())
		assert.Equal(t, reasoncode.ReasonCode("ErrorUnclassified"), util.ErrorReasonCode(err))
	}
}
*/

func Test_IKSExchangeIAMAPIKeyForAccessToken(t *testing.T) {
	var testCases = []struct {
		name          string
		expectedError error
	}{
		{
			name:          "Unable to fetch token",
			expectedError: errors.New("not nil"),
		},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			logger := zap.New(
				zapcore.NewCore(zapcore.NewJSONEncoder(zap.NewDevelopmentEncoderConfig()), consoleDebugging, lowPriority),
				zap.AddCaller(),
			)
			httpSetup()

			iksAuthConfig := &IksAuthConfiguration{
				PrivateAPIRoute: server.URL,
			}

			var err error
			tes := new(tokenExchangeIKSService)
			tes.httpClient, err = config.GeneralCAHttpClient()
			assert.Nil(t, err)
			tes.iksAuthConfig = iksAuthConfig
			tes.secretprovider = new(sp.FakeSecretProvider)

			_, actualError := tes.ExchangeIAMAPIKeyForAccessToken("apikey1", logger)
			if testCase.expectedError == nil {
				assert.Nil(t, actualError)
			} else {
				assert.NotNil(t, actualError)
			}
		})
	}
}

func TestNewTokenExchangeIKSService(t *testing.T) {
	iksAuthConfig := &IksAuthConfiguration{
		PrivateAPIRoute: server.URL,
	}

	_, err := NewTokenExchangeIKSService(iksAuthConfig)
	assert.NotNil(t, err)
}

func TestExchangeAccessTokenForIMSToken(t *testing.T) {
	tes := new(tokenExchangeIKSService)
	logger = zap.New(zapcore.NewCore(zapcore.NewJSONEncoder(zap.NewDevelopmentEncoderConfig()), consoleDebugging, lowPriority), zap.AddCaller())
	_, err := tes.ExchangeAccessTokenForIMSToken(iam.AccessToken{}, logger)
	assert.Nil(t, err)
}

func TestExchangeIAMAPIKeyForIMSToken(t *testing.T) {
	tes := new(tokenExchangeIKSService)
	logger = zap.New(zapcore.NewCore(zapcore.NewJSONEncoder(zap.NewDevelopmentEncoderConfig()), consoleDebugging, lowPriority), zap.AddCaller())
	_, err := tes.ExchangeIAMAPIKeyForIMSToken("", logger)
	assert.Nil(t, err)
}

func TestGetIAMAccountIDFromAccessToken(t *testing.T) {
	tes := new(tokenExchangeIKSService)
	logger = zap.New(zapcore.NewCore(zapcore.NewJSONEncoder(zap.NewDevelopmentEncoderConfig()), consoleDebugging, lowPriority), zap.AddCaller())
	_, err := tes.GetIAMAccountIDFromAccessToken(iam.AccessToken{}, logger)
	assert.Nil(t, err)
}

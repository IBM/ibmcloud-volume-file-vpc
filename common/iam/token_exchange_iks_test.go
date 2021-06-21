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
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	util "github.com/IBM/ibmcloud-volume-interface/lib/utils"
	"github.com/IBM/ibmcloud-volume-interface/lib/utils/reasoncode"
	"github.com/IBM/ibmcloud-volume-interface/provider/iam"
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

	tes, err := NewTokenExchangeIKSService(iksAuthConfig)
	assert.NoError(t, err)

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

	tes, err := NewTokenExchangeIKSService(iksAuthConfig)
	assert.NoError(t, err)

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

	tes, err := NewTokenExchangeIKSService(iksAuthConfig)
	assert.NoError(t, err)

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

	tes, err := NewTokenExchangeIKSService(iksAuthConfig)
	assert.NoError(t, err)

	r, err := tes.ExchangeRefreshTokenForAccessToken("testrefreshtoken", logger)
	assert.Nil(t, r)

	if assert.NotNil(t, err) {
		assert.Equal(t, "IAM token exchange request failed", err.Error())
		assert.Equal(t, reasoncode.ReasonCode("ErrorUnclassified"), util.ErrorReasonCode(err))
		assert.Equal(t, []string{"Post \"wrongProtocolURL/v1/iam/apikey\": unsupported protocol scheme \"\""},
			util.ErrorDeepUnwrapString(err))
	}
}

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

func TestExchangeAccessTokenForIMSToken(t *testing.T) {
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

	tes, err := NewTokenExchangeIKSService(iksAuthConfig)
	assert.NoError(t, err)

	r, err := tes.ExchangeAccessTokenForIMSToken(iam.AccessToken{}, logger)
	assert.Nil(t, r)
}

func TestExchangeIAMAPIKeyForIMSToken(t *testing.T) {
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

	tes, err := NewTokenExchangeIKSService(iksAuthConfig)
	assert.NoError(t, err)

	r, err := tes.ExchangeIAMAPIKeyForIMSToken("apikey", logger)
	assert.Nil(t, r)
}

func TestGetIAMAccountIDFromAccessToken(t *testing.T) {
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

	tes, err := NewTokenExchangeIKSService(iksAuthConfig)
	assert.NoError(t, err)

	r, err := tes.GetIAMAccountIDFromAccessToken(iam.AccessToken{}, logger)
	assert.Equal(t, "Not required to implement", r)
}
func Test_IKSExchangeIAMAPIKeyForAccessToken(t *testing.T) {
	var testCases = []struct {
		name               string
		apiHandler         func(w http.ResponseWriter, r *http.Request)
		expectedToken      string
		expectedError      *string
		expectedReasonCode string
	}{
		{
			name: "client error",
			apiHandler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(400)
			},
			expectedError:      iam.String("IAM token exchange request failed"),
			expectedReasonCode: "ErrorUnclassified",
		},
		{
			name: "success 200",
			apiHandler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(200)
				fmt.Fprint(w, `{ "token": "access_token_123" }`)
			},
			expectedToken: "access_token_123",
			expectedError: nil,
		},
		{
			name: "unauthorised",
			apiHandler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(401)
				fmt.Fprint(w, `{"description": "not authorised",
					"code": "authorisation",
					"type" : "more details",
					"incidentID" : "1000"
					}`)
			},
			expectedError:      iam.String("IAM token exchange request failed: not authorised"),
			expectedReasonCode: "ErrorFailedTokenExchange",
		},
		{
			name: "no error message",
			apiHandler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(400)
				fmt.Fprint(w, `{"code" : "ErrorUnclassified",
					"incidentID" : "10000"
					}`)
			},
			expectedError:      iam.String("Unexpected IAM token exchange response"),
			expectedReasonCode: "ErrorUnclassified",
		},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			logger := zap.New(
				zapcore.NewCore(zapcore.NewJSONEncoder(zap.NewDevelopmentEncoderConfig()), consoleDebugging, lowPriority),
				zap.AddCaller(),
			)
			httpSetup()

			// ResourceController endpoint
			mux.HandleFunc("/v1/iam/apikey", testCase.apiHandler)

			iksAuthConfig := &IksAuthConfiguration{
				PrivateAPIRoute: server.URL,
			}

			tes, err := NewTokenExchangeIKSService(iksAuthConfig)
			assert.NoError(t, err)

			r, actualError := tes.ExchangeIAMAPIKeyForAccessToken("apikey1", logger)
			if testCase.expectedError == nil {
				assert.NoError(t, actualError)
				if assert.NotNil(t, r) {
					assert.Equal(t, testCase.expectedToken, r.Token)
				}
			} else {
				if assert.Error(t, actualError) {
					assert.Equal(t, *testCase.expectedError, actualError.Error())
					assert.Equal(t, reasoncode.ReasonCode(testCase.expectedReasonCode), util.ErrorReasonCode(actualError))
				}
				assert.Nil(t, r)
			}
		})
	}
}

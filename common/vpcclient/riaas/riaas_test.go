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

// Package riaas ...
package riaas

import (
	"bytes"
	"io"
	"net/http"
	"testing"

	"github.com/IBM/ibmcloud-volume-file-vpc/common/vpcclient/client/fakes"
	"github.com/stretchr/testify/assert"
)

func TestLogin(t *testing.T) {
	client := &fakes.SessionClient{}

	riaas := Session{
		client: client,
	}

	err := riaas.Login("token")

	if assert.Equal(t, 1, client.WithAuthTokenCallCount()) {
		assert.Equal(t, "token", client.WithAuthTokenArgsForCall(0))
	}

	assert.NoError(t, err)
}

func TestNewSession(t *testing.T) {
	var b bytes.Buffer
	cfg := Config{
		BaseURL:       "http://gc",
		AccountID:     "test account ID",
		Username:      "tester",
		APIKey:        "tester",
		ResourceGroup: "test resource group",
		Password:      "tester",
		ContextID:     "tester",
		APIVersion:    "2019-06-05",
		APIGeneration: 2,
		HTTPClient:    &http.Client{},
		DebugWriter:   io.Writer(&b),
	}

	session, err := New(cfg)
	assert.Nil(t, err)
	assert.NotNil(t, session)

	d := DefaultRegionalAPIClientProvider{}
	regionalAPI, err := d.New(cfg)
	assert.Nil(t, err)
	assert.NotNil(t, regionalAPI)

	noAPIVerAndGen := Config{
		BaseURL:       "http://gc",
		AccountID:     "test account ID",
		Username:      "tester",
		APIKey:        "tester",
		ResourceGroup: "test resource group",
		Password:      "tester",
		ContextID:     "tester",
		HTTPClient:    &http.Client{},
		DebugWriter:   io.Writer(&b),
	}
	sessionAPI, err := New(noAPIVerAndGen)
	assert.Nil(t, err)
	assert.NotNil(t, sessionAPI)
}

func TestVolumeFileService(t *testing.T) {
	volumeManager := (&Session{}).FileShareService()
	assert.NotNil(t, volumeManager)
}

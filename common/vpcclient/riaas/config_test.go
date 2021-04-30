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
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestConfig(t *testing.T) {
	cfg := &Config{
		BaseURL:       "http://gc",
		AccountID:     "test account ID",
		Username:      "tester",
		APIKey:        "tester",
		ResourceGroup: "test resource group",
		Password:      "tester",
		ContextID:     "tester",
		APIVersion:    "01-01-2019",
		HTTPClient:    nil,
	}
	assert.NotNil(t, cfg.httpClient())
	cfg.HTTPClient = &http.Client{}
	assert.NotNil(t, cfg.httpClient())
	assert.Equal(t, "http://gc", cfg.baseURL())
}

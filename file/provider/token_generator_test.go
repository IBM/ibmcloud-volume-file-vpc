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
	"github.com/stretchr/testify/assert"

	"testing"

	"github.com/IBM/ibmcloud-volume-interface/lib/provider"
)

func TestTokenGenerator(t *testing.T) {
	logger, teardown := GetTestLogger(t)
	defer teardown()

	tg := tokenGenerator{}
	assert.NotNil(t, tg)

	cf := provider.ContextCredentials{
		AuthType:     provider.IAMAccessToken,
		Credential:   TestProviderAccessToken,
		IAMAccountID: TestIKSAccountID,
	}
	signedToken, err := tg.getServiceToken(cf, *logger)
	assert.Nil(t, signedToken)
	assert.NotNil(t, err)

	//TODO do we need to write test cases for valid/invalid keys ?
}

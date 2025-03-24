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

package provider

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIksVpcSession_Close(t *testing.T) {
	vpcIks := &IksVpcSession{}
	vpcIks.Close()
}

func TestIksVpcSession_GetProviderDisplayName(t *testing.T) {
	vpcIks := &IksVpcSession{}
	assert.Equal(t, Provider, vpcIks.GetProviderDisplayName())
}

func TestIksVpcSession_ProviderName(t *testing.T) {
	vpcIks := &IksVpcSession{}
	assert.Equal(t, Provider, vpcIks.ProviderName())
}

func TestIksVpcSession_Type(t *testing.T) {
	vpcIks := &IksVpcSession{}
	assert.Equal(t, VolumeType, vpcIks.Type())
}

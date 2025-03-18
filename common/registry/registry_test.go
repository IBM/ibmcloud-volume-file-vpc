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

package registry

import (
	"testing"

	vpc_provider "github.com/IBM/ibmcloud-volume-file-vpc/file/provider"
	"github.com/IBM/ibmcloud-volume-interface/provider/local"
	"github.com/stretchr/testify/assert"
)

// TestProviderRegistry_Get tests the Get method of ProviderRegistry
func TestProviderRegistryGet(t *testing.T) {
	pr := &ProviderRegistry{
		providers: map[string]local.Provider{
			"test-provider": &vpc_provider.VPCFileProvider{},
		},
	}

	prov, err := pr.Get("test-provider")
	assert.NoError(t, err)
	assert.NotNil(t, prov)

	_, err = pr.Get("unknown-provider")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Provider unknown: unknown-provider")
}

// TestProviderRegistry_Register tests the Register method of ProviderRegistry
func TestProviderRegistryRegister(t *testing.T) {
	pr := &ProviderRegistry{}

	prov := &vpc_provider.VPCFileProvider{}
	pr.Register("test-provider", prov)

	retrievedProv, err := pr.Get("test-provider")
	assert.NoError(t, err)
	assert.Equal(t, prov, retrievedProv)
}

// TestProviderRegistry_Register_WithNilMap tests the Register method of ProviderRegistry with a nil map
func TestProviderRegistryRegisterWithNilMap(t *testing.T) {
	pr := &ProviderRegistry{
		providers: nil,
	}

	prov := &vpc_provider.VPCFileProvider{}
	pr.Register("test-provider", prov)

	retrievedProv, err := pr.Get("test-provider")
	assert.NoError(t, err)
	assert.Equal(t, prov, retrievedProv)
}

// TestProviderRegistry_Register_WithExistingProvider tests the Register method of ProviderRegistry with an existing provider
func TestProviderRegistryRegisterWithExistingProvider(t *testing.T) {
	pr := &ProviderRegistry{
		providers: map[string]local.Provider{
			"test-provider": &vpc_provider.VPCFileProvider{},
		},
	}

	prov := &vpc_provider.VPCFileProvider{}
	pr.Register("test-provider", prov)

	retrievedProv, err := pr.Get("test-provider")
	assert.NoError(t, err)
	assert.Equal(t, prov, retrievedProv)
}

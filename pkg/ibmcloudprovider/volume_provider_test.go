/*
Copyright 2022 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at
    http://www.apache.org/licenses/LICENSE-2.0
Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Package ibmcloudprovider ...
package ibmcloudprovider

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/IBM/secret-utils-lib/pkg/k8s_utils"
	"github.com/stretchr/testify/assert"
)

func TestNewIBMCloudStorageProvider(t *testing.T) {
	// Creating test logger
	logger, teardown := GetTestLogger(t)
	defer teardown()

	pwd, err := os.Getwd()
	if err != nil {
		t.Errorf("Failed to get current working directory, some unit tests will fail")
	}

	//Valid Use case
	// As its required by NewIBMCloudStorageProvider
	secretConfigPath := filepath.Join(pwd, "..", "..", "test-fixtures", "valid")
	err = os.Setenv("SECRET_CONFIG_PATH", secretConfigPath)
	defer os.Unsetenv("SECRET_CONFIG_PATH")
	if err != nil {
		t.Errorf("This test will fail because of %v", err)
	}

	kc, _ := k8s_utils.FakeGetk8sClientSet()
	pwd, _ = os.Getwd()
	file := filepath.Join(pwd, "..", "..", "etc", "libconfig.toml")
	err = k8s_utils.FakeCreateSecret(kc, "DEFAULT", file)
	configPath := filepath.Join(pwd, "..", "..", "test-fixtures", "slconfig.toml")
	_ = k8s_utils.FakeCreateCM(kc, configPath)
	ibmCloudProvider, err := NewIBMCloudStorageProvider("test", &kc, logger)
	assert.NotNil(t, ibmCloudProvider) //TODO
	assert.Nil(t, err)                 //TODO

	//Invalid clusterinfo case
	// As its required by NewFakeIBMCloudStorageProvider
	// secretConfigPath = filepath.Join(pwd, "..", "..", "test-fixtures", "invalid")
	// err = os.Setenv("SECRET_CONFIG_PATH", secretConfigPath)
	// defer os.Unsetenv("SECRET_CONFIG_PATH")
	// if err != nil {
	// 	t.Errorf("This test will fail because of %v", err)
	// }

	// configPath = filepath.Join(pwd, "..", "..", "test-fixtures", "slconfig-invalid.toml")
	// ibmCloudProvider, err = NewIBMCloudStorageProvider(configPath, logger)
	// assert.NotNil(t, err)
	// assert.Nil(t, ibmCloudProvider)

	// //Invalid slconfig.toml case
	// secretConfigPath = filepath.Join(pwd, "..", "..", "test-fixtures", "valid")
	// err = os.Setenv("SECRET_CONFIG_PATH", secretConfigPath)
	// defer os.Unsetenv("SECRET_CONFIG_PATH")
	// if err != nil {
	// 	t.Errorf("This test will fail because of %v", err)
	// }
	// configPath = filepath.Join(pwd, "..", "..", "test-fixtures", "slconfig-invalid-format.toml")
	// ibmCloudProvider, err = NewIBMCloudStorageProvider(configPath, logger)
	// assert.NotNil(t, err)
	// assert.Nil(t, ibmCloudProvider)
}

func TestNewFakeIBMCloudStorageProvider(t *testing.T) {
	// Creating test logger
	logger, teardown := GetTestLogger(t)
	defer teardown()

	pwd, err := os.Getwd()
	if err != nil {
		t.Errorf("Failed to get current working directory, some unit tests will fail")
	}

	// As its required by NewFakeIBMCloudStorageProvider
	secretConfigPath := filepath.Join(pwd, "..", "..", "test-fixtures", "valid")
	err = os.Setenv("SECRET_CONFIG_PATH", secretConfigPath)
	defer os.Unsetenv("SECRET_CONFIG_PATH")
	if err != nil {
		t.Errorf("This test will fail because of %v", err)
	}

	configPath := filepath.Join(pwd, "..", "..", "test-fixtures", "slconfig.toml")
	ibmFakeCloudProvider, err := NewFakeIBMCloudStorageProvider(configPath, logger)
	assert.Nil(t, err)
	assert.NotNil(t, ibmFakeCloudProvider)

	fakeSession, err := ibmFakeCloudProvider.GetProviderSession(context.TODO(), logger)
	assert.Nil(t, err)
	assert.NotNil(t, fakeSession)

	cloudProviderConfig := ibmFakeCloudProvider.GetConfig()
	assert.NotNil(t, cloudProviderConfig)

	clusterID := ibmFakeCloudProvider.GetClusterID()
	assert.NotNil(t, clusterID)
}

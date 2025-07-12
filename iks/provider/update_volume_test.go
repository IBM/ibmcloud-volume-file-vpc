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

	userError "github.com/IBM/ibmcloud-volume-file-vpc/common/messages"
	provider "github.com/IBM/ibmcloud-volume-interface/lib/provider"
	"github.com/stretchr/testify/assert"
)

func TestValidateVolumeRequest(t *testing.T) {
	userError.MessagesEn = userError.InitMessages()
	// Define test cases
	testCases := []struct {
		name             string
		expectedErrMsg   string
		expectedErroCode string
		volumeRequest    provider.Volume
	}{
		{
			name:           "No error",
			expectedErrMsg: "",
			// Setup test environment
			volumeRequest: provider.Volume{
				VolumeID:   "test-volume-id",
				Provider:   "test-provider",
				VolumeType: "test-volume-type",
			},
		},
		{
			name: "Error with empty VolumeID ",
			// Setup test environment
			volumeRequest: provider.Volume{
				VolumeID:   "",
				Provider:   "test-provider",
				VolumeType: "test-volume-type",
			},
			expectedErroCode: "ErrorRequiredFieldMissing",
			expectedErrMsg:   "{Code:ErrorRequiredFieldMissing, Description:[VolumeID] is required to complete the operation, RC:400}",
		},
		{
			name: "Error with empty VolumeType ",
			// Setup test environment
			volumeRequest: provider.Volume{
				VolumeID:   "test-volume-id",
				Provider:   "test-provider",
				VolumeType: "",
			},
			expectedErroCode: "ErrorRequiredFieldMissing",
			expectedErrMsg:   "{Code:ErrorRequiredFieldMissing, Description:[VolumeType] is required to complete the operation, RC:400}",
		},
		{
			name: "Error with empty provider ",
			// Setup test environment
			volumeRequest: provider.Volume{
				VolumeID:   "test-volume-id",
				Provider:   "",
				VolumeType: "test-volume-type",
			},
			expectedErroCode: "ErrorRequiredFieldMissing",
			expectedErrMsg:   "{Code:ErrorRequiredFieldMissing, Description:[Provider] is required to complete the operation, RC:400}",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateVolumeRequest(tc.volumeRequest)

			if tc.expectedErrMsg != "" {
				assert.ErrorContains(t, err, tc.expectedErroCode)
			} else {
				assert.Nil(t, err)
			}

		})
	}
}

// Int64 returns a pointer to the int64 value provided
func Int64(v int64) *int64 {
	return &v
}

func TestNewUpdatePVC(t *testing.T) {

	// Define test cases
	testCases := []struct {
		name          string
		expectedIops  int64
		volumeRequest provider.Volume
	}{
		{
			name:         "Success",
			expectedIops: 1000,
			// Setup test environment
			// Create a volumeRequest
			volumeRequest: provider.Volume{
				VolumeID: "test-volume-id",
				VPCVolume: provider.VPCVolume{
					CRN:  "test-crn",
					Tags: []string{"test-tag:test-value"},
				},
				Provider:   "test-provider",
				VolumeType: "test-volume-type",
				Name:       String("test-volume-name"),
				Capacity:   Int(10),
				Iops:       String("1000"),
				Attributes: map[string]string{
					ClusterIDTagName: "test-cluster-id",
					VolumeStatus:     "available",
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			pvc := NewUpdatePVC(tc.volumeRequest)
			assert.NotNil(t, pvc)
			assert.Equal(t, pvc.Iops, tc.expectedIops)
		})
	}
}

// String returns a pointer to the string value provided
func String(v string) *string {
	return &v
}

// Int returns a pointer to the int value provided
func Int(v int) *int {
	return &v
}

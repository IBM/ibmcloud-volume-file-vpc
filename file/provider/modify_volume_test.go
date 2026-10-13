/**
 * Copyright 2026 IBM Corp.
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
	"errors"
	"fmt"
	"testing"

	userError "github.com/IBM/ibmcloud-volume-file-vpc/common/messages"
	"github.com/IBM/ibmcloud-volume-file-vpc/common/vpcclient/models"
	volumeServiceFakes "github.com/IBM/ibmcloud-volume-file-vpc/common/vpcclient/vpcfilevolume/fakes"
	"github.com/IBM/ibmcloud-volume-interface/lib/provider"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestModifyVolume(t *testing.T) {
	userError.MessagesEn = userError.InitMessages()

	logger, teardown := GetTestLogger(t)
	defer teardown()

	var fileShareService *volumeServiceFakes.FileShareService

	testCases := []struct {
		testCaseName string
		request      provider.ModifyVolumeRequest
		// fake share returned by ModifyShare (client layer)
		modifyShareResp *models.Share
		modifyShareErr  error
		// fake share returned by GetFileShare (for WaitForValidVolumeState)
		getShareResp *models.Share
		getShareErr  error
		// expected
		expectErr          bool
		expectedReasonCode string
	}{
		{
			testCaseName: "missing VolumeID returns error",
			request:      provider.ModifyVolumeRequest{},
			expectErr:    true,
		},
		{
			testCaseName: "no iops or bandwidth returns empty response",
			request:      provider.ModifyVolumeRequest{VolumeID: "share-id"},
			expectErr:    false,
		},
		{
			testCaseName: "successful IOPS modify (dp2)",
			request:      provider.ModifyVolumeRequest{VolumeID: "share-id", Iops: 5000},
			modifyShareResp: &models.Share{
				ID:     "share-id",
				Status: models.StatusType("stable"),
				Iops:   5000,
			},
			getShareResp: &models.Share{
				ID:     "share-id",
				Status: models.StatusType("stable"),
				Iops:   5000,
			},
			expectErr: false,
		},
		{
			testCaseName: "successful bandwidth modify (rfs)",
			request:      provider.ModifyVolumeRequest{VolumeID: "share-id", Bandwidth: 5000},
			modifyShareResp: &models.Share{
				ID:        "share-id",
				Status:    models.StatusType("stable"),
				Bandwidth: 5000,
			},
			getShareResp: &models.Share{
				ID:        "share-id",
				Status:    models.StatusType("stable"),
				Bandwidth: 5000,
			},
			expectErr: false,
		},
		{
			testCaseName:       "ModifyShare client error returns FailedToModifyVolume",
			request:            provider.ModifyVolumeRequest{VolumeID: "share-id", Iops: 5000},
			modifyShareErr:     errors.New("backend failure"),
			expectErr:          true,
			expectedReasonCode: "FailedToModifyVolume",
		},
		{
			testCaseName:    "ModifyShare returns empty share ID returns FailedToModifyVolume",
			request:         provider.ModifyVolumeRequest{VolumeID: "share-id", Iops: 5000},
			modifyShareResp: &models.Share{
				// empty ID simulates empty share ID in response
			},
			expectErr:          true,
			expectedReasonCode: "FailedToModifyVolume",
		},
		{
			testCaseName: "volume stuck in non-stable state returns VolumeNotInValidState",
			request:      provider.ModifyVolumeRequest{VolumeID: "share-id", Iops: 5000},
			modifyShareResp: &models.Share{
				ID:     "share-id",
				Status: models.StatusType("pending"),
				Iops:   5000,
			},
			getShareResp: &models.Share{
				ID:     "share-id",
				Status: models.StatusType("pending"),
			},
			expectErr:          true,
			expectedReasonCode: "VolumeNotInValidState",
		},
	}

	for _, testcase := range testCases {
		t.Run(testcase.testCaseName, func(t *testing.T) {
			logger.Info("Started", zap.String("testCase", testcase.testCaseName))

			vpcs, uc, sc, err := GetTestOpenSession(t, logger)
			// Limit retries to 1 so error-path cases don't sleep between attempts
			origMaxRetryAttempt := maxRetryAttempt
			maxRetryAttempt = 1
			t.Cleanup(func() { maxRetryAttempt = origMaxRetryAttempt })
			assert.NotNil(t, vpcs)
			assert.NotNil(t, uc)
			assert.NotNil(t, sc)
			assert.Nil(t, err)

			fileShareService = &volumeServiceFakes.FileShareService{}
			fmt.Println("Success volumeshareservice")
			assert.NotNil(t, fileShareService)
			uc.FileShareServiceReturns(fileShareService)

			fileShareService.ModifyShareReturns(testcase.modifyShareResp, testcase.modifyShareErr)
			fileShareService.GetFileShareReturns(testcase.getShareResp, testcase.getShareErr)

			resp, err := vpcs.ModifyVolume(testcase.request)

			if testcase.expectErr {
				assert.NotNil(t, err)
				logger.Info("Error details", zap.Reflect("Error", err))
				if testcase.expectedReasonCode != "" {
					assert.Contains(t, err.Error(), testcase.expectedReasonCode)
				}
			} else {
				assert.Nil(t, err)
				if testcase.request.Iops > 0 || testcase.request.Bandwidth > 0 {
					assert.NotNil(t, resp)
				}
			}
		})
	}
}

// TestValidateVolumeRequest_VACBandwidth exercises the bandwidth (throughput) path
// inside validateVolumeRequest, which is the entry-point for VAC bandwidth on
// CreateVolume.
func TestValidateVolumeRequest_VACBandwidth(t *testing.T) {
	userError.MessagesEn = userError.InitMessages()

	volName := "test-vol"
	capacity := 10
	rg := &provider.ResourceGroup{ID: "rg-id"}
	profile := func(name string) *provider.Profile { return &provider.Profile{Name: name} }

	testCases := []struct {
		name          string
		volume        provider.Volume
		wantBandwidth int32
		wantIops      int64
		wantErr       bool
		wantErrCode   string
	}{
		{
			name: "rfs: bandwidth from VAC is passed through",
			volume: provider.Volume{
				Name:     &volName,
				Capacity: &capacity,
				VPCVolume: provider.VPCVolume{
					Profile:       profile("rfs"),
					ResourceGroup: rg,
					Bandwidth:     5000,
				},
			},
			wantBandwidth: 5000,
		},
		{
			name: "rfs: zero bandwidth is valid (not set by VAC)",
			volume: provider.Volume{
				Name:     &volName,
				Capacity: &capacity,
				VPCVolume: provider.VPCVolume{
					Profile:       profile("rfs"),
					ResourceGroup: rg,
					Bandwidth:     0,
				},
			},
			wantBandwidth: 0,
		},
		{
			name: "dp2: bandwidth zero is valid",
			volume: provider.Volume{
				Name:     &volName,
				Capacity: &capacity,
				VPCVolume: provider.VPCVolume{
					Profile:       profile("dp2"),
					ResourceGroup: rg,
					Bandwidth:     0,
				},
			},
			wantBandwidth: 0,
		},
		{
			name: "iops set for dp2 is forwarded",
			volume: provider.Volume{
				Name:     &volName,
				Capacity: &capacity,
				Iops:     func() *string { s := "3000"; return &s }(),
				VPCVolume: provider.VPCVolume{
					Profile:       profile("dp2"),
					ResourceGroup: rg,
				},
			},
			wantIops: 3000,
		},
		{
			name: "missing volume name returns error",
			volume: provider.Volume{
				Capacity: &capacity,
				VPCVolume: provider.VPCVolume{
					Profile:       profile("dp2"),
					ResourceGroup: rg,
				},
			},
			wantErr:     true,
			wantErrCode: "InvalidVolumeName",
		},
		{
			name: "nil profile returns error",
			volume: provider.Volume{
				Name:     &volName,
				Capacity: &capacity,
				VPCVolume: provider.VPCVolume{
					ResourceGroup: rg,
					// Profile intentionally nil
				},
			},
			wantErr: true,
			// VolumeProfileEmpty has no entry in messages_en.go — just assert error returned
		},
		{
			name: "nil resource group returns error",
			volume: provider.Volume{
				Name:     &volName,
				Capacity: &capacity,
				VPCVolume: provider.VPCVolume{
					Profile: profile("dp2"),
				},
			},
			wantErr:     true,
			wantErrCode: "EmptyResourceGroup",
		},
		{
			name: "nil capacity returns error",
			volume: provider.Volume{
				Name: &volName,
				VPCVolume: provider.VPCVolume{
					Profile:       profile("dp2"),
					ResourceGroup: rg,
				},
			},
			wantErr:     true,
			wantErrCode: "VolumeCapacityInvalid",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, iops, bw, err := validateVolumeRequest(tc.volume)
			if tc.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.wantErrCode)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tc.wantBandwidth, bw, "bandwidth mismatch")
				assert.Equal(t, tc.wantIops, iops, "iops mismatch")
			}
		})
	}
}

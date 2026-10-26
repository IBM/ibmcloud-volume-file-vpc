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

		// expected verification
		verify func(t *testing.T, resp *provider.ModifyVolumeResponse, err error)
	}{
		{
			testCaseName: "VolumeID is empty",
			request:      provider.ModifyVolumeRequest{},
			verify: func(t *testing.T, resp *provider.ModifyVolumeResponse, err error) {
				assert.Nil(t, resp)
				assert.NotNil(t, err)
				assert.Contains(t, err.Error(), "ErrorRequiredFieldMissing")
			},
		},
		{
			testCaseName: "No iops or bandwidth requested",
			request:      provider.ModifyVolumeRequest{VolumeID: "share-id"},
			verify: func(t *testing.T, resp *provider.ModifyVolumeResponse, err error) {
				assert.NotNil(t, resp)
				assert.Nil(t, err)
			},
		},
		{
			testCaseName: "Modify iops for DP2 profile",
			request:      provider.ModifyVolumeRequest{VolumeID: "share-id", VolumePerformance: provider.VolumePerformance{Iops: 5000}},
			modifyShareResp: &models.Share{
				ID:     "share-id",
				Href:   "https://example/shares/share-id",
				CRN:    "crn:v1:share-id",
				Status: models.StatusType("stable"),
				Iops:   500,
			},
			getShareResp: &models.Share{
				ID:     "share-id",
				Status: models.StatusType("stable"),
				Iops:   5000,
			},
			verify: func(t *testing.T, resp *provider.ModifyVolumeResponse, err error) {
				assert.NotNil(t, resp)
				assert.Nil(t, err)
				assert.Equal(t, "share-id", resp.VolumeID)
				assert.Equal(t, int64(500), resp.VolumePerformance.Iops)
			},
		},
		{
			testCaseName: "Modify bandwidth for RFS profile",
			request:      provider.ModifyVolumeRequest{VolumeID: "share-id", VolumePerformance: provider.VolumePerformance{Bandwidth: 5000}},
			modifyShareResp: &models.Share{
				ID:        "share-id",
				Status:    models.StatusType("stable"),
				Bandwidth: 500,
			},
			getShareResp: &models.Share{
				ID:        "share-id",
				Status:    models.StatusType("stable"),
				Bandwidth: 5000,
			},
			verify: func(t *testing.T, resp *provider.ModifyVolumeResponse, err error) {
				assert.NotNil(t, resp)
				assert.Nil(t, err)
				assert.Equal(t, "share-id", resp.VolumeID)
				assert.Equal(t, int32(500), resp.VolumePerformance.Bandwidth)
			},
		},
		{
			testCaseName:   "ModifyShare client error returns FailedToModifyVolume",
			request:        provider.ModifyVolumeRequest{VolumeID: "share-id", VolumePerformance: provider.VolumePerformance{Iops: 5000}},
			modifyShareErr: errors.New("backend failure"),
			verify: func(t *testing.T, resp *provider.ModifyVolumeResponse, err error) {
				assert.Nil(t, resp)
				assert.NotNil(t, err)
				assert.Contains(t, err.Error(), "FailedToModifyVolume")
			},
		},
		{
			testCaseName:    "ModifyShare returns empty share ID returns FailedToModifyVolume",
			request:         provider.ModifyVolumeRequest{VolumeID: "share-id", VolumePerformance: provider.VolumePerformance{Iops: 5000}},
			modifyShareResp: &models.Share{
				// empty ID simulates empty share ID in response
			},
			verify: func(t *testing.T, resp *provider.ModifyVolumeResponse, err error) {
				assert.Nil(t, resp)
				assert.NotNil(t, err)
				assert.Contains(t, err.Error(), "FailedToModifyVolume")
			},
		},
		{
			testCaseName: "volume stuck in non-stable state returns VolumeNotInValidState",
			request:      provider.ModifyVolumeRequest{VolumeID: "share-id", VolumePerformance: provider.VolumePerformance{Iops: 5000}},
			modifyShareResp: &models.Share{
				ID:     "share-id",
				Status: models.StatusType("pending"),
				Iops:   5000,
			},
			getShareResp: &models.Share{
				ID:     "share-id",
				Status: models.StatusType("pending"),
			},
			verify: func(t *testing.T, resp *provider.ModifyVolumeResponse, err error) {
				assert.Nil(t, resp)
				assert.NotNil(t, err)
				assert.Contains(t, err.Error(), "VolumeNotInValidState")
			},
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

			if testcase.verify != nil {
				testcase.verify(t, resp, err)
			}
		})
	}
}

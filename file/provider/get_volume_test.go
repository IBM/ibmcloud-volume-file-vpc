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
	"errors"
	"testing"

	"github.com/IBM/ibmcloud-volume-file-vpc/common/vpcclient/models"
	fileShareServiceFakes "github.com/IBM/ibmcloud-volume-file-vpc/common/vpcclient/vpcfilevolume/fakes"
	"github.com/IBM/ibmcloud-volume-interface/lib/provider"
	util "github.com/IBM/ibmcloud-volume-interface/lib/utils"
	"github.com/IBM/ibmcloud-volume-interface/lib/utils/reasoncode"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

func TestGetVolume(t *testing.T) {
	//var err error
	logger, teardown := GetTestLogger(t)
	defer teardown()

	var (
		volumeService *fileShareServiceFakes.FileShareService
	)

	testCases := []struct {
		testCaseName string
		volumeID     string
		baseVolume   *models.Share

		setup func()

		skipErrTest        bool
		expectedErr        string
		expectedReasonCode string

		verify func(t *testing.T, volumeResponse *provider.Volume, err error)
	}{
		{
			testCaseName: "OK",
			volumeID:     "16f293bf-test-4bff-816f-e199c0c65db5",
			baseVolume: &models.Share{
				ID:     "16f293bf-test-4bff-816f-e199c0c65db5",
				Name:   "test-volume-name",
				Status: models.StatusType("OK"),
				Size:   int64(10),
				Iops:   int64(1000),
				Zone:   &models.Zone{Name: "test-zone"},
			},
			verify: func(t *testing.T, volumeResponse *provider.Volume, err error) {
				assert.NotNil(t, volumeResponse)
				assert.Nil(t, err)
			},
		}, {
			testCaseName: "Wrong volume ID",
			volumeID:     "Wrong volume ID",
			baseVolume: &models.Share{
				ID:     "wrong-wrong-id",
				Name:   "test-volume-name",
				Status: models.StatusType("OK"),
				Size:   int64(10),
				Iops:   int64(1000),
			},
			expectedErr:        "{Code:ErrorUnclassified, Type:InvalidRequest, Description:'Wrong volume ID' volume ID is not valid. Please check https://cloud.ibm.com/docs/infrastructure/vpc?topic=vpc-rias-error-messages#volume_id_invalid, BackendError:, RC:400}",
			expectedReasonCode: "ErrorUnclassified",
			verify: func(t *testing.T, volumeResponse *provider.Volume, err error) {
				assert.Nil(t, volumeResponse)
				assert.NotNil(t, err)
			},
		}, {
			testCaseName:       "Volume without zone",
			volumeID:           "16f293bf-test-4bff-816f-e199c0c65db5",
			expectedErr:        "{Code:ErrorUnclassified, Type:RetrivalFailed, Description:Failed to find '16f293bf-test-4bff-816f-e199c0c65db5' volume ID., BackendError:StorageFindFailedWithVolumeId, RC:404}",
			expectedReasonCode: "ErrorUnclassified",
			verify: func(t *testing.T, volumeResponse *provider.Volume, err error) {
				assert.Nil(t, volumeResponse)
				assert.NotNil(t, err)
			},
		},
	}

	for _, testcase := range testCases {
		t.Run(testcase.testCaseName, func(t *testing.T) {
			vpcs, uc, sc, err := GetTestOpenSession(t, logger)
			assert.NotNil(t, vpcs)
			assert.NotNil(t, uc)
			assert.NotNil(t, sc)
			assert.Nil(t, err)

			volumeService = &fileShareServiceFakes.FileShareService{}
			assert.NotNil(t, volumeService)
			uc.FileShareServiceReturns(volumeService)

			if testcase.expectedErr != "" {
				volumeService.GetFileShareReturns(testcase.baseVolume, errors.New(testcase.expectedReasonCode))
			} else {
				volumeService.GetFileShareReturns(testcase.baseVolume, nil)
			}
			volume, err := vpcs.GetVolume(testcase.volumeID)
			logger.Info("Volume details", zap.Reflect("volume", volume))

			if testcase.expectedErr != "" {
				assert.NotNil(t, err)
				logger.Info("Error details", zap.Reflect("Error details", err.Error()))
				assert.Equal(t, reasoncode.ReasonCode(testcase.expectedReasonCode), util.ErrorReasonCode(err))
			}

			if testcase.verify != nil {
				testcase.verify(t, volume, err)
			}
		})
	}
}

func TestGetVolumeByName(t *testing.T) {
	//var err error
	logger, teardown := GetTestLogger(t)
	defer teardown()

	var (
		volumeService *fileShareServiceFakes.FileShareService
	)

	testCases := []struct {
		testCaseName string
		volumeName   string
		baseVolume   *models.Share

		setup func()

		skipErrTest        bool
		expectedErr        string
		expectedReasonCode string

		verify func(t *testing.T, volumeResponse *provider.Volume, err error)
	}{
		{
			testCaseName: "OK",
			volumeName:   "Test volume",
			baseVolume: &models.Share{
				ID:     "16f293bf-test-4bff-816f-e199c0c65db5",
				Name:   "test-volume-name",
				Status: models.StatusType("OK"),
				Size:   int64(10),
				Iops:   int64(1000),
				Zone:   &models.Zone{Name: "test-zone"},
			},
			verify: func(t *testing.T, volumeResponse *provider.Volume, err error) {
				assert.NotNil(t, volumeResponse)
				assert.Nil(t, err)
			},
		}, {
			testCaseName: "Wrong volume ID",
			volumeName:   "Wrong volume name",
			baseVolume: &models.Share{
				ID:     "wrong-wrong-id",
				Name:   "test-volume-name",
				Status: models.StatusType("OK"),
				Size:   int64(10),
				Iops:   int64(1000),
			},
			expectedErr:        "{Code:ErrorUnclassified, Type:InvalidRequest, Description:'Wrong volume ID' volume ID is not valid. Please check https://cloud.ibm.com/docs/infrastructure/vpc?topic=vpc-rias-error-messages#volume_id_invalid, BackendError:, RC:400}",
			expectedReasonCode: "ErrorUnclassified",
			verify: func(t *testing.T, volumeResponse *provider.Volume, err error) {
				assert.Nil(t, volumeResponse)
				assert.NotNil(t, err)
			},
		}, {
			testCaseName:       "Volume without zone",
			volumeName:         "Test volume",
			expectedErr:        "{Code:ErrorUnclassified, Type:RetrivalFailed, Description:Failed to find '16f293bf-test-4bff-816f-e199c0c65db5' volume ID., BackendError:StorageFindFailedWithVolumeId, RC:404}",
			expectedReasonCode: "ErrorUnclassified",
			verify: func(t *testing.T, volumeResponse *provider.Volume, err error) {
				assert.Nil(t, volumeResponse)
				assert.NotNil(t, err)
			},
		}, {
			testCaseName:       "Empty volume name",
			volumeName:         "",
			expectedErr:        "{Code:ErrorUnclassified, Type:RetrivalFailed, Description:Failed to find '16f293bf-test-4bff-816f-e199c0c65db5' volume ID., BackendError:StorageFindFailedWithVolumeId, RC:404}",
			expectedReasonCode: "ErrorUnclassified",
			verify: func(t *testing.T, volumeResponse *provider.Volume, err error) {
				assert.Nil(t, volumeResponse)
				assert.NotNil(t, err)
			},
		},
	}

	for _, testcase := range testCases {
		t.Run(testcase.testCaseName, func(t *testing.T) {
			vpcs, uc, sc, err := GetTestOpenSession(t, logger)
			assert.NotNil(t, vpcs)
			assert.NotNil(t, uc)
			assert.NotNil(t, sc)
			assert.Nil(t, err)

			volumeService = &fileShareServiceFakes.FileShareService{}
			assert.NotNil(t, volumeService)
			uc.FileShareServiceReturns(volumeService)

			if testcase.expectedErr != "" {
				volumeService.GetFileShareByNameReturns(testcase.baseVolume, errors.New(testcase.expectedReasonCode))
			} else {
				volumeService.GetFileShareByNameReturns(testcase.baseVolume, nil)
			}
			volume, err := vpcs.GetVolumeByName(testcase.volumeName)
			logger.Info("Volume details", zap.Reflect("volume", volume))

			if testcase.expectedErr != "" {
				assert.NotNil(t, err)
				logger.Info("Error details", zap.Reflect("Error details", err.Error()))
				assert.Equal(t, reasoncode.ReasonCode(testcase.expectedReasonCode), util.ErrorReasonCode(err))
			}

			if testcase.verify != nil {
				testcase.verify(t, volume, err)
			}
		})
	}
}

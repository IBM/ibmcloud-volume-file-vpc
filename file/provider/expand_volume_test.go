/**
 * Copyright 2022 IBM Corp.
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

	"github.com/IBM/ibmcloud-volume-file-vpc/common/vpcclient/models"
	volumeServiceFakes "github.com/IBM/ibmcloud-volume-file-vpc/common/vpcclient/vpcfilevolume/fakes"
	"github.com/IBM/ibmcloud-volume-interface/lib/provider"
	util "github.com/IBM/ibmcloud-volume-interface/lib/utils"
	"github.com/IBM/ibmcloud-volume-interface/lib/utils/reasoncode"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

func TestExpandVolume(t *testing.T) {
	logger, teardown := GetTestLogger(t)
	defer teardown()

	var (
		fileShareService *volumeServiceFakes.FileShareService
	)

	testCases := []struct {
		testCaseName       string
		volumeID           string
		baseVolume         *models.Share
		newSize            int64
		expectedErr        string
		expectedSize       int64
		expectedReasonCode string
	}{
		{
			testCaseName: "New size greater than current size -> success",
			baseVolume: &models.Share{
				ID:     "16f293bf-test-4bff-816f-e199c0c65db5",
				Status: models.StatusType("stable"),
				Size:   int64(10),
				Iops:   int64(1000),
				Zone:   &models.Zone{Name: "test-zone"},
			},
			newSize:      20,
			expectedSize: 20,
		},
		{
			testCaseName: "New size same as current size -> success",
			baseVolume: &models.Share{
				ID:     "16f293bf-test-4bff-816f-e199c0c65db5",
				Status: models.StatusType("stable"),
				Size:   int64(10),
				Iops:   int64(1000),
				Zone:   &models.Zone{Name: "test-zone"},
			},
			newSize:      10,
			expectedSize: 10,
		},
		{
			testCaseName:       "volume not found",
			baseVolume:         nil,
			expectedErr:        "{Code:ErrorUnclassified, Type:InvalidRequest, Description:'Wrong volume ID' volume ID is not valid. Please check https://cloud.ibm.com/docs/infrastructure/vpc?topic=vpc-rias-error-messages#volume_id_invalid, BackendError:, RC:400}",
			expectedReasonCode: "ErrorUnclassified",
			newSize:            10,
			expectedSize:       -1,
		},
		{
			testCaseName: "New size less than current size",
			baseVolume: &models.Share{
				ID:     "16f293bf-test-4bff-816f-e199c0c65db5",
				Status: models.StatusType("stable"),
				Size:   int64(10),
				Iops:   int64(1000),
				Zone:   &models.Zone{Name: "test-zone"},
			},
			newSize:      5,
			expectedSize: 10,
		},
		{
			testCaseName: "Correct size but stuck in pending state",
			baseVolume: &models.Share{
				ID:     "16f293bf-test-4bff-816f-e199c0c65db5",
				Status: models.StatusType("pending"),
				Size:   int64(10),
				Iops:   int64(1000),
				Zone:   &models.Zone{Name: "test-zone"},
			},
			newSize:      20,
			expectedSize: -1,
		},
	}

	for _, testcase := range testCases {
		t.Run(testcase.testCaseName, func(t *testing.T) {
			logger.Info("Started")
			vpcs, uc, sc, err := GetTestOpenSession(t, logger)
			assert.NotNil(t, vpcs)
			assert.NotNil(t, uc)
			assert.NotNil(t, sc)
			assert.Nil(t, err)

			fileShareService = &volumeServiceFakes.FileShareService{}
			fmt.Println("Success volumeshareservice")
			assert.NotNil(t, fileShareService)
			uc.FileShareServiceReturns(fileShareService)

			if testcase.expectedErr != "" {
				fileShareService.GetFileShareReturns(testcase.baseVolume, errors.New(testcase.expectedReasonCode))
				fileShareService.ExpandVolumeReturns(testcase.baseVolume, errors.New(testcase.expectedReasonCode))
			} else {
				fileShareService.GetFileShareReturns(testcase.baseVolume, nil)
				fileShareService.ExpandVolumeReturns(testcase.baseVolume, nil)
			}
			requestExp := provider.ExpandVolumeRequest{VolumeID: testcase.baseVolume.ID, Capacity: testcase.newSize}
			size, err := vpcs.ExpandVolume(requestExp)

			if testcase.expectedErr != "" {
				assert.NotNil(t, err)
				logger.Info("Error details", zap.Reflect("Error details", err.Error()))
				assert.Equal(t, reasoncode.ReasonCode(testcase.expectedReasonCode), util.ErrorReasonCode(err))
			}
			assert.Equal(t, size, testcase.expectedSize)
		})
	}

}

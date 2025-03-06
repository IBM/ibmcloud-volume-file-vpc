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

func TestUpdateVolume(t *testing.T) {
	logger, teardown := GetTestLogger(t)
	defer teardown()

	var (
		fileShareService *volumeServiceFakes.FileShareService
	)

	testCases := []struct {
		testCaseName       string
		volumeID           string
		tags               []string
		baseVolume         *models.Share
		etag               string
		newSize            int64
		expectedErr        string
		expectedSize       int64
		expectedReasonCode string
	}{
		{
			testCaseName:       "volume not found",
			volumeID:           "16f293bf-test-4bff-816f-e199c0c65db5",
			baseVolume:         nil,
			expectedErr:        "{Trace Code:16f293bf-test-4bff-816f-e199c0c65db5, Code:share_not_found, Description:testerr, RC:404 Not Found}",
			expectedReasonCode: "ErrorUnclassified",
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
				fileShareService.GetFileShareEtagReturns(testcase.baseVolume, testcase.etag, errors.New(testcase.expectedReasonCode))
				fileShareService.UpdateFileShareWithEtagReturns(errors.New(testcase.expectedReasonCode))
			} else {
				fileShareService.GetFileShareEtagReturns(testcase.baseVolume, testcase.etag, nil)
				fileShareService.UpdateFileShareWithEtagReturns(nil)
			}

			requestExp := provider.Volume{VolumeID: testcase.volumeID,
				VPCVolume: provider.VPCVolume{Tags: testcase.tags}}

			err = vpcs.UpdateVolume(requestExp)

			if testcase.expectedErr != "" {
				assert.NotNil(t, err)
				logger.Info("Error details", zap.Reflect("Error details", err.Error()))
				assert.Equal(t, reasoncode.ReasonCode(testcase.expectedReasonCode), util.ErrorReasonCode(err))
			} else {
				assert.Nil(t, err)
			}
		})
	}

}

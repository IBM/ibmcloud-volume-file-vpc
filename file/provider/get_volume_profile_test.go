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

	fileShareServiceFakes "github.com/IBM/ibmcloud-volume-file-vpc/common/vpcclient/vpcfilevolume/fakes"
	util "github.com/IBM/ibmcloud-volume-interface/lib/utils"
	"github.com/IBM/ibmcloud-volume-interface/lib/utils/reasoncode"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

func TestGetVolumeProfileByName(t *testing.T) {
	//var err error
	logger, teardown := GetTestLogger(t)
	defer teardown()

	var (
		volumeService *fileShareServiceFakes.FileShareService
	)

	testCases := []struct {
		testCaseName      string
		volumeProfileName string

		setup func()

		skipErrTest        bool
		expectedErr        string
		expectedReasonCode string
	}{
		{
			testCaseName:      "OK",
			volumeProfileName: "rfs",
		}, {
			testCaseName:       "Wrong Profile Name",
			volumeProfileName:  "rfs2",
			expectedErr:        "{Code:ErrorUnclassified, Type:InvalidRequest, Description:'rfs2' volume profile name is not valid, BackendError:, RC:400}",
			expectedReasonCode: "ErrorUnclassified",
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
				volumeService.GetShareProfileReturns(errors.New(testcase.expectedReasonCode))
			} else {
				volumeService.GetShareProfileReturns(nil)
			}
			err := vpcs.GetShareProfile(testcase.volumeName)
			logger.Info("Volume details", zap.Reflect("volume", volume))

			if testcase.expectedErr != "" {
				assert.NotNil(t, err)
				logger.Info("Error details", zap.Reflect("Error details", err.Error()))
				assert.Equal(t, reasoncode.ReasonCode(testcase.expectedReasonCode), util.ErrorReasonCode(err))
			}
		})
	}
}

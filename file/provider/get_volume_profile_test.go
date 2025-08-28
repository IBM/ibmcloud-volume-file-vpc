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

// Package provider ...
package provider

import (
	"errors"
	"testing"

	"github.com/IBM/ibmcloud-volume-file-vpc/common/vpcclient/models"
	fileShareServiceFakes "github.com/IBM/ibmcloud-volume-file-vpc/common/vpcclient/vpcfilevolume/fakes"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

func TestGetVolumeProfileByName(t *testing.T) {
	logger, teardown := GetTestLogger(t)
	defer teardown()

	var (
		volumeService *fileShareServiceFakes.FileShareService
	)

	testCases := []struct {
		testCaseName      string
		volumeProfileName string
		baseProfile       *models.Profile

		setup func()

		skipErrTest        bool
		expectedErr        string
		expectedReasonCode string
	}{
		{
			testCaseName:      "OK",
			volumeProfileName: "rfs",
			baseProfile: &models.Profile{
				Name: "rfs",
				Href: "href",
				Capacity: models.CapIops{
					Default: 1,
					Max:     32000,
					Min:     1,
					Step:    1,
					Type:    "range",
				},
				Family:       "defined_performance",
				ResourceType: "share_profile",
			},
		}, {
			testCaseName:       "Wrong Profile Name",
			volumeProfileName:  "rfs2",
			expectedErr:        "{Code:InvalidRequest, Description:'rfs2' volume profile name is not valid, BackendError:, RC:400}",
			expectedReasonCode: "{Code:InvalidRequest, Description:'rfs2' volume profile name is not valid, BackendError:, RC:400}",
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
				volumeService.GetShareProfileReturns(nil, errors.New(testcase.expectedReasonCode))
			} else {
				volumeService.GetShareProfileReturns(testcase.baseProfile, nil)
			}
			profile, err := vpcs.GetVolumeProfileByName(testcase.volumeProfileName)

			if testcase.expectedErr != "" {
				assert.NotNil(t, err)
				logger.Info("Error details", zap.Reflect("Error details", err.Error()))
				assert.Equal(t, err.Error(), testcase.expectedErr)
			} else {
				assert.Nil(t, err)
				assert.NotNil(t, profile)
				assert.Equal(t, testcase.volumeProfileName, profile.Name)
			}
		})
	}
}

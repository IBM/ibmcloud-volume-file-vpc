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
	volumeServiceFakes "github.com/IBM/ibmcloud-volume-file-vpc/common/vpcclient/vpcfilevolume/fakes"
	"github.com/IBM/ibmcloud-volume-interface/lib/provider"
	util "github.com/IBM/ibmcloud-volume-interface/lib/utils"
	"github.com/IBM/ibmcloud-volume-interface/lib/utils/reasoncode"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

func TestGetVolumeAccessPoint(t *testing.T) {
	//var err error
	logger, teardown := GetTestLogger(t)
	defer teardown()

	var (
		volumeService *volumeServiceFakes.FileShareService
	)

	testCases := []struct {
		testCaseName                      string
		providerVolumeAccessPointRequest  provider.VolumeAccessPointRequest
		baseVolumeAccessPointResponse     *models.ShareTarget
		providerVolumeAccessPointResponse provider.VolumeAccessPointResponse
		volumeTargetList                  *models.ShareTargetList

		setup func(providerVolume *provider.Volume)

		skipErrTest        bool
		expectedErr        string
		expectedReasonCode string

		verify func(t *testing.T, volumeAccessPointResponse *provider.VolumeAccessPointResponse, err error)
	}{
		{
			testCaseName: "VPC ID and Target ID is nil",
			providerVolumeAccessPointRequest: provider.VolumeAccessPointRequest{
				VolumeID: "volume-id1",
			},
			verify: func(t *testing.T, volumeAccessPointResponse *provider.VolumeAccessPointResponse, err error) {
				assert.Nil(t, volumeAccessPointResponse)
				assert.NotNil(t, err)
			},
		}, {
			testCaseName: "Volume ID is nil",
			providerVolumeAccessPointRequest: provider.VolumeAccessPointRequest{
				VPCID:         "VPC-id1",
				AccessPointID: "target-id1",
			},
			verify: func(t *testing.T, volumeAccessPointResponse *provider.VolumeAccessPointResponse, err error) {
				assert.Nil(t, volumeAccessPointResponse)
				assert.NotNil(t, err)
			},
		},
		{
			testCaseName: "Volume Access Point exist for the VPCID",
			providerVolumeAccessPointRequest: provider.VolumeAccessPointRequest{
				VolumeID: "volume-id1",
				VPCID:    "VPC-id1",
			},

			baseVolumeAccessPointResponse: &models.ShareTarget{
				ID:        "16f293bf-test-4bff-816f-e199c0c65db5",
				MountPath: "abac:/asdsads/asdsad",
				Name:      "test volume name",
				Status:    "stable",
				VPC:       &provider.VPC{ID: "VPC-id1"},
				ShareID:   "",
				Zone:      &models.Zone{Name: "test-zone"},
			},

			volumeTargetList: &models.ShareTargetList{
				First: &models.HReference{Href: "https://eu-gb.iaas.cloud.ibm.com/v1/volumes?start=16f293bf-test-4bff-816f-e199c0c65db5\u0026limit=50"},
				Next:  nil,
				Limit: 50,
				ShareTargets: []*models.ShareTarget{
					{
						ID:        "16f293bf-test-4bff-816f-e199c0c65db5",
						MountPath: "abac:/asdsads/asdsad",
						Name:      "test volume name",
						Status:    "stable",
						VPC:       &provider.VPC{ID: "VPC-id1"},
						ShareID:   "",
						Zone:      &models.Zone{Name: "test-zone"},
					},
				},
			},

			verify: func(t *testing.T, volumeAccessPointResponse *provider.VolumeAccessPointResponse, err error) {
				assert.NotNil(t, volumeAccessPointResponse)
				assert.Nil(t, err)
			},
		},
		{
			testCaseName: "Volume Access Point does not exist for the VPC ID",
			providerVolumeAccessPointRequest: provider.VolumeAccessPointRequest{
				VolumeID: "volume-id1",
				VPCID:    "VPC-id1",
			},

			baseVolumeAccessPointResponse: nil,
			volumeTargetList:              nil,

			expectedErr:        "{Code:ErrorUnclassified, Type:RetrivalFailed, Description:Failed to find 'volume-id1' volume ID with 'VPC-id1' VPC ID ., BackendError:StorageFindFailedWithVolumeIdAndVPCId, RC:404}",
			expectedReasonCode: "ErrorUnclassified",

			verify: func(t *testing.T, volumeAccessPointResponse *provider.VolumeAccessPointResponse, err error) {
				assert.Nil(t, volumeAccessPointResponse)
				assert.NotNil(t, err)
			},
		},
		{
			testCaseName: "Volume Access Point does not exist for the TargetID",
			providerVolumeAccessPointRequest: provider.VolumeAccessPointRequest{
				VolumeID:      "volume-id1",
				AccessPointID: "target-id1",
			},

			baseVolumeAccessPointResponse: nil,
			volumeTargetList:              nil,

			expectedErr:        "{Code:ErrorUnclassified, Type:RetrivalFailed, Description:Failed to find 'volume-id1' volume ID with target 'target-id1' targer ID ., BackendError:StorageFindFailedWithVolumeIdAndTargetId, RC:404}",
			expectedReasonCode: "ErrorUnclassified",

			verify: func(t *testing.T, volumeAccessPointResponse *provider.VolumeAccessPointResponse, err error) {
				assert.Nil(t, volumeAccessPointResponse)
				assert.NotNil(t, err)
			},
		},
		{
			testCaseName: "Volume Access Point exist for the TargetID",
			providerVolumeAccessPointRequest: provider.VolumeAccessPointRequest{
				VolumeID:      "volume-id1",
				AccessPointID: "target-id1",
			},

			baseVolumeAccessPointResponse: &models.ShareTarget{
				ID:        "16f293bf-test-4bff-816f-e199c0c65db5",
				MountPath: "abac:/asdsads/asdsad",
				Name:      "test volume name",
				Status:    "stable",
				VPC:       &provider.VPC{ID: "VPC-id1"},
				ShareID:   "",
				Zone:      &models.Zone{Name: "test-zone"},
			},

			volumeTargetList: nil,

			verify: func(t *testing.T, volumeAccessPointResponse *provider.VolumeAccessPointResponse, err error) {
				assert.NotNil(t, volumeAccessPointResponse)
				assert.Nil(t, err)
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

			volumeService = &volumeServiceFakes.FileShareService{}
			assert.NotNil(t, volumeService)
			uc.FileShareServiceReturns(volumeService)

			if testcase.expectedErr != "" {
				volumeService.GetFileShareTargetReturns(testcase.baseVolumeAccessPointResponse, errors.New(testcase.expectedReasonCode))
				volumeService.ListFileShareTargetsReturns(testcase.volumeTargetList, errors.New(testcase.expectedReasonCode))
			} else {
				volumeService.GetFileShareTargetReturns(testcase.baseVolumeAccessPointResponse, nil)
				volumeService.ListFileShareTargetsReturns(testcase.volumeTargetList, nil)
			}
			volumeAccessPoint, err := vpcs.GetVolumeAccessPoint(testcase.providerVolumeAccessPointRequest)
			logger.Info("Volume access point details", zap.Reflect("VolumeAccessPointResponse", volumeAccessPoint))

			if testcase.expectedErr != "" {
				assert.NotNil(t, err)
				logger.Info("Error details", zap.Reflect("Error details", err.Error()))
				assert.Equal(t, reasoncode.ReasonCode(testcase.expectedReasonCode), util.ErrorReasonCode(err))
			}

			if testcase.verify != nil {
				testcase.verify(t, volumeAccessPoint, err)
			}
		})
	}
}

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

func TestCreateVolume(t *testing.T) {
	//var err error
	logger, teardown := GetTestLogger(t)
	defer teardown()

	var (
		volumeService *fileShareServiceFakes.FileShareService
		profileName   string
	)

	testCases := []struct {
		testCaseName   string
		baseVolume     *models.Share
		providerVolume provider.Volume
		profileName    string

		setup func(providerVolume *provider.Volume)

		skipErrTest        bool
		expectedErr        string
		expectedReasonCode string

		verify func(t *testing.T, volumeResponse *provider.Volume, err error)
	}{
		{
			testCaseName: "Volume capacity is nil",
			baseVolume: &models.Share{
				ID:     "16f293bf-test-4bff-816f-e199c0c65db5",
				Name:   "test volume name",
				Status: models.StatusType("OK"),
				Iops:   int64(1000),
				Zone:   &models.Zone{Name: "test-zone"},
			},
			providerVolume: provider.Volume{
				VolumeID: "16f293bf-test-4bff-816f-e199c0c65db5",
				Name:     String("test volume name"),
				Capacity: nil,
			},
			verify: func(t *testing.T, volumeResponse *provider.Volume, err error) {
				assert.Nil(t, volumeResponse)
				assert.NotNil(t, err)
			},
		}, {
			testCaseName: "Volume capacity is zero",
			providerVolume: provider.Volume{
				VolumeID: "16f293bf-test-4bff-816f-e199c0c65db5",
				Name:     String("test volume name"),
				Capacity: Int(0),
			},
			verify: func(t *testing.T, volumeResponse *provider.Volume, err error) {
				assert.Nil(t, volumeResponse)
				assert.NotNil(t, err)
			},
		}, {
			testCaseName: "Volume with tier-10iops profile and invalid iops",
			profileName:  "tier-10iops",
			providerVolume: provider.Volume{
				VolumeID: "16f293bf-test-4bff-816f-e199c0c65db5",
				Name:     String("test volume name"),
				Capacity: Int(10),
				Iops:     String("1000"),
				VPCVolume: provider.VPCVolume{
					Profile: &provider.Profile{Name: profileName},
				},
			},
			verify: func(t *testing.T, volumeResponse *provider.Volume, err error) {
				assert.Nil(t, volumeResponse)
				assert.NotNil(t, err)
			},
		}, {
			testCaseName: "Volume with no validation issues",
			profileName:  "tier-10iops",
			baseVolume: &models.Share{
				ID:     "16f293bf-test-4bff-816f-e199c0c65db5",
				Name:   "test-volume-name",
				Status: models.StatusType("stable"),
				Size:   int64(10),
				Iops:   int64(1000),
				Zone:   &models.Zone{Name: "test-zone"},
				ShareTargets: &[]models.ShareTarget{
					{
						ID: "testVolumeAccessPointId",
						VPC: &provider.VPC{
							ID: "1234",
						},
						Zone: &models.Zone{Name: "test-zone"},
					},
					{
						ID: "testVolumeAccessPointId",
						VPC: &provider.VPC{
							ID: "1234",
						},
						Zone: &models.Zone{Name: "test-zone"},
					},
				},
			},
			providerVolume: provider.Volume{
				VolumeID: "16f293bf-test-4bff-816f-e199c0c65db5",
				Name:     String("test volume name"),
				Capacity: Int(10),
				Iops:     String("0"),
				VPCVolume: provider.VPCVolume{
					Profile:       &provider.Profile{Name: profileName},
					ResourceGroup: &provider.ResourceGroup{ID: "default resource group id", Name: "default resource group"},
				},
			},
			verify: func(t *testing.T, volumeResponse *provider.Volume, err error) {
				assert.NotNil(t, volumeResponse)
				assert.Nil(t, err)
			},
		}, {
			testCaseName: "Volume profile is nil",
			providerVolume: provider.Volume{
				VolumeID: "16f293bf-test-4bff-816f-e199c0c65db5",
				Name:     String("test volume name"),
				Capacity: Int(10),
				Iops:     String("0"),
				VPCVolume: provider.VPCVolume{
					Profile:       nil,
					ResourceGroup: &provider.ResourceGroup{ID: "default resource group id", Name: "default resource group"},
				},
			},
			verify: func(t *testing.T, volumeResponse *provider.Volume, err error) {
				assert.Nil(t, volumeResponse)
				assert.NotNil(t, err)
			},
		}, {
			testCaseName: "Volume with VPC Mode",
			profileName:  "tier-10iops",
			baseVolume: &models.Share{
				ID:     "16f293bf-test-4bff-816f-e199c0c65db5",
				Name:   "test-volume-name",
				Status: models.StatusType("stable"),
				Size:   int64(10),
				Iops:   int64(1000),
				Zone:   &models.Zone{Name: "test-zone"},
				ShareTargets: &[]models.ShareTarget{
					{
						ID: "testVolumeAccessPointId",
						VPC: &provider.VPC{
							ID: "1234",
						},
						Zone: &models.Zone{Name: "test-zone"},
					},
				},
			},
			providerVolume: provider.Volume{
				VolumeID: "16f293bf-test-4bff-816f-e199c0c65db5",
				Name:     String("test volume name"),
				Capacity: Int(10),
				Iops:     String("0"),
				VPCVolume: provider.VPCVolume{
					Profile:       &provider.Profile{Name: profileName},
					ResourceGroup: &provider.ResourceGroup{ID: "default resource group id", Name: "default resource group"},
					VPCFileVolume: provider.VPCFileVolume{
						AccessControlMode: "VPC",
						VPCID:             "VPC-id1",
					},
				},
			},
			verify: func(t *testing.T, volumeResponse *provider.Volume, err error) {
				assert.NotNil(t, volumeResponse)
				assert.Nil(t, err)
			},
		}, {
			testCaseName: "Volume with securityGroup Mode",
			profileName:  "tier-10iops",
			baseVolume: &models.Share{
				ID:     "16f293bf-test-4bff-816f-e199c0c65db5",
				Name:   "test-volume-name",
				Status: models.StatusType("stable"),
				Size:   int64(10),
				Iops:   int64(1000),
				Zone:   &models.Zone{Name: "test-zone"},
				ShareTargets: &[]models.ShareTarget{
					{
						ID: "testVolumeAccessPointId",
						VPC: &provider.VPC{
							ID: "1234",
						},
						Zone: &models.Zone{Name: "test-zone"},
					},
				},
			},
			providerVolume: provider.Volume{
				VolumeID: "16f293bf-test-4bff-816f-e199c0c65db5",
				Name:     String("test volume name"),
				Capacity: Int(10),
				Iops:     String("0"),
				VPCVolume: provider.VPCVolume{
					Profile:       &provider.Profile{Name: profileName},
					ResourceGroup: &provider.ResourceGroup{ID: "default resource group id", Name: "default resource group"},
					VPCFileVolume: provider.VPCFileVolume{
						AccessControlMode: "security_group",
						VPCID:             "VPC-id1",
						TransitEncryption: "user_managed",
						SecurityGroups: &[]provider.SecurityGroup{
							{
								ID: "securityGroup-1",
							},
							{
								ID: "securityGroup-2",
							},
						},
						PrimaryIP: &provider.PrimaryIP{
							PrimaryIPID: provider.PrimaryIPID{
								ID: "primary-ip-id-1",
							},
						},
						SubnetID: "subnetID-1",
					},
				},
			},
			verify: func(t *testing.T, volumeResponse *provider.Volume, err error) {
				assert.NotNil(t, volumeResponse)
				assert.Nil(t, err)
			},
		}, {
			testCaseName: "Volume creation failure",
			profileName:  "tier-10iops",
			providerVolume: provider.Volume{
				VolumeID: "16f293bf-test-4bff-816f-e199c0c65db5",
				Name:     String("test volume name"),
				Capacity: Int(10),
				Iops:     String("0"),
				VPCVolume: provider.VPCVolume{
					Profile:       &provider.Profile{Name: profileName},
					ResourceGroup: &provider.ResourceGroup{ID: "default resource group id", Name: "default resource group"},
				},
			},
			expectedErr:        "{Code:ErrorUnclassified, Type:InvalidRequest, Description: Volume creation failed. ",
			expectedReasonCode: "ErrorUnclassified",
			verify: func(t *testing.T, volumeResponse *provider.Volume, err error) {
				assert.Nil(t, volumeResponse)
				assert.NotNil(t, err)
			},
		}, {
			testCaseName: "Volume creation with encryption",
			profileName:  "tier-10iops",
			baseVolume: &models.Share{
				ID:     "16f293bf-test-4bff-816f-e199c0c65db5",
				Name:   "test-volume-name",
				Status: models.StatusType("stable"),
				Size:   int64(10),
				Iops:   int64(1000),
				Zone:   &models.Zone{Name: "test-zone"},
			},
			providerVolume: provider.Volume{
				VolumeID: "16f293bf-test-4bff-816f-e199c0c65db5",
				Name:     String("test volume name"),
				Capacity: Int(10),
				Iops:     String("0"),
				VPCVolume: provider.VPCVolume{
					Profile:             &provider.Profile{Name: profileName},
					ResourceGroup:       &provider.ResourceGroup{ID: "default resource group id", Name: "default resource group"},
					VolumeEncryptionKey: &provider.VolumeEncryptionKey{CRN: "crn:v1:bluemix:public:kms:us-south:a/abcd32a619db2b564b82a816400bcd12:t36097fd-5051-4582-a641-8f51b5334cfa:key:abc05f428-5fb7-4546-958b-0f4e65266d5c"},
				},
			},
			verify: func(t *testing.T, volumeResponse *provider.Volume, err error) {
				assert.NotNil(t, volumeResponse)
				assert.Nil(t, err)
			},
		}, {
			testCaseName: "Volume creation with resource group ID and Name empty",
			profileName:  "tier-10iops",
			providerVolume: provider.Volume{
				VolumeID: "16f293bf-test-4bff-816f-e199c0c65db5",
				Name:     String("test volume name"),
				Capacity: Int(10),
				Iops:     String("0"),
				VPCVolume: provider.VPCVolume{
					Profile:       &provider.Profile{Name: profileName},
					ResourceGroup: &provider.ResourceGroup{},
				},
			},
			expectedErr:        "{Code:ErrorUnclassified, Type:InvalidRequest, Description: Volume creation failed. ",
			expectedReasonCode: "ErrorUnclassified",
			verify: func(t *testing.T, volumeResponse *provider.Volume, err error) {
				assert.Nil(t, volumeResponse)
				assert.NotNil(t, err)
			},
		}, {
			testCaseName: "Volume with test-purpose profile and invalid iops",
			profileName:  "tier-10iops",
			providerVolume: provider.Volume{
				VolumeID: "16f293bf-test-4bff-816f-e199c0c65db5",
				Name:     String("test volume name"),
				Capacity: Int(10),
				VPCVolume: provider.VPCVolume{
					Profile: &provider.Profile{Name: profileName},
				},
			},
			expectedErr:        "{Code:ErrorUnclassified, Type:InvalidRequest, Description: Volume creation failed. ",
			expectedReasonCode: "ErrorUnclassified",
			verify: func(t *testing.T, volumeResponse *provider.Volume, err error) {
				assert.Nil(t, volumeResponse)
				assert.NotNil(t, err)
			},
		}, {
			testCaseName: "Volume creation failure",
			profileName:  "tier-10iops",
			providerVolume: provider.Volume{
				VolumeID: "16f293bf-test-4bff-816f-e199c0c65db5",
				Name:     String("test volume name"),
				Capacity: Int(10),
				Iops:     String("0"),
				VPCVolume: provider.VPCVolume{
					Profile:       &provider.Profile{Name: profileName},
					ResourceGroup: &provider.ResourceGroup{ID: "default resource group id", Name: "default resource group"},
				},
			},
			expectedErr:        "{Code:ErrorUnclassified, Type:InvalidRequest, Description: Volume creation failed. ",
			expectedReasonCode: "ErrorUnclassified",
			verify: func(t *testing.T, volumeResponse *provider.Volume, err error) {
				assert.Nil(t, volumeResponse)
				assert.NotNil(t, err)
			},
		}, {
			testCaseName: "Volume name is nil",
			providerVolume: provider.Volume{
				VolumeID: "16f293bf-test-4bff-816f-e199c0c65db5",
			},
			verify: func(t *testing.T, volumeResponse *provider.Volume, err error) {
				assert.Nil(t, volumeResponse)
				assert.NotNil(t, err)
			},
		}, {
			testCaseName: "Volume name is empty",
			baseVolume: &models.Share{
				ID:     "16f293bf-test-4bff-816f-e199c0c65db5",
				Status: models.StatusType("OK"),
				Name:   "",
				Iops:   int64(1000),
				Zone:   &models.Zone{Name: "test-zone"},
			},
			providerVolume: provider.Volume{
				VolumeID: "16f293bf-test-4bff-816f-e199c0c65db5",
				Name:     String(""),
			},
			verify: func(t *testing.T, volumeResponse *provider.Volume, err error) {
				assert.Nil(t, volumeResponse)
				assert.NotNil(t, err)
			},
		},

		{
			testCaseName: "Volume in pending state",
			profileName:  "tier-10iops",
			baseVolume: &models.Share{
				ID:     "16f293bf-test-4bff-816f-e199c0c65db5",
				Name:   "test-volume-name",
				Status: models.StatusType("pending"),
				Size:   int64(10),
				Iops:   int64(1000),
				Zone:   &models.Zone{Name: "test-zone"},
			},
			providerVolume: provider.Volume{
				VolumeID: "16f293bf-test-4bff-816f-e199c0c65db5",
				Name:     String("test volume name"),
				Capacity: Int(10),
				Iops:     String("0"),
				VPCVolume: provider.VPCVolume{
					Profile:       &provider.Profile{Name: profileName},
					ResourceGroup: &provider.ResourceGroup{ID: "default resource group id", Name: "default resource group"},
				},
			},
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
				volumeService.CreateFileShareReturns(testcase.baseVolume, errors.New(testcase.expectedReasonCode))
				volumeService.GetFileShareReturns(testcase.baseVolume, errors.New(testcase.expectedReasonCode))
			} else {
				volumeService.CreateFileShareReturns(testcase.baseVolume, nil)
				volumeService.GetFileShareReturns(testcase.baseVolume, nil)
			}
			volume, err := vpcs.CreateVolume(testcase.providerVolume)
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

// String returns a pointer to the string value provided
func String(v string) *string {
	return &v
}

// Int returns a pointer to the int value provided
func Int(v int) *int {
	return &v
}

// Int64 returns a pointer to the int64 value provided
func Int64(v int64) *int64 {
	return &v
}

/**
<<<<<<< HEAD
<<<<<<< HEAD
 * Copyright 2023 IBM Corp.
=======
 * Copyright 2021 IBM Corp.
>>>>>>> adfefb0 (ENI support)
=======
 * Copyright 2023 IBM Corp.
>>>>>>> 2dc2601 (Adding support for GetSecurityGroupByName)
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

func TestGetSubnet(t *testing.T) {
	//var err error
	logger, teardown := GetTestLogger(t)
	defer teardown()

	var (
		volumeService *fileShareServiceFakes.FileShareService
	)

	testCases := []struct {
		testCaseName string
		subnetReq    provider.SubnetRequest
		subnetList   *models.SubnetList

		setup func()

		skipErrTest        bool
		expectedErr        string
		expectedReasonCode string

		verify func(t *testing.T, subnet string, err error)
	}{
		{
			testCaseName: "OK",
			subnetReq: provider.SubnetRequest{
				SubnetIDList: "16f293bf-test-4bff-816f-e199c0c65db5,16f293bf-test-4bff-816f-e199c0c65db2",
				ZoneName:     "test-zone",
				VPCID:        "VPC-id1",
				ResourceGroup: &provider.ResourceGroup{
					ID: "16f293bf-test-4bff-816f-e199c0wdwd5db5",
				},
			},

			subnetList: &models.SubnetList{
				Limit: 50,
				Subnets: []models.Subnet{
					{
						ID:   "16f293bf-test-4bff-816f-e199c0c65db5",
						VPC:  &provider.VPC{ID: "VPC-id1"},
						Zone: &models.Zone{Name: "test-zone"},
					},
				},
			},
			verify: func(t *testing.T, subnet string, err error) {
				assert.Equal(t, "16f293bf-test-4bff-816f-e199c0c65db5", subnet)
				assert.Nil(t, err)
			},
		}, {
			testCaseName: "Wrong subnetIDList",
			subnetReq: provider.SubnetRequest{
				SubnetIDList: "16f293bf-test-4bff-816f-e199c0c65db5ss,16f293bf-test-4bff-816f-e199c0c65db2",
				ZoneName:     "test-zone",
				VPCID:        "VPC-id1",
				ResourceGroup: &provider.ResourceGroup{
					ID: "16f293bf-test-4bff-816f-e199c0wdwd5db5",
				},
			},
			subnetList: &models.SubnetList{
				Limit: 50,
				Subnets: []models.Subnet{
					{
						ID:   "16f293bf-test-4bff-816f-e199c0c65db5",
						VPC:  &provider.VPC{ID: "VPC-id1"},
						Zone: &models.Zone{Name: "test-zone"},
					},
				},
			},
			expectedErr:        "{Code:ErrorUnclassified, Type:InvalidRequest, Description:'A subnet with the specified zone test-zone and available cluster subnet list {16f293bf-test-4bff-816f-e199c0c65db5ss,16f293bf-test-4bff-816f-e199c0c65db2} could not be found.",
			expectedReasonCode: "ErrorUnclassified",
			verify: func(t *testing.T, subnet string, err error) {
				assert.Equal(t, "", subnet)
				assert.NotNil(t, err)
			},
		},
		{
			testCaseName: "Wrong zone",
			subnetReq: provider.SubnetRequest{
				SubnetIDList: "16f293bf-test-4bff-816f-e199c0c65db5,16f293bf-test-4bff-816f-e199c0c65db2",
				ZoneName:     "test-zone-1",
				VPCID:        "VPC-id1",
				ResourceGroup: &provider.ResourceGroup{
					ID: "16f293bf-test-4bff-816f-e199c0wdwd5db5",
				},
			},
			subnetList: &models.SubnetList{
				Limit: 50,
				Subnets: []models.Subnet{
					{
						ID:   "16f293bf-test-4bff-816f-e199c0c65db5",
						VPC:  &provider.VPC{ID: "VPC-id1"},
						Zone: &models.Zone{Name: "test-zone"},
					},
				},
			},
			expectedErr:        "{Code:ErrorUnclassified, Type:InvalidRequest, Description:'A subnet with the specified zone test-zone and available cluster subnet list {16f293bf-test-4bff-816f-e199c0c65db5ss,16f293bf-test-4bff-816f-e199c0c65db2} could not be found.",
			expectedReasonCode: "ErrorUnclassified",
			verify: func(t *testing.T, subnet string, err error) {
				assert.Equal(t, "", subnet)
				assert.NotNil(t, err)
			},
		},
		{
			testCaseName: "Wrong VPC",
			subnetReq: provider.SubnetRequest{
				SubnetIDList: "16f293bf-test-4bff-816f-e199c0c65db5,16f293bf-test-4bff-816f-e199c0c65db2",
				ZoneName:     "test-zone",
				VPCID:        "VPC-id2",
				ResourceGroup: &provider.ResourceGroup{
					ID: "16f293bf-test-4bff-816f-e199c0wdwd5db5",
				},
			},
			subnetList: &models.SubnetList{
				Limit: 50,
				Subnets: []models.Subnet{
					{
						ID:   "16f293bf-test-4bff-816f-e199c0c65db5",
						VPC:  &provider.VPC{ID: "VPC-id1"},
						Zone: &models.Zone{Name: "test-zone"},
					},
				},
			},
			expectedErr:        "{Code:ErrorUnclassified, Type:InvalidRequest, Description:'A subnet with the specified zone test-zone and available cluster subnet list {16f293bf-test-4bff-816f-e199c0c65db5ss,16f293bf-test-4bff-816f-e199c0c65db2} could not be found.",
			expectedReasonCode: "ErrorUnclassified",
			verify: func(t *testing.T, subnet string, err error) {
				assert.Equal(t, "", subnet)
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
				volumeService.ListSubnetsReturns(testcase.subnetList, errors.New(testcase.expectedReasonCode))
			} else {
				volumeService.ListSubnetsReturns(testcase.subnetList, nil)
			}
			subnet, err := vpcs.GetSubnetForVolumeAccessPoint(testcase.subnetReq)
			logger.Info("Subnet details", zap.Reflect("subnet", subnet))

			if testcase.expectedErr != "" {
				assert.NotNil(t, err)
				logger.Info("Error details", zap.Reflect("Error details", err.Error()))
				assert.Equal(t, reasoncode.ReasonCode(testcase.expectedReasonCode), util.ErrorReasonCode(err))
			}

			if testcase.verify != nil {
				testcase.verify(t, subnet, err)
			}
		})
	}
}

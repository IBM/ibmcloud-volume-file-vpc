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

func TestGetSecurityGroup(t *testing.T) {
	//var err error
	logger, teardown := GetTestLogger(t)
	defer teardown()

	var (
		volumeService *fileShareServiceFakes.FileShareService
	)

	testCases := []struct {
		testCaseName      string
		securityGroupReq  provider.SecurityGroupRequest
		securityGroupList *models.SecurityGroupList

		setup func()

		skipErrTest        bool
		expectedErr        string
		expectedReasonCode string

		verify func(t *testing.T, securityGroup string, err error)
	}{
		{
			testCaseName: "OK",
			securityGroupReq: provider.SecurityGroupRequest{
				Name:  "kube-cluster-1",
				VPCID: "VPC-id1",
				ResourceGroup: &provider.ResourceGroup{
					ID: "16f293bf-test-4bff-816f-e199c0wdwd5db5",
				},
			},

			securityGroupList: &models.SecurityGroupList{
				Limit: 50,
				SecurityGroups: []models.SecurityGroup{
					{
						ID:   "kube-cluster-1",
						VPC:  &provider.VPC{ID: "VPC-id1"},
						Name: "kube-cluster-1",
					},
				},
			},
			verify: func(t *testing.T, securityGroup string, err error) {
				assert.Equal(t, "kube-cluster-1", securityGroup)
				assert.Nil(t, err)
			},
		}, {
			testCaseName: "Wrong securityGroupName",
			securityGroupReq: provider.SecurityGroupRequest{
				Name:  "kube-cluster-2",
				VPCID: "VPC-id1",
				ResourceGroup: &provider.ResourceGroup{
					ID: "16f293bf-test-4bff-816f-e199c0wdwd5db5",
				},
			},
			securityGroupList: &models.SecurityGroupList{
				Limit: 50,
				SecurityGroups: []models.SecurityGroup{
					{
						ID:   "kube-cluster-1",
						VPC:  &provider.VPC{ID: "VPC-id1"},
						Name: "kube-cluster-1",
					},
				},
			},
			expectedErr:        "{Code:ErrorUnclassified, Type:InvalidRequest, Description:'A securityGroup with the specified zone test-zone and available cluster securityGroup list {16f293bf-test-4bff-816f-e199c0c65db5ss,16f293bf-test-4bff-816f-e199c0c65db2} could not be found.",
			expectedReasonCode: "ErrorUnclassified",
			verify: func(t *testing.T, securityGroup string, err error) {
				assert.Equal(t, "", securityGroup)
				assert.NotNil(t, err)
			},
		},
		{
			testCaseName: "Wrong VPC",
			securityGroupReq: provider.SecurityGroupRequest{
				Name:  "kube-cluster-1",
				VPCID: "VPC-id2",
				ResourceGroup: &provider.ResourceGroup{
					ID: "16f293bf-test-4bff-816f-e199c0wdwd5db5",
				},
			},
			securityGroupList: &models.SecurityGroupList{
				Limit: 50,
				SecurityGroups: []models.SecurityGroup{
					{
						ID:   "kube-cluster-1",
						VPC:  &provider.VPC{ID: "VPC-id1"},
						Name: "kube-cluster-1",
					},
				},
			},
			expectedErr:        "{Code:ErrorUnclassified, Type:InvalidRequest, Description:'A securityGroup with the specified zone test-zone and available cluster securityGroup list {16f293bf-test-4bff-816f-e199c0c65db5ss,16f293bf-test-4bff-816f-e199c0c65db2} could not be found.",
			expectedReasonCode: "ErrorUnclassified",
			verify: func(t *testing.T, securityGroup string, err error) {
				assert.Equal(t, "", securityGroup)
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
				volumeService.ListSecurityGroupsReturns(testcase.securityGroupList, errors.New(testcase.expectedReasonCode))
			} else {
				volumeService.ListSecurityGroupsReturns(testcase.securityGroupList, nil)
			}
			securityGroup, err := vpcs.GetSecurityGroupForVolumeAccessPoint(testcase.securityGroupReq)
			logger.Info("SecurityGroup details", zap.Reflect("securityGroup", securityGroup))

			if testcase.expectedErr != "" {
				assert.NotNil(t, err)
				logger.Info("Error details", zap.Reflect("Error details", err.Error()))
				assert.Equal(t, reasoncode.ReasonCode(testcase.expectedReasonCode), util.ErrorReasonCode(err))
			}

			if testcase.verify != nil {
				testcase.verify(t, securityGroup, err)
			}
		})
	}
}

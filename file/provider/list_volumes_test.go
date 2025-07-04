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
	"strings"
	"testing"

	"github.com/IBM/ibmcloud-volume-file-vpc/common/vpcclient/models"
	fileShareServiceFakes "github.com/IBM/ibmcloud-volume-file-vpc/common/vpcclient/vpcfilevolume/fakes"
	"github.com/IBM/ibmcloud-volume-interface/lib/provider"
	util "github.com/IBM/ibmcloud-volume-interface/lib/utils"
	"github.com/IBM/ibmcloud-volume-interface/lib/utils/reasoncode"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

func TestListVolumes(t *testing.T) {
	//var err error
	logger, teardown := GetTestLogger(t)
	defer teardown()

	var (
		volumeService *fileShareServiceFakes.FileShareService
	)

	testCases := []struct {
		testCaseName string
		volumeList   *models.ShareList

		limit int
		start string
		tags  map[string]string

		setup func()

		skipErrTest        bool
		expectedErr        string
		expectedReasonCode string

		verify func(t *testing.T, next_token string, volumes *provider.VolumeList, err error)
	}{
		{
			testCaseName: "Filter by name",
			volumeList: &models.ShareList{
				First: &models.HReference{Href: "https://eu-gb.iaas.cloud.ibm.com/v1/volumes?start=16f293bf-test-4bff-816f-e199c0c65db5\u0026limit=50"},
				Next:  nil,
				Limit: 50,
				Shares: []*models.Share{
					{
						ID:     "16f293bf-test-4bff-816f-e199c0c65db5",
						Name:   "test-volume-name1",
						Status: models.StatusType("OK"),
						Size:   int64(10),
						Iops:   int64(1000),
						Zone:   &models.Zone{Name: "test-zone"},
					},
				},
			},
			tags: map[string]string{
				"name": "test-volume-name1",
			},
			verify: func(t *testing.T, next_token string, volumes *provider.VolumeList, err error) {
				assert.NotNil(t, volumes.Volumes)
				assert.Equal(t, next_token, volumes.Next)
				assert.Nil(t, err)
			},
		}, {
			testCaseName: "Filter by name: volume not found",
			tags: map[string]string{
				"name": "test-volume-name1",
			},
			expectedErr:        "{Code:ErrorUnclassified, Type:RetrivalFailed, Description: Unable to fetch list of volumes. ",
			expectedReasonCode: "ErrorUnclassified",
			verify: func(t *testing.T, next_token string, volumes *provider.VolumeList, err error) {
				assert.Nil(t, volumes)
				assert.NotNil(t, err)
			},
		}, {
			testCaseName: "Filter by resource group ID",
			volumeList: &models.ShareList{
				First: &models.HReference{Href: "https://eu-gb.iaas.cloud.ibm.com/v1/volumes?start=16f293bf-test-4bff-816f-e199c0c65db5\u0026limit=50"},
				Next:  nil,
				Limit: 50,
				Shares: []*models.Share{
					{
						ID:     "16f293bf-test-4bff-816f-e199c0c65db5",
						Name:   "test-volume-name1",
						Status: models.StatusType("OK"),
						Size:   int64(10),
						Iops:   int64(1000),
						Zone:   &models.Zone{Name: "test-zone-1"},
					}, {
						ID:     "23b154fr-test-4bff-816f-f213s1y34gj8",
						Name:   "test-volume-name2",
						Status: models.StatusType("OK"),
						Size:   int64(10),
						Iops:   int64(1000),
						Zone:   &models.Zone{Name: "test-zone-2"},
					},
				},
			},
			tags: map[string]string{
				"resource_group.id": "12345xy4567z89776",
			},
			verify: func(t *testing.T, next_token string, volumes *provider.VolumeList, err error) {
				assert.NotNil(t, volumes.Volumes)
				assert.Equal(t, next_token, volumes.Next)
				assert.Nil(t, err)
			},
		}, {
			testCaseName: "Filter by resource group ID: no volume found",
			volumeList: &models.ShareList{
				First:  &models.HReference{Href: "https://eu-gb.iaas.cloud.ibm.com/v1/volumes?limit=50"},
				Next:   nil,
				Limit:  50,
				Shares: []*models.Share{},
			},
			tags: map[string]string{
				"resource_group.id": "12345xy4567z89776",
			},
			verify: func(t *testing.T, next_token string, volumes *provider.VolumeList, err error) {
				assert.Nil(t, volumes.Volumes)
				assert.Equal(t, next_token, volumes.Next)
				assert.Nil(t, err)
			},
		}, {
			testCaseName: "List all volumes",
			volumeList: &models.ShareList{
				First: &models.HReference{Href: "https://eu-gb.iaas.cloud.ibm.com/v1/volumes?start=16f293bf-test-4bff-816f-e199c0c65db5\u0026limit=50"},
				Next:  nil,
				Limit: 50,
				Shares: []*models.Share{
					{
						ID:     "16f293bf-test-4bff-816f-e199c0c65db5",
						Name:   "test-volume-name1",
						Status: models.StatusType("OK"),
						Size:   int64(10),
						Iops:   int64(1000),
						Zone:   &models.Zone{Name: "test-zone-1"},
					}, {
						ID:     "23b154fr-test-4bff-816f-f213s1y34gj8",
						Name:   "test-volume-name2",
						Status: models.StatusType("OK"),
						Size:   int64(10),
						Iops:   int64(1000),
						Zone:   &models.Zone{Name: "test-zone-2"},
					},
				},
			},
			verify: func(t *testing.T, next_token string, volumes *provider.VolumeList, err error) {
				assert.NotNil(t, volumes.Volumes)
				assert.Equal(t, next_token, volumes.Next)
				assert.Nil(t, err)
			},
		}, {
			testCaseName: "Unexpected format of 'Next' parameter in ListVolumes response",
			volumeList: &models.ShareList{
				First: &models.HReference{Href: "https://eu-gb.iaas.cloud.ibm.com/v1/volumes?start=16f293bf-test-4bff-816f-e199c0c65db5\u0026limit=50"},
				Next:  &models.HReference{Href: "https://eu-gb.iaas.cloud.ibm.com/v1/volumes?invalid=16f293bf-test-4bff-816f-e199c0c65db5\u0026limit=50"},
				Limit: 1,
				Shares: []*models.Share{
					{
						ID:     "16f293bf-test-4bff-816f-e199c0c65db5",
						Name:   "test-volume-name1",
						Status: models.StatusType("OK"),
						Size:   int64(10),
						Iops:   int64(1000),
						Zone:   &models.Zone{Name: "test-zone-1"},
					}, {
						ID:     "23b154fr-test-4bff-816f-f213s1y34gj8",
						Name:   "test-volume-name2",
						Status: models.StatusType("OK"),
						Size:   int64(10),
						Iops:   int64(1000),
						Zone:   &models.Zone{Name: "test-zone-2"},
					},
				},
			},
			limit: 1,
			verify: func(t *testing.T, next_token string, volumes *provider.VolumeList, err error) {
				assert.NotNil(t, volumes.Volumes)
				assert.Equal(t, next_token, volumes.Next)
				assert.Nil(t, err)
			},
		}, {
			testCaseName: "Invalid limit value",
			limit:        -1,
			verify: func(t *testing.T, next_token string, volumes *provider.VolumeList, err error) {
				assert.Nil(t, volumes)
				assert.Error(t, err)
			},
		}, {
			testCaseName:       "Invalid start volume ID",
			start:              "invalid-start-vol-id",
			expectedErr:        "{Code:ErrorUnclassified, Type:InvalidRequest, Description: The volume with the ID specified as the page " + startVolumeIDNotFoundMsg + ".",
			expectedReasonCode: "ErrorUnclassified",
			verify: func(t *testing.T, next_token string, volumes *provider.VolumeList, err error) {
				assert.Nil(t, volumes)
				assert.Error(t, err)
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
				volumeService.ListFileSharesReturns(testcase.volumeList, errors.New(testcase.expectedErr))
			} else {
				volumeService.ListFileSharesReturns(testcase.volumeList, nil)
			}
			volumes, err := vpcs.ListVolumes(testcase.limit, testcase.start, testcase.tags)
			logger.Info("VolumesList details", zap.Reflect("VolumesList", volumes))

			if testcase.expectedErr != "" {
				assert.NotNil(t, err)
				logger.Info("Error details", zap.Reflect("Error details", err.Error()))
				assert.Equal(t, reasoncode.ReasonCode(testcase.expectedReasonCode), util.ErrorReasonCode(err))
			}

			if testcase.verify != nil {
				var next string
				if testcase.volumeList != nil {
					if testcase.volumeList.Next != nil {
						// "Next":{"href":"https://eu-gb.iaas.cloud.ibm.com/v1/volumes?start=3e898aa7-ac71-4323-952d-a8d741c65a68\u0026limit=1\u0026zone.name=eu-gb-1"}
						if strings.Contains(testcase.volumeList.Next.Href, "start=") {
							next = strings.Split(strings.Split(testcase.volumeList.Next.Href, "start=")[1], "\u0026")[0]
						}
					}
				}
				testcase.verify(t, next, volumes, err)
				if volumes != nil && volumes.Volumes != nil {
					for index, vol := range volumes.Volumes {
						assert.Equal(t, testcase.volumeList.Shares[index].ID, vol.VolumeID)
						assert.Equal(t, testcase.volumeList.Shares[index].Size, int64(*vol.Capacity))

						iops := *vol.Iops
						assert.Equal(t, testcase.volumeList.Shares[index].Iops, iops)
						assert.Equal(t, testcase.volumeList.Shares[index].Zone, &models.Zone{Name: *vol.Az})
					}
				}
			}
		})
	}
}

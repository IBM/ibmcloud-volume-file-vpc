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

package vpcfilevolume_test

import (
	"net/http"
	"testing"

	"github.com/IBM/ibmcloud-volume-file-vpc/common/vpcclient/models"
	"github.com/IBM/ibmcloud-volume-file-vpc/common/vpcclient/riaas/test"
	"github.com/IBM/ibmcloud-volume-file-vpc/common/vpcclient/vpcfilevolume"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

func TestCreateFileShare(t *testing.T) {
	// Setup new style zap logger
	logger, _ := GetTestContextLogger()
	defer logger.Sync()

	testCases := []struct {
		name string

		// Response
		status  int
		content string

		// Expected return
		expectErr string
		verify    func(*testing.T, *models.Share, error)
	}{
		{
			name:   "Verify that the correct endpoint is invoked",
			status: http.StatusNoContent,
		}, {
			name:      "Verify that a 404 is returned to the caller",
			status:    http.StatusNotFound,
			content:   "{\"errors\":[{\"message\":\"testerr\",\"Code\":\"share_not_found\"}], \"trace\":\"2af63776-4df7-4970-b52d-4e25676ec0e4\"}",
			expectErr: "Trace Code:2af63776-4df7-4970-b52d-4e25676ec0e4, Code:share_not_found, Description:testerr, RC:404 Not Found",
		}, {
			name:    "Verify that the share is parsed correctly",
			status:  http.StatusOK,
			content: "{\"id\":\"share-id\",\"name\":\"share-name\",\"size\":10,\"iops\":3000,\"status\":\"pending\",\"zone\":{\"name\":\"test-1\",\"href\":\"https://us-south.iaas.cloud.ibm.com/v1/regions/us-south/zones/test-1\"},\"crn\":\"crn:v1:bluemix:public:is:test-1:a/rg1::share:vol1\"}",
			verify: func(t *testing.T, share *models.Share, err error) {
				if assert.NotNil(t, share) {
					assert.Equal(t, "share-id", share.ID)
				}
			},
		}, {
			name:    "Verify that the share is parsed correctly with encryption key",
			status:  http.StatusOK,
			content: "{\"id\":\"share-id\",\"name\":\"share-name\",\"size\":10,\"iops\":3000,\"status\":\"pending\",\"zone\":{\"name\":\"test-1\",\"href\":\"https://us-south.iaas.cloud.ibm.com/v1/regions/us-south/zones/test-1\"},\"encryption_key\":{\"crn\":\"crn:v1:bluemix:public:kms:us-south:a/abcd32a619db2b564b82a816400bcd12:t36097fd-5051-4582-a641-8f51b5334cfa:key:abc05f428-5fb7-4546-958b-0f4e65266d5c\"},\"crn\":\"crn:v1:bluemix:public:is:test-1:a/rg1::share:vol1\"}",
			verify: func(t *testing.T, share *models.Share, err error) {
				if assert.NotNil(t, share) {
					assert.Equal(t, "share-id", share.ID)
				}
			},
		}, {
			name:    "False positive: What if the share ID is not matched",
			status:  http.StatusOK,
			content: "{\"id\":\"wrong-vol\",\"name\":\"wrong-vol\",\"size\":10,\"iops\":3000,\"status\":\"pending\",\"zone\":{\"name\":\"test-1\",\"href\":\"https://us-south.iaas.cloud.ibm.com/v1/regions/us-south/zones/test-1\"},\"crn\":\"crn:v1:bluemix:public:is:test-1:a/rg1::share:wrong-vol\"}",
			verify: func(t *testing.T, share *models.Share, err error) {
				if assert.NotNil(t, share) {
					assert.NotEqual(t, "vol1", share.ID)
				}
			},
		},
	}

	for _, testcase := range testCases {
		t.Run(testcase.name, func(t *testing.T) {
			template := &models.Share{
				Name: "share-name",
				Size: 10,
				ResourceGroup: &models.ResourceGroup{
					ID: "rg1",
				},
				Zone: &models.Zone{Name: "test-1"},
			}

			mux, client, teardown := test.SetupServer(t)
			test.SetupMuxResponse(t, mux, vpcfilevolume.Version+"/shares", http.MethodPost, nil, testcase.status, testcase.content, nil)

			defer teardown()

			logger.Info("Test case being executed", zap.Reflect("testcase", testcase.name))

			shareService := vpcfilevolume.New(client)

			share, err := shareService.CreateFileShare(template, logger)
			logger.Info("Volume details", zap.Reflect("share", share))

			if testcase.expectErr != "" && assert.Error(t, err) {
				assert.Equal(t, testcase.expectErr, err.Error())
				assert.Nil(t, share)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, share)
			}

			if testcase.verify != nil {
				testcase.verify(t, share, err)
			}
		})
	}
}

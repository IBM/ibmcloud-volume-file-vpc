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

func TestGetFileShareEtag(t *testing.T) {
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
			content: "{\"id\":\"vol1\",\"name\":\"vol1\",\"size\":10,\"iops\":3000,\"status\":\"pending\",\"zone\":{\"name\":\"test-1\",\"href\":\"https://us-south.iaas.cloud.ibm.com/v1/regions/us-south/zones/test-1\"},\"crn\":\"crn:v1:bluemix:public:is:test-1:a/rg1::share:vol1\"}",
			verify: func(t *testing.T, share *models.Share, err error) {
				if assert.NotNil(t, share) {
					assert.Equal(t, "vol1", share.ID)
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
			mux, client, teardown := test.SetupServer(t)
			emptyString := ""
			test.SetupMuxResponse(t, mux, vpcfilevolume.Version+"/shares/share-id", http.MethodGet, &emptyString, testcase.status, testcase.content, nil)

			defer teardown()

			logger.Info("Test case being executed", zap.Reflect("testcase", testcase.name))

			shareService := vpcfilevolume.New(client)

			share, _, err := shareService.GetFileShareEtag("share-id", logger)
			logger.Info("Share details", zap.Reflect("share", share))

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

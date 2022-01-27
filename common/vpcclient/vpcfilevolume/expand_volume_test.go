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

// Package vpcfilevolume ...
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

func TestExpandVolume(t *testing.T) {
	// Setup new style zap logger
	logger, _ := GetTestContextLogger()
	defer logger.Sync()

	testCases := []struct {
		name     string
		template *models.Share
		// Response
		status  int
		content string
		// Expected return
		expectErr string
		verify    func(*testing.T, *models.Share, error)
	}{
		{
			name: "Verify that the correct endpoint is invoked",
			template: &models.Share{
				Size: 300,
			},
			status: http.StatusNoContent,
		},
		{
			name: "Verify that the volume expanded correctly if correct size is given",
			template: &models.Share{
				Size: 300,
			},
			status:  http.StatusOK,
			content: "{\"id\":\"share-id\",\"name\":\"share-name\",\"size\":300,\"iops\":3000,\"lifecycle_state\":\"updating\",\"zone\":{\"name\":\"test-1\",\"href\":\"https://us-south.iaas.cloud.ibm.com/v1/regions/us-south/zones/test-1\"},\"crn\":\"crn:v1:bluemix:public:is:test-1:a/rg1::volume:vol1\"}",
			verify: func(t *testing.T, volume *models.Share, err error) {
				if assert.NotNil(t, volume) {
					assert.Equal(t, "share-id", volume.ID)
					assert.Equal(t, int64(300), volume.Size)
				}
			},
		},
		{
			name: "Verify that a 404 is returned to the caller",
			template: &models.Share{
				Size: 300,
			},
			status:    http.StatusNotFound,
			content:   "{\"errors\":[{\"message\":\"testerr\"}]}",
			expectErr: "Trace Code:, testerr Please check ",
		},
		{
			name: "False positive: What if the volume ID is not matched",
			template: &models.Share{
				Size: 300,
			},
			status:  http.StatusOK,
			content: "{\"id\":\"wrong-vol\",\"name\":\"wrong-vol\",\"size\":10,\"iops\":3000,\"lifecycle_state\":\"updating\",\"zone\":{\"name\":\"test-1\",\"href\":\"https://us-south.iaas.cloud.ibm.com/v1/regions/us-south/zones/test-1\"},\"crn\":\"crn:v1:bluemix:public:is:test-1:a/rg1::volume:wrong-vol\", \"tags\":[\"Wrong Tag\"]}",
		},
	}

	for _, testcase := range testCases {
		t.Run(testcase.name, func(t *testing.T) {
			mux, client, teardown := test.SetupServer(t)
			test.SetupMuxResponse(t, mux, vpcfilevolume.Version+"/shares/share-id", http.MethodPatch, nil, testcase.status, testcase.content, nil)
			logger.Info("tested SetupMuxResponse")
			defer teardown()

			logger.Info("Test case being executed", zap.Reflect("testcase", testcase.name))

			volumeService := vpcfilevolume.New(client)

			volume, err := volumeService.ExpandVolume("share-id", testcase.template, logger)
			logger.Info("Volume details", zap.Reflect("volume", volume))

			if testcase.expectErr != "" && assert.Error(t, err) {
				assert.Equal(t, testcase.expectErr, err.Error())
				assert.Nil(t, volume)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, volume)
			}

			if testcase.verify != nil {
				testcase.verify(t, volume, err)
			}
		})
	}
}

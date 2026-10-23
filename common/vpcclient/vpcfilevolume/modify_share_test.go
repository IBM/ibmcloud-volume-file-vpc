/**
 * Copyright 2026 IBM Corp.
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

func TestModifyShare(t *testing.T) {
	logger, _ := GetTestContextLogger()
	defer logger.Sync()

	testCases := []struct {
		name     string
		shareID  string
		template *models.Share
		// Response
		status  int
		content string
		// Expected
		expectErr string
		verify    func(*testing.T, *models.Share, error)
	}{
		{
			name:    "Successfully modify share IOPS (dp2)",
			shareID: "share-id",
			template: &models.Share{
				Iops: 5000,
			},
			status:  http.StatusOK,
			content: `{"id":"share-id","name":"share-name","size":100,"iops":5000,"lifecycle_state":"updating","zone":{"name":"us-south-1","href":"https://us-south.iaas.cloud.ibm.com/v1/regions/us-south/zones/us-south-1"},"crn":"crn:v1:bluemix:public:is:us-south-1:a/rg1::share:share-id"}`,
			verify: func(t *testing.T, share *models.Share, err error) {
				if assert.NotNil(t, share) {
					assert.Equal(t, "share-id", share.ID)
					assert.Equal(t, int64(5000), share.Iops)
				}
			},
		},
		{
			name:    "Successfully modify share bandwidth (rfs)",
			shareID: "share-id",
			template: &models.Share{
				Bandwidth: 5000,
			},
			status:  http.StatusOK,
			content: `{"id":"share-id","name":"share-name","size":100,"iops":3000,"bandwidth":5000,"lifecycle_state":"updating","zone":{"name":"us-south-1","href":"https://us-south.iaas.cloud.ibm.com/v1/regions/us-south/zones/us-south-1"},"crn":"crn:v1:bluemix:public:is:us-south-1:a/rg1::share:share-id"}`,
			verify: func(t *testing.T, share *models.Share, err error) {
				if assert.NotNil(t, share) {
					assert.Equal(t, "share-id", share.ID)
					assert.Equal(t, int32(5000), share.Bandwidth)
				}
			},
		},
		{
			name:    "Verify that a 404 returns an error to the caller",
			shareID: "share-id",
			template: &models.Share{
				Iops: 5000,
			},
			status:    http.StatusNotFound,
			content:   `{"errors":[{"message":"testerr","Code":"share_not_found"}],"trace":"2af63776-4df7-4970-b52d-4e25676ec0e4"}`,
			expectErr: "Trace Code:2af63776-4df7-4970-b52d-4e25676ec0e4, Code:share_not_found, Description:testerr, RC:404 Not Found",
		},
		{
			name:    "Verify that a 400 bad request returns an error",
			shareID: "share-id",
			template: &models.Share{
				Iops: -1,
			},
			status:    http.StatusBadRequest,
			content:   `{"errors":[{"message":"invalid iops value","Code":"invalid_field_value"}],"trace":"abc-trace-123"}`,
			expectErr: "Trace Code:abc-trace-123, Code:invalid_field_value, Description:invalid iops value, RC:400 Bad Request",
		},
		{
			name:    "Verify correct endpoint is invoked (204 No Content)",
			shareID: "share-id",
			template: &models.Share{
				Iops: 3000,
			},
			status: http.StatusNoContent,
		},
	}

	for _, testcase := range testCases {
		t.Run(testcase.name, func(t *testing.T) {
			mux, client, teardown := test.SetupServer(t)
			test.SetupMuxResponse(t, mux, vpcfilevolume.Version+"/shares/"+testcase.shareID, http.MethodPatch, nil, testcase.status, testcase.content, nil)
			defer teardown()

			logger.Info("Test case being executed", zap.Reflect("testcase", testcase.name))

			volumeService := vpcfilevolume.New(client)
			share, err := volumeService.ModifyShare(testcase.shareID, testcase.template, logger)
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

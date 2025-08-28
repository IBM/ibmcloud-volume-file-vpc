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

func TestGetShareProfile(t *testing.T) {
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
		verify    func(*testing.T, *models.Profile, error)
	}{
		{
			name:   "Verify that the correct endpoint is invoked",
			status: http.StatusNoContent,
		}, {
			name:      "Verify that a 404 is returned to the caller",
			status:    http.StatusNotFound,
			content:   "{\"errors\":[{\"message\":\"testerr\",\"Code\":\"profile_not_found\"}], \"trace\":\"2af63776-4df7-4970-b52d-4e25676ec0e4\"}",
			expectErr: "Trace Code:2af63776-4df7-4970-b52d-4e25676ec0e4, Code:profile_not_found, Description:testerr, RC:404 Not Found",
		}, {
			name:    "Verify that the profile exists",
			content: "{\"capacity\":{\"default\":10,\"max\":32000,\"min\":10,\"step\":1,\"type\":\"range\"},\"family\":\"defined_performance\",\"href\":\"https://us-south-stage01.iaasdev.cloud.ibm.com/v1/share/profiles/dp2\",\"name\":\"dp2\",\"resource_type\":\"share_profile\"}",
			status:  http.StatusOK,

			verify: func(t *testing.T, profile *models.Profile, err error) {
				if assert.NotNil(t, profile) {
					assert.Equal(t, "dp2", profile.Name)
				}
			},
		},
	}

	for _, testcase := range testCases {
		t.Run(testcase.name, func(t *testing.T) {
			mux, client, teardown := test.SetupServer(t)
			emptyString := ""
			test.SetupMuxResponse(t, mux, vpcfilevolume.Version+"/share/profiles/profile-name", http.MethodGet, &emptyString, testcase.status, testcase.content, nil)

			defer teardown()

			logger.Info("Test case being executed", zap.Reflect("testcase", testcase.name))

			shareService := vpcfilevolume.New(client)

			profile, err := shareService.GetShareProfile("profile-name", logger)

			if testcase.expectErr != "" && assert.Error(t, err) {
				assert.Equal(t, testcase.expectErr, err.Error())
				assert.Nil(t, profile)
			} else {
				assert.NoError(t, err)
			}

			if testcase.verify != nil {
				testcase.verify(t, profile, err)
			}

		})
	}
}

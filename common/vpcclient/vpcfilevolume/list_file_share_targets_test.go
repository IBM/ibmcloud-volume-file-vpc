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

// Package instances_test ...
package vpcfilevolume_test

import (
	"net/http"
	"testing"

	"github.com/IBM/ibmcloud-volume-file-vpc/common/vpcclient/models"
	"github.com/IBM/ibmcloud-volume-file-vpc/common/vpcclient/riaas/test"
	"github.com/IBM/ibmcloud-volume-file-vpc/common/vpcclient/vpcfilevolume"
	"github.com/IBM/ibmcloud-volume-interface/lib/provider"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

func TestListFileShareTargets(t *testing.T) {
	// Setup new style zap logger
	logger, _ := GetTestContextLogger()
	defer logger.Sync()

	shareID := "testShare"

	testCases := []struct {
		name string

		// Response
		status  int
		content string

		// Expected return
		expectErr string
		verify    func(*testing.T, *models.ShareTargetList, error)
	}{
		{
			name:   "Verify that the correct endpoint is invoked",
			status: http.StatusNoContent,
		}, {
			name:      "Verify that a 404 is returned to the caller",
			status:    http.StatusNotFound,
			content:   "{\"errors\":[{\"message\":\"testerr\"}]}",
			expectErr: "Trace Code:, testerr Please check ",
		}, {
			name:    "Verify that the share targets is done correctly",
			status:  http.StatusOK,
			content: "{\"mount_targets\":[{\"id\":\"sharetargetid1\", \"name\":\"share target\", \"vpc\": {\"id\":\"xvdc\"},\"status\":\"pending\"}]}",
			verify: func(t *testing.T, shareTargetList *models.ShareTargetList, err error) {
				assert.NotNil(t, shareTargetList)
				assert.Equal(t, len(shareTargetList.ShareTargets), 1)
			},
		},
	}

	for _, testcase := range testCases {
		t.Run(testcase.name, func(t *testing.T) {
			mux, client, teardown := test.SetupServer(t)
			test.SetupMuxResponse(t, mux, "/v1/shares/testShare/targets", http.MethodGet, nil, testcase.status, testcase.content, nil)

			defer teardown()

			template := &models.ShareTarget{
				ID:      "sharetargetid",
				Name:    "share target",
				VPC:     &provider.VPC{ID: "xvdc"},
				ShareID: shareID,
			}

			logger.Info("Test case being executed", zap.Reflect("testcase", testcase.name))

			shareService := vpcfilevolume.New(client)

			shareTargetsList, err := shareService.ListFileShareTargets(template.ShareID, nil, logger)

			if testcase.expectErr != "" && assert.Error(t, err) {
				assert.Equal(t, testcase.expectErr, err.Error())
				assert.Nil(t, shareTargetsList)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, shareTargetsList)
			}

			if testcase.verify != nil {
				testcase.verify(t, shareTargetsList, err)
			}
		})
	}
}

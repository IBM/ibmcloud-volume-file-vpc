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

func TestGetFileShareTarget(t *testing.T) {
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
		verify    func(*testing.T, *http.Response, error)
	}{
		{
			name:    "Verify that the get share target by target ID is done correctly",
			status:  http.StatusOK,
			content: "{\"id\":\"share target id\", \"name\":\"share target\", \"vpc\": {\"id\":\"xvdc\"},\"mount_path\":\"161.26.114.179:/nxg_s_volle91789b5_54d0_4cd3_87ab_776683355ec0/dd843c3b-7523-4940-aa5b-9c89119f6515\",\"status\":\"pending\"}}",
			verify: func(t *testing.T, httpResponse *http.Response, err error) {
				if assert.Nil(t, err) {
					assert.Nil(t, httpResponse)
				}
			},
		},
	}

	for _, testcase := range testCases {
		t.Run(testcase.name, func(t *testing.T) {
			template := &models.ShareTarget{
				ID:      "sharetargetid",
				Name:    "share target",
				VPC:     &provider.VPC{ID: "xvdc"},
				ShareID: shareID,
			}
			mux, client, teardown := test.SetupServer(t)
			test.SetupMuxResponse(t, mux, "/v1/shares/testShare/mount_targets/sharetargetid", http.MethodGet, nil, testcase.status, testcase.content, nil)

			defer teardown()

			logger.Info("Test case being executed", zap.Reflect("testcase", testcase.name))
			shareService := vpcfilevolume.New(client)

			shareTarget, err := shareService.GetFileShareTarget(template.ShareID, template.ID, logger)
			logger.Info("Share target details", zap.Reflect("shareTarget", shareTarget))

			if testcase.expectErr != "" && assert.Error(t, err) {
				assert.Equal(t, testcase.expectErr, err.Error())
				assert.NotNil(t, shareTarget)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, shareTarget)
			}
		})
	}
}

func TestGetFileShareTargetByName(t *testing.T) {
	// Setup new style zap logger
	logger, _ := GetTestContextLogger()
	defer logger.Sync()

	testCases := []struct {
		name string

		// Response
		status  int
		shares  string
		content string

		// Expected return
		expectErr string
		verify    func(*testing.T, *models.ShareTarget, error)
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
			name:    "Verify that the share name is parsed correctly",
			status:  http.StatusOK,
			content: "{\"mount_targets\":[{\"id\":\"voltarget1\", \"name\":\"vvoltarget1\", \"vpc\": {\"id\":\"xvdc\"},\"status\":\"pending\"}]}",
			verify: func(t *testing.T, shareTarget *models.ShareTarget, err error) {
				if assert.NotNil(t, shareTarget) {
					assert.Equal(t, "voltarget1", shareTarget.ID)
				}
			},
		}, {
			name:      "Verify that the share target is empty if the shares are empty",
			status:    http.StatusOK,
			expectErr: "Trace Code:, testerr Please check ",
		},
	}

	for _, testcase := range testCases {
		t.Run(testcase.name, func(t *testing.T) {
			mux, client, teardown := test.SetupServer(t)
			emptyString := ""
			test.SetupMuxResponse(t, mux, vpcfilevolume.Version+"/shares/shareID/mount_targets", http.MethodGet, &emptyString, testcase.status, testcase.content, nil)

			defer teardown()

			template := &models.ShareTarget{
				Name:    "voltarget1",
				VPC:     &provider.VPC{ID: "xvdc"},
				ShareID: "shareID",
			}

			logger.Info("Test case being executed", zap.Reflect("testcase", testcase.name))

			shareService := vpcfilevolume.New(client)
			share, err := shareService.GetFileShareTargetByName(template.Name, template.ShareID, logger)
			logger.Info("Share target details", zap.Reflect("share", share))

			if testcase.verify != nil {
				testcase.verify(t, share, err)
			}
		})
	}
}

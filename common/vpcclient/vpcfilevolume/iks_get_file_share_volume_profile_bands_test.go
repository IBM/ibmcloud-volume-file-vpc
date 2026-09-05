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

package vpcfilevolume_test

import (
	"net/http"
	"testing"

	"github.com/IBM/ibmcloud-volume-file-vpc/common/vpcclient/riaas/test"
	"github.com/IBM/ibmcloud-volume-file-vpc/common/vpcclient/vpcfilevolume"
	"github.com/IBM/ibmcloud-volume-interface/lib/provider"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

func TestGetShareProfileBands(t *testing.T) {
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
		verify    func(*testing.T, []provider.VolumeProfileBand, error)
	}{
		{
			name:   "Verify that the correct endpoint is invoked",
			status: http.StatusOK,
			content: `{"id":"dp2","config_validation":[` +
				`{"capacity":{"min":10,"max":39},"iops":{"min":100,"max":1000}}` +
				`]}`,
			verify: func(t *testing.T, bands []provider.VolumeProfileBand, err error) {
				assert.NoError(t, err)
				assert.Len(t, bands, 1)
			},
		}, {
			name:      "Incorrect endpoint returns 404",
			status:    http.StatusNotFound,
			content:   `{"incidentID":"abc","code":"P0404","description":"Not found","RC":404}`,
			expectErr: "Trace Code:abc, Code:P0404, Description:Not found, RC:404",
		},
	}

	for _, testcase := range testCases {
		t.Run(testcase.name, func(t *testing.T) {
			mux, client, teardown := test.SetupServer(t)
			test.SetupMuxResponse(t, mux, "/v2/storage/vpc/volumeProfile", http.MethodGet, nil, testcase.status, testcase.content, nil)

			defer teardown()

			logger.Info("Test case being executed", zap.Reflect("testcase", testcase.name))

			volumeService := vpcfilevolume.NewIKSVolumeService(client)

			bands, err := volumeService.GetShareProfileBands("dp2", logger)

			if testcase.expectErr != "" && assert.Error(t, err) {
				assert.Equal(t, testcase.expectErr, err.Error())
			} else if testcase.verify != nil {
				testcase.verify(t, bands, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

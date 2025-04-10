/**
 * Copyright 2024 IBM Corp.
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

func TestUpdateVolume(t *testing.T) {
	// Setup new style zap logger
	logger, _ := GetTestContextLogger()
	defer logger.Sync()

	pvcTemplate := provider.UpdatePVC{
		ID:         "volume-id",
		VolumeType: "vpc-share",
		Provider:   "g2_file",
		Cluster:    "cluster-id",
		CRN:        "crn:v1:staging:public:is:us-south-1:a/account-id::volume:volume-id",
		Tags:       []string{"tag1:val1", "tag2:val2"},
		Capacity:   2,
		Iops:       300,
	}

	testCases := []struct {
		name string

		// Response
		status           int
		updatePVCRequest provider.UpdatePVC
		content          string
		// Expected return
		expectErr string
		verify    func(*testing.T, *provider.UpdatePVC, error)
	}{
		{
			name:   "Verify that the correct endpoint is invoked",
			status: http.StatusNoContent,
		}, {
			name:             "Verify that the volume is updated successfully",
			status:           http.StatusOK,
			updatePVCRequest: pvcTemplate,
		}, {
			name:      "Incorrect endpoint is invoked",
			status:    http.StatusNotFound,
			content:   "{\"incidentID\":\"2af63776-4df7-4970-b52d-4e25676ec0e4\",\"code\":\"P0404\", \"description\":\"Not found\",\"RC\":404}",
			expectErr: "Trace Code:2af63776-4df7-4970-b52d-4e25676ec0e4, Code:P0404, Description:Not found, RC:404",
		},
	}

	for _, testcase := range testCases {
		t.Run(testcase.name, func(t *testing.T) {
			mux, client, teardown := test.SetupServer(t)
			test.SetupMuxResponse(t, mux, "/v2/storage/updateVolume", http.MethodPost, nil, testcase.status, testcase.content, nil)

			defer teardown()

			logger.Info("Test case being executed", zap.Reflect("testcase", testcase.name))

			volumeService := vpcfilevolume.NewIKSVolumeService(client)

			err := volumeService.UpdateVolume(&testcase.updatePVCRequest, logger)

			if testcase.expectErr != "" && assert.Error(t, err) {
				assert.Equal(t, testcase.expectErr, err.Error())
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

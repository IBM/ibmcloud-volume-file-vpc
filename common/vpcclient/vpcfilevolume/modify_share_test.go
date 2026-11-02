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
		name        string
		shareID     string
		template    *models.Share
		status      int
		content     string
		expectErr   string
		expectShare bool
		verify      func(*testing.T, *models.Share, error)
	}{
		{
			name:    "modify  IOPS for dp2 profile",
			shareID: "share-id",
			template: &models.Share{
				Iops: 5000,
			},
			status:      http.StatusOK,
			content:     `{"id":"share-id","name":"share-name","size":100,"iops":6000,"lifecycle_state":"updating","zone":{"name":"us-south-1","href":"https://us-south.iaas.cloud.ibm.com/v1/regions/us-south/zones/us-south-1"},"crn":"crn:v1:bluemix:public:is:us-south-1:a/rg1::share:share-id"}`,
			expectShare: true,
			verify: func(t *testing.T, share *models.Share, err error) {
				if assert.NoError(t, err) && assert.NotNil(t, share) {
					assert.Equal(t, "share-id", share.ID)
					assert.Equal(t, int64(6000), share.Iops)
				}
			},
		},
		{
			name:    "modify  bandwidth for rfs profile",
			shareID: "share-id",
			template: &models.Share{
				Bandwidth: 5000,
			},
			status:      http.StatusOK,
			content:     `{"id":"share-id","name":"share-name","size":100,"iops":3000,"bandwidth":6000,"lifecycle_state":"updating","zone":{"name":"us-south-1","href":"https://us-south.iaas.cloud.ibm.com/v1/regions/us-south/zones/us-south-1"},"crn":"crn:v1:bluemix:public:is:us-south-1:a/rg1::share:share-id"}`,
			expectShare: true,
			verify: func(t *testing.T, share *models.Share, err error) {
				if assert.NoError(t, err) && assert.NotNil(t, share) {
					assert.Equal(t, "share-id", share.ID)
					assert.Equal(t, int32(6000), share.Bandwidth)
				}
			},
		},
	}

	for _, testcase := range testCases {
		t.Run(testcase.name, func(t *testing.T) {
			mux, client, teardown := test.SetupServer(t)
			defer teardown()

			test.SetupMuxResponse(
				t,
				mux,
				vpcfilevolume.Version+"/shares/"+testcase.shareID,
				http.MethodPatch,
				nil,
				testcase.status,
				testcase.content,
				nil,
			)

			logger.Info("Test case being executed", zap.Reflect("testcase", testcase.name))

			volumeService := vpcfilevolume.New(client)
			share, err := volumeService.ModifyShare(
				testcase.shareID,
				testcase.template,
				logger,
			)

			logger.Info("Share details", zap.Reflect("share", share))

			if testcase.expectErr != "" {
				if assert.Error(t, err) {
					assert.EqualError(t, err, testcase.expectErr)
				}
				assert.Nil(t, share)
				return
			}

			assert.NoError(t, err)

			if testcase.expectShare {
				assert.NotNil(t, share)
			} else {
				assert.Nil(t, share)
			}

			if testcase.verify != nil {
				testcase.verify(t, share, err)
			}
		})
	}
}

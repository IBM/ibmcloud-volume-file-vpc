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

// Package provider ...
package provider

import (
	"errors"
	"testing"
	"time"

	"github.com/IBM/ibmcloud-volume-file-vpc/common/vpcclient/models"
	serviceFakes "github.com/IBM/ibmcloud-volume-file-vpc/common/vpcclient/vpcfilevolume/fakes"
	"github.com/IBM/ibmcloud-volume-interface/lib/provider"
	util "github.com/IBM/ibmcloud-volume-interface/lib/utils"
	"github.com/IBM/ibmcloud-volume-interface/lib/utils/reasoncode"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

func TestGetSnapshot(t *testing.T) {
	//var err error
	logger, teardown := GetTestLogger(t)
	defer teardown()

	var (
		snapshotService *serviceFakes.SnapshotManager
	)
	timeNow := time.Now()

	testCases := []struct {
		testCaseName string

		snapshotID   string
		baseSnapshot *models.Snapshot
		setup        func()

		skipErrTest        bool
		expectedErr        string
		expectedReasonCode string

		verify func(t *testing.T, snapshotResponse *provider.Snapshot, err error)
	}{
		{
			testCaseName: "OK",
			snapshotID:   "16f293bf-test-4bff-816f-e199c0c65db5",
			baseSnapshot: &models.Snapshot{
				ID:             "16f293bf-test-4bff-816f-e199c0c65db5",
				Name:           "test-snapshot-name",
				LifecycleState: snapshotReadyState,
				CreatedAt:      &timeNow,
			},
			verify: func(t *testing.T, snapshotResponse *provider.Snapshot, err error) {
				assert.NotNil(t, snapshotResponse)
				assert.Nil(t, err)
			},
		}, {
			testCaseName: "Wrong snapshot ID",
			snapshotID:   "Wrong snapshot ID",
			baseSnapshot: &models.Snapshot{
				ID:             "wrong-wrong-id",
				Name:           "test-snapshot-name",
				LifecycleState: snapshotReadyState,
				CreatedAt:      &timeNow,
			},
			expectedErr:        "{Code:ErrorUnclassified, Type:InvalidRequest, Description:'Wrong snapshot ID' snapshot ID is not valid., RC:400}",
			expectedReasonCode: "ErrorUnclassified",
			verify: func(t *testing.T, snapshotResponse *provider.Snapshot, err error) {
				assert.Nil(t, snapshotResponse)
				assert.NotNil(t, err)
			},
		},
	}

	for _, testcase := range testCases {
		t.Run(testcase.testCaseName, func(t *testing.T) {
			vpcs, uc, sc, err := GetTestOpenSession(t, logger)
			assert.NotNil(t, vpcs)
			assert.NotNil(t, uc)
			assert.NotNil(t, sc)
			assert.Nil(t, err)

			snapshotService = &serviceFakes.SnapshotManager{}
			assert.NotNil(t, snapshotService)
			uc.SnapshotServiceReturns(snapshotService)

			if testcase.expectedErr != "" {
				snapshotService.GetSnapshotReturns(testcase.baseSnapshot, errors.New(testcase.expectedReasonCode))
			} else {
				snapshotService.GetSnapshotReturns(testcase.baseSnapshot, nil)
			}
			snapshot, err := vpcs.GetSnapshot(testcase.snapshotID, "16f293bf-test-4bff-816f-e199c0c65db5")
			logger.Info("Snapshot details", zap.Reflect("snapshot", snapshot))

			if testcase.expectedErr != "" {
				assert.NotNil(t, err)
				logger.Info("Error details", zap.Reflect("Error details", err.Error()))
				assert.Equal(t, reasoncode.ReasonCode(testcase.expectedReasonCode), util.ErrorReasonCode(err))
			}

			if testcase.verify != nil {
				testcase.verify(t, snapshot, err)
			}
		})
	}
}

func TestGetSnapshotByName(t *testing.T) {
	//var err error
	logger, teardown := GetTestLogger(t)
	defer teardown()

	var (
		snapshotService *serviceFakes.SnapshotManager
	)
	timeNow := time.Now()

	testCases := []struct {
		testCaseName string
		snapshotName string
		baseSnapshot *models.Snapshot

		setup func()

		skipErrTest        bool
		expectedErr        string
		expectedReasonCode string

		verify func(t *testing.T, snapshotResponse *provider.Snapshot, err error)
	}{
		{
			testCaseName: "OK",
			snapshotName: "Test snapshot",
			baseSnapshot: &models.Snapshot{
				ID:             "wrong-wrong-id",
				Name:           "test-snapshot-name",
				LifecycleState: snapshotReadyState,
				CreatedAt:      &timeNow,
			},
			verify: func(t *testing.T, snapshotResponse *provider.Snapshot, err error) {
				assert.NotNil(t, snapshotResponse)
				assert.Nil(t, err)
			},
		}, {
			testCaseName: "Wrong snapshot ID",
			snapshotName: "Wrong snapshot name",
			baseSnapshot: &models.Snapshot{
				ID:             "wrong-wrong-id",
				Name:           "test-snapshot-name",
				LifecycleState: snapshotReadyState,
				CreatedAt:      &timeNow,
			},
			expectedErr:        "{Code:ErrorUnclassified, Type:InvalidRequest, Description:'Wrong snapshot ID' snapshot ID is not valid., RC:400}",
			expectedReasonCode: "ErrorUnclassified",
			verify: func(t *testing.T, snapshotResponse *provider.Snapshot, err error) {
				assert.Nil(t, snapshotResponse)
				assert.NotNil(t, err)
			},
		}, {
			testCaseName:       "Empty volume name",
			snapshotName:       "",
			expectedErr:        "{Code:ErrorUnclassified, Type:RetrivalFailed, Description:Failed to find '16f293bf-test-4bff-816f-e199c0c65db5' snapshot ID., BackendError:SnapshotIDNotFound, RC:404}",
			expectedReasonCode: "ErrorUnclassified",
			verify: func(t *testing.T, snapshotResponse *provider.Snapshot, err error) {
				assert.Nil(t, snapshotResponse)
				assert.NotNil(t, err)
			},
		},
	}

	for _, testcase := range testCases {
		t.Run(testcase.testCaseName, func(t *testing.T) {
			vpcs, uc, sc, err := GetTestOpenSession(t, logger)
			assert.NotNil(t, vpcs)
			assert.NotNil(t, uc)
			assert.NotNil(t, sc)
			assert.Nil(t, err)

			snapshotService = &serviceFakes.SnapshotManager{}
			assert.NotNil(t, snapshotService)
			uc.SnapshotServiceReturns(snapshotService)

			if testcase.expectedErr != "" {
				snapshotService.GetSnapshotByNameReturns(testcase.baseSnapshot, errors.New(testcase.expectedReasonCode))
			} else {
				snapshotService.GetSnapshotByNameReturns(testcase.baseSnapshot, nil)
			}
			snapshot, err := vpcs.GetSnapshotByName(testcase.snapshotName, "16f293bf-test-4bff-816f-e199c0c65db5")
			logger.Info("Snapshot details", zap.Reflect("volume", snapshot))

			if testcase.expectedErr != "" {
				assert.NotNil(t, err)
				logger.Info("Error details", zap.Reflect("Error details", err.Error()))
				assert.Equal(t, reasoncode.ReasonCode(testcase.expectedReasonCode), util.ErrorReasonCode(err))
			}

			if testcase.verify != nil {
				testcase.verify(t, snapshot, err)
			}
		})
	}
}

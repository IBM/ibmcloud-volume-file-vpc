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

// Package vpcfilevolume ...
package vpcfilevolume

import (
	"time"

	"github.com/IBM/ibmcloud-volume-file-vpc/common/vpcclient/client"
	util "github.com/IBM/ibmcloud-volume-interface/lib/utils"
	"go.uber.org/zap"
)

// VolumeProfileBand represents one capacity-to-IOPS band returned by the
// armada-storage-api GET /v2/storage/vpc/getVolumeProfiles endpoint.
type VolumeProfileBand struct {
	CapacityMin int64 `json:"capacityMin"`
	CapacityMax int64 `json:"capacityMax"`
	IOPSMin     int64 `json:"iopsMin"`
	IOPSMax     int64 `json:"iopsMax"`
}

// volumeProfileBandsResponse is the JSON envelope returned by armada-storage-api.
type volumeProfileBandsResponse struct {
	Profile string              `json:"profile"`
	Bands   []VolumeProfileBand `json:"bands"`
}

// GetVolumeProfileBands GETs /v2/storage/vpc/getVolumeProfiles?profile=<name> via
// armada-storage-api. The IKS session client already carries the IAM bearer token
// set during Login(), so no additional token management is needed here.
func (vs *IKSVolumeService) GetVolumeProfileBands(profile string, ctxLogger *zap.Logger) ([]VolumeProfileBand, error) {
	ctxLogger.Debug("Entry Backend IKSVolumeService.GetVolumeProfileBands")
	defer ctxLogger.Debug("Exit Backend IKSVolumeService.GetVolumeProfileBands")

	defer util.TimeTracker("IKSVolumeService.GetVolumeProfileBands", time.Now())

	operation := &client.Operation{
		Name:        "GetVolumeProfileBands",
		Method:      "GET",
		PathPattern: vs.pathPrefix + vpcGetVolumeProfiles,
	}

	var result volumeProfileBandsResponse
	apiErr := vs.receiverError

	request := vs.client.NewRequest(operation)
	// profile is passed as a query parameter, not a path segment
	request.SetQueryValue("profile", profile)
	ctxLogger.Info("Equivalent curl command", zap.Reflect("URL", request.URL()), zap.Reflect("Operation", operation))

	_, err := request.JSONSuccess(&result).JSONError(apiErr).Invoke()
	if err != nil {
		ctxLogger.Error("GetVolumeProfileBands failed", zap.String("profile", profile), zap.Error(err))
		return nil, err
	}

	ctxLogger.Info("GetVolumeProfileBands succeeded",
		zap.String("profile", profile),
		zap.Int("bands", len(result.Bands)))
	return result.Bands, nil
}

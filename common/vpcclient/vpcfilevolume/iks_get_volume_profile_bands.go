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
	"encoding/json"
	"time"

	"github.com/IBM/ibmcloud-volume-file-vpc/common/catalog"
	"github.com/IBM/ibmcloud-volume-file-vpc/common/vpcclient/client"
	util "github.com/IBM/ibmcloud-volume-interface/lib/utils"
	"go.uber.org/zap"
)

// VolumeProfileBand is a type alias for catalog.VolumeProfileBand.
// Using a type alias (not a redefinition) ensures that every package in the
// call chain — IKSVolumeService, IksVpcSession, file/provider, and the CSI
// driver's VolumeProfileBandsFetcher interface — all refer to the exact same
// Go type, so the interface type assertion succeeds at runtime.
type VolumeProfileBand = catalog.VolumeProfileBand

// GetVolumeProfileBands GETs /v2/storage/vpc/volumeProfile?profile=<name> via
// armada-storage-api and converts the response to []VolumeProfileBand.
// JSON parsing is delegated to catalog.ParseVolumeProfileBands to avoid
// duplicating the wire-format struct definitions.
// The IKS session client already carries the IAM bearer token set during
// Login(), so no additional token management is needed here.
func (vs *IKSVolumeService) GetVolumeProfileBands(profile string, ctxLogger *zap.Logger) ([]VolumeProfileBand, error) {
	ctxLogger.Debug("Entry Backend IKSVolumeService.GetVolumeProfileBands")
	defer ctxLogger.Debug("Exit Backend IKSVolumeService.GetVolumeProfileBands")

	defer util.TimeTracker("IKSVolumeService.GetVolumeProfileBands", time.Now())

	operation := &client.Operation{
		Name:        "GetVolumeProfileBands",
		Method:      "GET",
		PathPattern: vs.pathPrefix + vpcVolumeProfile,
	}

	apiErr := vs.receiverError

	request := vs.client.NewRequest(operation)
	// profile is passed as a query parameter, not a path segment
	request.SetQueryValue("profile", profile)
	ctxLogger.Info("Equivalent curl command", zap.Reflect("URL", request.URL()), zap.Reflect("Operation", operation))

	// Capture the raw JSON so that catalog.ParseVolumeProfileBands can decode
	// it — this avoids duplicating the wire-format struct definitions here.
	var raw json.RawMessage
	_, err := request.JSONSuccess(&raw).JSONError(apiErr).Invoke()
	if err != nil {
		ctxLogger.Error("GetVolumeProfileBands failed", zap.String("profile", profile), zap.Error(err))
		return nil, err
	}

	bands, err := catalog.ParseVolumeProfileBands([]byte(raw), profile)
	if err != nil {
		ctxLogger.Error("GetVolumeProfileBands: parse failed", zap.String("profile", profile), zap.Error(err))
		return nil, err
	}

	ctxLogger.Info("GetVolumeProfileBands succeeded",
		zap.String("profile", profile),
		zap.Int("bands", len(bands)))
	return bands, nil
}

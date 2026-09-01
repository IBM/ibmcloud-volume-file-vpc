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
	"fmt"
	"time"

	"github.com/IBM/ibmcloud-volume-file-vpc/common/vpcclient/client"
	"github.com/IBM/ibmcloud-volume-interface/lib/provider"
	util "github.com/IBM/ibmcloud-volume-interface/lib/utils"
	"go.uber.org/zap"
)

// VolumeProfileBand is a type alias for provider.VolumeProfileBand.
// The canonical struct is defined in ibmcloud-volume-interface so it is
// shared across all session implementations without duplication.
type VolumeProfileBand = provider.VolumeProfileBand

// volumeProfileResponse mirrors the JSON envelope returned by armada-storage-api
// GET /v2/storage/vpc/volumeProfile?profile=<name>.
type volumeProfileResponse struct {
	ID               string                  `json:"id"`
	ConfigValidation []configValidationEntry `json:"config_validation"`
}

// configValidationEntry is one entry in the config_validation array.
type configValidationEntry struct {
	Capacity capacityRange `json:"capacity"`
	IOPS     *metricRange  `json:"iops,omitempty"`
}

type capacityRange struct {
	Min int64 `json:"min"`
	Max int64 `json:"max"`
}

type metricRange struct {
	Min int64 `json:"min"`
	Max int64 `json:"max"`
}

// ParseVolumeProfileBands decodes a raw armada-storage-api
// GET /v2/storage/vpc/volumeProfile response body and returns the
// capacity/IOPS bands as []VolumeProfileBand.
//
// Entries without an "iops" field (e.g. throughput-only bands) are skipped.
// Returns an error if the body cannot be decoded or no IOPS bands are found.
func ParseVolumeProfileBands(body []byte, profile string) ([]VolumeProfileBand, error) {
	var parsed volumeProfileResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("volume profile: decode response: %w", err)
	}

	if len(parsed.ConfigValidation) == 0 {
		return nil, fmt.Errorf("GetVolumeProfileBands: no bands returned for profile %q", profile)
	}

	bands := make([]VolumeProfileBand, 0, len(parsed.ConfigValidation))
	for _, entry := range parsed.ConfigValidation {
		if entry.IOPS == nil {
			continue
		}
		bands = append(bands, VolumeProfileBand{
			CapacityMin: entry.Capacity.Min,
			CapacityMax: entry.Capacity.Max,
			IOPSMin:     entry.IOPS.Min,
			IOPSMax:     entry.IOPS.Max,
		})
	}

	if len(bands) == 0 {
		return nil, fmt.Errorf("GetVolumeProfileBands: no iops bands found for profile %q", profile)
	}
	return bands, nil
}

// GetVolumeProfileBands GETs /v2/storage/vpc/volumeProfile?profile=<name> via
// armada-storage-api and converts the response to []VolumeProfileBand.
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
	// This endpoint is an armada-storage-api proxy, not a VPC RIAAS endpoint.
	// The IKS session client injects "generation" and "version" into every
	// request by default; strip them here so armada-storage-api does not reject
	// the call with ST0020 ("unsupported query parameter").
	request.DeleteQueryValue("generation")
	request.DeleteQueryValue("version")
	// profile is passed as a query parameter, not a path segment
	request.SetQueryValue("profile", profile)
	ctxLogger.Info("Equivalent curl command", zap.Reflect("URL", request.URL()), zap.Reflect("Operation", operation))

	// Capture the raw JSON so that ParseVolumeProfileBands can decode it.
	var raw json.RawMessage
	_, err := request.JSONSuccess(&raw).JSONError(apiErr).Invoke()
	if err != nil {
		ctxLogger.Error("GetVolumeProfileBands failed", zap.String("profile", profile), zap.Error(err))
		return nil, err
	}

	bands, err := ParseVolumeProfileBands([]byte(raw), profile)
	if err != nil {
		ctxLogger.Error("GetVolumeProfileBands: parse failed", zap.String("profile", profile), zap.Error(err))
		return nil, err
	}

	ctxLogger.Info("GetVolumeProfileBands succeeded",
		zap.String("profile", profile),
		zap.Int("bands", len(bands)))
	return bands, nil
}

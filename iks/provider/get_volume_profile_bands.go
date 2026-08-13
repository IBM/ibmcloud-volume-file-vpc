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
	"time"

	userError "github.com/IBM/ibmcloud-volume-file-vpc/common/messages"
	vpc_provider "github.com/IBM/ibmcloud-volume-file-vpc/file/provider"
	"github.com/IBM/ibmcloud-volume-interface/lib/metrics"
	"go.uber.org/zap"
)

// GetVolumeProfileBands retrieves the capacity-to-IOPS bands for the named
// VPC file volume profile (e.g. "dp2") by calling armada-storage-api through
// the IKS session. The IKS session client already carries the IAM bearer token
// set during OpenSession → Login(), so no additional token handling is needed.
//
// This follows the same pattern as UpdateVolume: the IKS-specific session
// method delegates to IksSession.Apiclient.FileShareService(), which sends the
// request to armada-storage-api at /v2/storage/vpc/getVolumeProfiles?profile=<name>
// with the Authorization header injected automatically by authenticationHandler.
func (vpcIks *IksVpcSession) GetVolumeProfileBands(profile string) ([]vpc_provider.VolumeProfileBand, error) {
	vpcIks.Logger.Debug("Entry of GetVolumeProfileBands method...", zap.String("profile", profile))
	defer vpcIks.Logger.Debug("Exit from GetVolumeProfileBands method...", zap.String("profile", profile))

	defer metrics.UpdateDurationFromStart(vpcIks.Logger, "GetVolumeProfileBands", time.Now())

	var bands []vpc_provider.VolumeProfileBand
	err := vpcIks.APIRetry.FlexyRetry(vpcIks.Logger, func() (error, bool) {
		var callErr error
		bands, callErr = vpcIks.IksSession.Apiclient.FileShareService().GetVolumeProfileBands(profile, vpcIks.Logger)
		return callErr, callErr == nil || vpc_provider.SkipRetryForIKS(callErr)
	})

	if err != nil {
		vpcIks.Logger.Error("Failed to fetch volume profile bands",
			zap.String("profile", profile),
			zap.Error(err))
		return nil, userError.GetUserError("StorageFindFailed", err)
	}

	vpcIks.Logger.Info("Successfully fetched volume profile bands",
		zap.String("profile", profile),
		zap.Int("count", len(bands)))
	return bands, nil
}

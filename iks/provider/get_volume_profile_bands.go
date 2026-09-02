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
	"github.com/IBM/ibmcloud-volume-interface/lib/metrics"
	"github.com/IBM/ibmcloud-volume-interface/lib/provider"
	"go.uber.org/zap"
)

// GetVolumeProfileBands retrieves the capacity-to-IOPS bands for the named
// VPC file volume profile (e.g. "dp2") by calling armada-storage-api through
// the IKS session. This overrides the VPCSession stub promoted via embedding,
// providing the real implementation for IKS clusters.
// Satisfies the provider.VolumeManager interface.
//
// The IKS session client already carries the IAM bearer token set during
// OpenSession → Login(), so no additional token handling is needed.
//
// No retry is performed: this call is used once at driver startup to warm the
// capacity-round-off cache. A transient failure is non-fatal — the driver logs
// a warning and continues without the cache; any StorageClass that sets
// allowCapacityRoundoffForIops=true will return a clear error at PVC creation
// time. Retrying here would block SetupIBMCSIDriver() for minutes and cause
// the liveness probe to kill the container before startup completes.
func (vpcIks *IksVpcSession) GetVolumeProfileBands(profile string) ([]provider.VolumeProfileBand, error) {
	vpcIks.Logger.Debug("Entry of GetVolumeProfileBands method...", zap.String("profile", profile))
	defer vpcIks.Logger.Debug("Exit from GetVolumeProfileBands method...", zap.String("profile", profile))

	defer metrics.UpdateDurationFromStart(vpcIks.Logger, "GetVolumeProfileBands", time.Now())

	bands, err := vpcIks.IksSession.Apiclient.FileShareService().GetShareProfileBands(profile, vpcIks.Logger)
	if err != nil {
		vpcIks.Logger.Error("Failed to fetch share profile bands",
			zap.String("profile", profile),
			zap.Error(err))
		return nil, userError.GetUserError("StorageFindFailed", err)
	}

	vpcIks.Logger.Info("Successfully fetched share profile bands",
		zap.String("profile", profile),
		zap.Int("count", len(bands)))
	return bands, nil
}

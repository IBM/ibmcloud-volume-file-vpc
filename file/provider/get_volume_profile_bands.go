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
	"fmt"

	"github.com/IBM/ibmcloud-volume-file-vpc/common/vpcclient/vpcfilevolume"
	"go.uber.org/zap"
)

// VolumeProfileBand re-exports the band type so callers of this package
// do not need to import common/vpcclient/vpcfilevolume directly.
type VolumeProfileBand = vpcfilevolume.VolumeProfileBand

// GetVolumeProfileBands retrieves the capacity-to-IOPS bands for the named
// VPC file volume profile (e.g. "dp2") via the session's API client.
//
// On a VPC (RIAAS) session this always returns an error because the profile
// band data is only available via the armada-storage-api IKS proxy.
// On an IKS session the call is routed through IksVpcSession.GetVolumeProfileBands.
func (vpcs *VPCSession) GetVolumeProfileBands(profile string) ([]VolumeProfileBand, error) {
	vpcs.Logger.Debug("Entry of GetVolumeProfileBands method...", zap.String("profile", profile))
	defer vpcs.Logger.Debug("Exit from GetVolumeProfileBands method...", zap.String("profile", profile))

	return nil, fmt.Errorf("GetVolumeProfileBands is only available via the IKS session (armada-storage-api proxy); profile=%q", profile)
}

/**
 * Copyright 2022 IBM Corp.
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
	"strconv"
	"time"

	userError "github.com/IBM/ibmcloud-volume-file-vpc/common/messages"
	"github.com/IBM/ibmcloud-volume-file-vpc/common/vpcclient/models"
	"github.com/IBM/ibmcloud-volume-interface/lib/metrics"
	"github.com/IBM/ibmcloud-volume-interface/lib/provider"
	"go.uber.org/zap"
)

func (vpcs *VPCSession) ModifyVolume(modifyVolumeRequest provider.ModifyVolumeRequest) (Iops int64, Bandwidth int32, err error) {
	vpcs.Logger.Debug("Entry of ModifyVolume method...")
	defer vpcs.Logger.Debug("Exit from ModifyVolume method...")
	defer metrics.UpdateDurationFromStart(vpcs.Logger, "ModifyVolume", time.Now())

	// Get volume details
	existingVolume, err := vpcs.GetVolume(modifyVolumeRequest.VolumeID)
	if err != nil {
		return -1, -1, err
	}

	isIopsUpdate := modifyVolumeRequest.Iops > 0
	isBandwidthUpdate := modifyVolumeRequest.Bandwidth > 0

	if !isIopsUpdate && !isBandwidthUpdate {
		vpcs.Logger.Warn("No updates requested")

		var currIops int64
		if existingVolume.Iops != nil && *existingVolume.Iops != "" {
			currIops, _ = strconv.ParseInt(*existingVolume.Iops, 10, 64)
		}

		currBandwidth := existingVolume.Bandwidth

		return currIops, currBandwidth, nil
	}

	vpcs.Logger.Info("Successfully validated inputs for ModifyVolume request... ")

	var newIops int64
	var newBandwidth int32

	if isIopsUpdate {
		newIops = modifyVolumeRequest.Iops
	}
	if isBandwidthUpdate {
		newBandwidth = modifyVolumeRequest.Bandwidth
	}

	shareTemplate := &models.Share{}
	if isIopsUpdate {
		shareTemplate.Iops = newIops
	}
	if isBandwidthUpdate {
		shareTemplate.Bandwidth = newBandwidth
	}

	vpcs.Logger.Info("Calling VPC provider for volume Modify...")
	var share *models.Share
	err = retry(vpcs.Logger, func() error {
		share, err = vpcs.Apiclient.FileShareService().ExpandVolume(
			modifyVolumeRequest.VolumeID,
			shareTemplate,
			vpcs.Logger,
		)
		return err
	})

	if err != nil {
		vpcs.Logger.Debug("Failed to modify volume from VPC provider", zap.Reflect("BackendError", err))
		return -1, -1, userError.GetUserError("FailedToModifyVolume", err, modifyVolumeRequest.VolumeID)
	}

	vpcs.Logger.Info("Successfully accepted volume modify request, now waiting for volume state equal to stable")
	err = WaitForValidVolumeState(vpcs, share.ID)
	if err != nil {
		return -1, -1, userError.GetUserError("VolumeNotInValidState", err, share.ID)
	}

	vpcs.Logger.Info("Volume got valid (stable) state", zap.Reflect("VolumeDetails", share))

	updatedIops := share.Iops
	updatedBandwidth := share.Bandwidth

	return updatedIops, updatedBandwidth, nil
}

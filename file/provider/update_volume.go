/**
 * Copyright 2021 IBM Corp.
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
	"strings"
	"time"

	userError "github.com/IBM/ibmcloud-volume-file-vpc/common/messages"
	"github.com/IBM/ibmcloud-volume-file-vpc/common/vpcclient/models"
	"github.com/IBM/ibmcloud-volume-interface/lib/provider"
	"go.uber.org/zap"
)

const (
	StatusFailed             = "failed"
	StatusProvisioningFailed = "provisioning_failed"
)

func convertMapToTagList(tagMap map[string]string) []string {
	tags := []string{}
	for k, v := range tagMap {
		tags = append(tags, k+":"+v)
	}
	return tags
}

// UpdateVolume PATCH to /volumes
func (vpcs *VPCSession) UpdateVolume(volumeTemplate provider.Volume) error {
	var existShare *models.Share
	var err error
	var etag string
	updatePayload := &models.Share{}
	shouldUpdate := false

	requestedTags := convertMapToTagList(volumeTemplate.Tags)

	//Fetch existing volume Tags
	err = retryWithMinRetries(vpcs.Logger, func() error {
		// Get volume details
		existShare, etag, err = vpcs.Apiclient.FileShareService().GetFileShareEtag(volumeTemplate.VolumeID, vpcs.Logger)

		if err != nil {
			return err
		}
		if existShare == nil || existShare.Status != StatusStable {
			return userError.GetUserError("VolumeNotInValidState", err, volumeTemplate.VolumeID)
		}

		vpcs.Logger.Info("Volume got valid (stable) state", zap.Reflect("etag", etag))

		// Tag check using new map-based tags
		if !ifTagsEqual(existShare.UserTags, requestedTags) {
			updatePayload.UserTags = append(existShare.UserTags, requestedTags...)
			shouldUpdate = true
		}

		// Profile check for bandwidth / iops
		profile := strings.ToLower(existShare.Profile.Name)

		// Bandwidth support for rfs
		if profile == "rfs" && volumeTemplate.Bandwidth != nil {
			newBandwidth := ToInt64(*volumeTemplate.Bandwidth)
			if existShare.Bandwidth == nil || *existShare.Bandwidth != newBandwidth {
				updatePayload.Bandwidth = &newBandwidth
				shouldUpdate = true
			}
		}

		// IOPS support for dp2
		if profile == "dp2" && volumeTemplate.Iops != nil {
			newIops := ToInt64(*volumeTemplate.Iops)
			if existShare.Iops != newIops {
				updatePayload.Iops = newIops
				shouldUpdate = true
			}
		}

		// If no change detected, skip API call
		if !shouldUpdate {
			vpcs.Logger.Info("No changes detected, skipping update call")
			return nil
		}

		if !shouldUpdate {
			vpcs.Logger.Info("No changes detected, skipping update call")
			return nil
		}

		vpcs.Logger.Info("Calling VPC provider for volume UpdateVolumeWithTags...",
			zap.Reflect("VolumeID", volumeTemplate.VolumeID),
			zap.Reflect("Payload", updatePayload),
		)

		err = vpcs.Apiclient.FileShareService().UpdateFileShareWithEtag(volumeTemplate.VolumeID, etag, updatePayload, vpcs.Logger)
		return err
	})

	if err != nil {
		vpcs.Logger.Error("Failed to update volume tags from VPC provider", zap.Reflect("BackendError", err))
		return userError.GetUserError("FailedToUpdateVolume", err, volumeTemplate.VolumeID)
	}
	// Wait until volume returns to 'stable'
	vpcs.Logger.Info("Waiting for volume to reach stable state...")
	err = waitForStableState(vpcs, volumeTemplate.VolumeID, vpcs.Logger, 20, 10*time.Second)
	if err != nil {
		vpcs.Logger.Error("Volume did not reach stable state", zap.Error(err))
		return err
	}
	return err
}

// ifTagsEqual will check if there is change to existing tags
func ifTagsEqual(existingTags []string, newTags []string) bool {
	//Join slice into a string
	tags := strings.ToLower(strings.Join(existingTags, ","))
	for _, v := range newTags {
		if !strings.Contains(tags, strings.ToLower(v)) {
			//Tags are different
			return false
		}
	}
	//Tags are equal
	return true
}

// waitForStableState polls the volume (file share) status until it reaches a 'stable' state.
// It retries up to `maxRetries` with a sleep interval of `interval` between attempts.
// Returns an error if the volume enters a failed state or does not become stable within the retry limit.
func waitForStableState(vpcs *VPCSession, shareID string, ctxLogger *zap.Logger, maxRetries int, interval time.Duration) error {
	for i := 0; i < maxRetries; i++ {
		share, _, err := vpcs.Apiclient.FileShareService().GetFileShareEtag(shareID, ctxLogger)
		if err != nil {
			ctxLogger.Warn("Failed to get volume state during wait", zap.Error(err))
			return err
		}

		state := strings.ToLower(string(share.Status))
		ctxLogger.Info("Polling share state", zap.String("state", state), zap.Int("attempt", i+1))

		if state == StatusStable {
			return nil
		}
		if state == "failed" || state == "provisioning_failed" {
			return userError.GetUserError("VolumeNotInValidState", nil, shareID)
		}

		if state == StatusStable {
			return nil
		}
		if state == StatusFailed || state == StatusProvisioningFailed {
			return userError.GetUserError("VolumeNotInValidState", nil, shareID)
		}

		time.Sleep(interval)
	}

	return userError.GetUserError("VolumeNotInValidState", nil, shareID)
}

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
	"go.uber.org/zap"
)

// GetVolumeProfileByName ...
func (vpcs *VPCSession) GetVolumeProfileByName(name string) error {
	vpcs.Logger.Debug("Entry of GetVolumeProfileByName method...")
	defer vpcs.Logger.Debug("Exit from GetVolumeProfileByName method...")

	vpcs.Logger.Info("Checking if Volume Profile is valid...", zap.Reflect("VolumeProfileName", name))

	err := vpcs.Apiclient.FileShareService().GetShareProfile(name, vpcs.Logger)

	if err != nil {
		vpcs.Logger.Warn("Volume Profile is not valid...", zap.Reflect("VolumeProfileName", name))
		return err
	}

	vpcs.Logger.Info("Volume Profile is valid...", zap.Reflect("VolumeProfileName", name))

	return nil
}

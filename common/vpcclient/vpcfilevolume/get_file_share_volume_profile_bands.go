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
	"fmt"

	"go.uber.org/zap"
)

// GetShareProfileBands is not supported on the direct VPC (RIAAS) session.
// Volume profile band data is only available via the armada-storage-api proxy
// (IKS session). Callers on IKS clusters use IKSVolumeService.GetShareProfileBands.
func (vs *FileShareService) GetShareProfileBands(profile string, ctxLogger *zap.Logger) ([]ShareProfileBand, error) {
	ctxLogger.Warn("GetShareProfileBands is not supported on the VPC RIAAS session; use the IKS session",
		zap.String("profile", profile))
	return nil, fmt.Errorf("GetShareProfileBands is only available via the IKS session (armada-storage-api proxy)")
}

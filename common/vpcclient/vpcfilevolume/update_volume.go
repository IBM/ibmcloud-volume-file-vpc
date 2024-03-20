/**
 * Copyright 2020 IBM Corp.
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

// Package vpcvolume ...
package vpcfilevolume

import (
	"errors"

	"github.com/IBM/ibmcloud-volume-file-vpc/common/vpcclient/models"
	"go.uber.org/zap"
)

// UpdateVolume POSTs to /volumes. Riaas/VPC does have volume update support yet
func (vs *FileShareService) UpdateVolume(volumeTemplate *models.Volume, ctxLogger *zap.Logger) error {
	return errors.New("unsupported Operation")
}
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

// Package models ...
package models

import (
	"strconv"
	"time"

	"github.com/IBM/ibmcloud-volume-interface/lib/provider"
)

const (
	//ClusterIDTagName ...
	ClusterIDTagName = "clusterid"
	//VolumeStatus ...
	VolumeStatus = "status"
)

// Volume ...
type Volume struct {
	Href          string         `json:"href,omitempty"`
	ID            string         `json:"id,omitempty"`
	Name          string         `json:"name,omitempty"`
	Capacity      int64          `json:"capacity,omitempty"`
	Iops          int64          `json:"iops,omitempty"`
	ResourceGroup *ResourceGroup `json:"resource_group,omitempty"`
	Tags          []string       `json:"tags,omitempty"` //We need to validate and remove this if not required.

	CRN        string     `json:"crn,omitempty"`
	Cluster    string     `json:"cluster,omitempty"`
	Provider   string     `json:"provider,omitempty"`
	Status     StatusType `json:"status,omitempty"`
	VolumeType string     `json:"volume_type,omitempty"`
}

// Share ...
type Share struct {
	CRN           string         `json:"crn,omitempty"`
	Href          string         `json:"href,omitempty"`
	ID            string         `json:"id,omitempty"`
	Name          string         `json:"name,omitempty"`
	Size          int64          `json:"size,omitempty"`
	Iops          int64          `json:"iops,omitempty"`
	EncryptionKey *EncryptionKey `json:"encryption_key,omitempty"`
	ResourceGroup *ResourceGroup `json:"resource_group,omitempty"`
	InitialOwner  *InitialOwner  `json:"initial_owner,omitempty"`
	Profile       *Profile       `json:"profile,omitempty"`
	CreatedAt     *time.Time     `json:"created_at,omitempty"`
	// Status of share named - deleted, deleting, failed, pending, stable, updating, waiting, suspended
	Status            StatusType     `json:"lifecycle_state,omitempty"`
	ShareTargets      *[]ShareTarget `json:"mount_targets,omitempty"`
	Zone              *Zone          `json:"zone,omitempty"`
	AccessControlMode string         `json:"access_control_mode,omitempty"`
}

// ListShareTargerFilters ...
type ListShareTargetFilters struct {
	ShareTargetName string `json:"name,omitempty"`
}

// ListShareFilters ...
type ListShareFilters struct {
	ResourceGroupID string `json:"resource_group.id,omitempty"`
	ShareName       string `json:"name,omitempty"`
}

// ShareList ...
type ShareList struct {
	First      *HReference `json:"first,omitempty"`
	Next       *HReference `json:"next,omitempty"`
	Shares     []*Share    `json:"shares"`
	Limit      int         `json:"limit,omitempty"`
	TotalCount int         `json:"total_count,omitempty"`
}

// HReference ...
type HReference struct {
	Href string `json:"href,omitempty"`
}

// NewVolume created model volume from provider volume
func NewVolume(volumeRequest provider.Volume) Volume {
	// Build the template to send to backend

	volume := Volume{
		ID:         volumeRequest.VolumeID,
		CRN:        volumeRequest.CRN,
		Tags:       volumeRequest.VPCVolume.Tags,
		Provider:   string(volumeRequest.Provider),
		VolumeType: string(volumeRequest.VolumeType),
	}
	if volumeRequest.Name != nil {
		volume.Name = *volumeRequest.Name
	}
	if volumeRequest.Capacity != nil {
		volume.Capacity = int64(*volumeRequest.Capacity)
	}

	if volumeRequest.VPCVolume.ResourceGroup != nil {
		volume.ResourceGroup = &ResourceGroup{
			ID:   volumeRequest.VPCVolume.ResourceGroup.ID,
			Name: volumeRequest.VPCVolume.ResourceGroup.Name,
		}
	}

	if volumeRequest.Iops != nil {
		value, err := strconv.ParseInt(*volumeRequest.Iops, 10, 64)
		if err != nil {
			volume.Iops = 0
		}
		volume.Iops = value
	}

	volume.Cluster = volumeRequest.Attributes[ClusterIDTagName]
	volume.Status = StatusType(volumeRequest.Attributes[VolumeStatus])

	return volume
}

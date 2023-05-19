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
	"errors"
	"net/url"
	"strings"
	"time"

	userError "github.com/IBM/ibmcloud-volume-file-vpc/common/messages"
	"github.com/IBM/ibmcloud-volume-file-vpc/common/vpcclient/models"
	"github.com/IBM/ibmcloud-volume-interface/lib/metrics"
	"github.com/IBM/ibmcloud-volume-interface/lib/provider"
	"github.com/IBM/ibmcloud-volume-interface/lib/utils/reasoncode"

	"go.uber.org/zap"
)

// VpcVolumeAccessPoint ...
const (
	StatusStable   = "stable"
	StatusDeleting = "deleting"
	StatusDeleted  = "deleted"
)

// VolumeAccessPoint create volume target based on given volume accessPoint request
func (vpcs *VPCSession) CreateVolumeAccessPoint(volumeAccessPointRequest provider.VolumeAccessPointRequest) (*provider.VolumeAccessPointResponse, error) {
	vpcs.Logger.Debug("Entry of CreateVolumeAccessPoint method...")
	defer vpcs.Logger.Debug("Exit from CreateVolumeAccessPoint method...")
	defer metrics.UpdateDurationFromStart(vpcs.Logger, "CreateVolumeAccessPoint", time.Now())
	var err error
	vpcs.Logger.Info("Validating basic inputs for CreateVolumeAccessPoint method...", zap.Reflect("volumeAccessPointRequest", volumeAccessPointRequest))
	err = vpcs.validateVolumeAccessPointRequest(volumeAccessPointRequest)
	if err != nil {
		return nil, err
	}
	var volumeAccessPointResult *models.ShareTarget
	var varp *provider.VolumeAccessPointResponse

	var subnet *models.Subnet
	var subnetID string

	volumeAccessPoint := models.NewShareTarget(volumeAccessPointRequest)

	err = vpcs.APIRetry.FlexyRetry(vpcs.Logger, func() (error, bool) {
		/*First , check if volume target is already created
		Even if we remove this check RIAAS will respond "shares_target_vpc_duplicate" erro code.
		We need to again do GetVolumeAccessPoint to fetch the already created access point */
		vpcs.Logger.Info("Checking if volume accessPoint is already created by other thread")
		currentVolAccessPoint, err := vpcs.GetVolumeAccessPoint(volumeAccessPointRequest)
		if err == nil && currentVolAccessPoint != nil {
			vpcs.Logger.Info("Volume accessPoint is already created", zap.Reflect("currentVolAccessPoint", currentVolAccessPoint))
			varp = currentVolAccessPoint
			return nil, true // stop retry volume accessPoint already created
		}

		// If ENI is enabled
		if volumeAccessPointRequest.AccessControlMode == SecurityGroupMode {
			volumeAccessPoint.VPC = nil
			volumeAccessPoint.EncryptionInTransit = volumeAccessPointRequest.EncryptionInTransit
			volumeAccessPoint.VirtualNetworkInterface = &models.VirtualNetworkInterface{
				SecurityGroups: volumeAccessPointRequest.SecurityGroups,
				ResourceGroup:  volumeAccessPointRequest.ResourceGroup,
			}

			// If primaryIP.ID is provided subnet is not mandatory for rest of the cases it is mandatory
			if volumeAccessPointRequest.PrimaryIP == nil || len(volumeAccessPointRequest.PrimaryIP.ID) == 0 {

				if len(volumeAccessPointRequest.SubnetID) == 0 {
					vpcs.Logger.Info("Getting subnet from VPC provider...")
					subnet, err = vpcs.getSubnet(volumeAccessPointRequest)
					// Keep retry, until we get the proper volumeAccessPointResult object
					if err != nil && subnet == nil {
						return err, skipRetryForObviousErrors(err)
					}
					subnetID = subnet.ID

				} else {
					vpcs.Logger.Info("Using subnet provided by user...", zap.Reflect("subnetID", volumeAccessPointRequest.SubnetID))
					subnetID = volumeAccessPointRequest.SubnetID
				}

				volumeAccessPoint.VirtualNetworkInterface.Subnet = &models.SubnetRef{
					ID: subnetID,
				}
			}

			if volumeAccessPointRequest.PrimaryIP != nil {
				vpcs.Logger.Info("Primary IP ID provided using it for virtual network interface...")
				volumeAccessPoint.VirtualNetworkInterface.PrimaryIP = (*models.PrimaryIP)(volumeAccessPointRequest.PrimaryIP)
			}
		}

		//Try creating volume accessPoint if it's not already created or there is error in getting current volume accessPoint
		vpcs.Logger.Info("Creating volume accessPoint from VPC provider...")
		volumeAccessPointResult, err = vpcs.Apiclient.FileShareService().CreateFileShareTarget(&volumeAccessPoint, vpcs.Logger)
		// Keep retry, until we get the proper volumeAccessPointResult object
		if err != nil && volumeAccessPointResult == nil {
			return err, skipRetryForObviousErrors(err)
		}
		varp = volumeAccessPointResult.ToVolumeAccessPointResponse()

		return err, true // stop retry as no error
	})

	if err != nil {
		userErr := userError.GetUserError(string(userError.CreateVolumeAccessPointFailed), err, volumeAccessPointRequest.VolumeID, volumeAccessPointRequest.VPCID)
		return nil, userErr
	}
	vpcs.Logger.Info("Successfully created volume accessPoint from VPC provider", zap.Reflect("volumeAccessPointResponse", varp))
	varp.VolumeID = volumeAccessPointRequest.VolumeID
	return varp, nil
}

// validateVolume validating volume ID and VPC ID
func (vpcs *VPCSession) validateVolumeAccessPointRequest(volumeAccessPointRequest provider.VolumeAccessPointRequest) error {
	var err error
	// Check for VolumeID - required validation
	if len(volumeAccessPointRequest.VolumeID) == 0 {
		err = userError.GetUserError(string(reasoncode.ErrorRequiredFieldMissing), nil, "VolumeID")
		vpcs.Logger.Error("volumeAccessPointRequest.VolumeID is required", zap.Error(err))
		return err
	}

	// Check for VPC ID - required validation
	if len(volumeAccessPointRequest.VPCID) == 0 && len(volumeAccessPointRequest.SubnetID) == 0 && len(volumeAccessPointRequest.AccessPointID) == 0 {
		err = userError.GetUserError(string(reasoncode.ErrorRequiredFieldMissing), nil, "VPCID")
		vpcs.Logger.Error("One of volumeAccessPointRequest.VPCID, volumeAccessPointRequest.SubnetID and volumeAccessPointRequest.AccessPoint is required", zap.Error(err))
		return err
	}
	return nil
}

// GetSubnet  get the subnet based on the request
func (vpcs *VPCSession) getSubnet(volumeAccessPointRequest provider.VolumeAccessPointRequest) (*models.Subnet, error) {
	vpcs.Logger.Debug("Entry of GetSubnet method...", zap.Reflect("volumeAccessPointRequest", volumeAccessPointRequest))
	defer vpcs.Logger.Debug("Exit from GetSubnet method...")
	var err error

	// Get Subnet by VPC ID and zone. This is inefficient operation which requires iteration over subnet list
	subnet, err := vpcs.getSubnetByVPCIDAndZone(volumeAccessPointRequest)
	vpcs.Logger.Info("getSubnetByVPCIDAndZone response", zap.Reflect("subnet", subnet), zap.Error(err))
	return subnet, err
}

func (vpcs *VPCSession) getSubnetByVPCIDAndZone(volumeAccessPointRequest provider.VolumeAccessPointRequest) (*models.Subnet, error) {
	vpcs.Logger.Debug("Entry of getSubnetByVPCIDAndZone()")
	defer vpcs.Logger.Debug("Exit from getSubnetByVPCIDAndZone()")
	vpcs.Logger.Info("Getting getSubnetByVPCIDAndZone from VPC provider...")
	var err error
	var subnets *models.SubnetList
	var start = ""

	filters := &models.ListSubnetFilters{ResourceGroupID: volumeAccessPointRequest.ResourceGroup.ID}

	for {

		subnets, err = vpcs.Apiclient.FileShareService().ListSubnets(10, start, filters, vpcs.Logger)

		if err != nil {
			// API call is failed
			userErr := userError.GetUserError("ListSubnetsFailed", err)
			return nil, userErr
		}

		// Iterate over the subnet list for given volume
		if subnets != nil {
			subnetList := subnets.Subnets
			for _, subnetItem := range subnetList {
				// Check if VPC ID and zone name is matching with requested input
				if subnetItem.VPC != nil && subnetItem.VPC.ID == volumeAccessPointRequest.VPCID && subnetItem.Zone != nil && subnetItem.Zone.Name == volumeAccessPointRequest.Zone && strings.Contains(volumeAccessPointRequest.SubnetIDList, subnetItem.ID) {
					vpcs.Logger.Info("Successfully found subnet", zap.Reflect("subnetItem", subnetItem))
					return subnetItem, nil
				}
			}

			if subnets.Next == nil {
				break // No more pages, exit the loop
			}

			// Fetch the start of next page
			startUrl, err := url.Parse(subnets.Next.Href)
			if err != nil {
				// API call is failed
				userErr := userError.GetUserError("NextSubnetPageParsingError", err, subnets.Next.Href)
				return nil, userErr
			}

			vpcs.Logger.Info("startUrl", zap.Reflect("startUrl", startUrl))
			start = startUrl.Query().Get("start") //parse query param into map
			if start == "" {
				// API call is failed
				userErr := userError.GetUserError("StartSubnetIDEmpty", err, startUrl)
				return nil, userErr
			}

		}
	}

	// No volume Subnet found in the  list. So return error
	userErr := userError.GetUserError(string("SubnetFindFailedWithZoneAndVPC"), errors.New("no subnet found"), volumeAccessPointRequest.Zone, volumeAccessPointRequest.VPCID)
	vpcs.Logger.Error("Subnet not found", zap.Error(err))
	return nil, userErr
}

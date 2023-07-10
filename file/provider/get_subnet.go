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
	userError "github.com/IBM/ibmcloud-volume-file-vpc/common/messages"
	"github.com/IBM/ibmcloud-volume-file-vpc/common/vpcclient/models"
	"github.com/IBM/ibmcloud-volume-interface/lib/provider"
	"go.uber.org/zap"
	"net/url"
	"strings"
)

// / GetSubnet  get the subnet based on the request
func (vpcs *VPCSession) GetSubnetForVolumeAccessPoint(subnetRequest provider.SubnetRequest) (string, error) {
	vpcs.Logger.Info("Entry of GetSubnetForVolumeAccessPoint method...", zap.Reflect("subnetRequest", subnetRequest))
	defer vpcs.Logger.Info("Exit from GetSubnetForVolumeAccessPoint method...")
	var err error

	// Get Subnet by zone and cluster subnet list. This is inefficient operation which requires iteration over subnet list
	subnet, err := vpcs.getSubnetByZoneAndSubnetID(subnetRequest)
	vpcs.Logger.Info("getSubnetByVPCIDAndZone response", zap.Reflect("subnet", subnet), zap.Error(err))
	return subnet, err
}

func (vpcs *VPCSession) getSubnetByZoneAndSubnetID(subnetRequest provider.SubnetRequest) (string, error) {
	vpcs.Logger.Debug("Entry of getSubnetByVPCIDAndZone()")
	defer vpcs.Logger.Debug("Exit from getSubnetByVPCIDAndZone()")
	vpcs.Logger.Info("Getting getSubnetByVPCIDAndZone from VPC provider...")
	var err error
	var start = ""

	filters := &models.ListSubnetFilters{ResourceGroupID: subnetRequest.ResourceGroup.ID}

	for {

		subnets, err := vpcs.Apiclient.FileShareService().ListSubnets(pageSize, start, filters, vpcs.Logger)

		if err != nil {
			// API call is failed
			userErr := userError.GetUserError("ListSubnetsFailed", err)
			return "", userErr
		}

		// Iterate over the subnet list for given volume
		if subnets != nil {
			subnetList := subnets.Subnets
			for _, subnetItem := range subnetList {
				// Check if zone and subnet is matching with requested input
				if subnetItem.Zone != nil && subnetItem.Zone.Name == subnetRequest.Zone && strings.Contains(subnetRequest.SubnetIDList, subnetItem.ID) {
					vpcs.Logger.Info("Successfully found subnet", zap.Reflect("subnetItem", subnetItem))
					return subnetItem.ID, nil
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
				return "", userErr
			}

			vpcs.Logger.Info("startUrl", zap.Reflect("startUrl", startUrl))
			start = startUrl.Query().Get("start") //parse query param into map
			if start == "" {
				// API call is failed
				userErr := userError.GetUserError("StartSubnetIDEmpty", err, startUrl)
				return "", userErr
			}

		}
	}

	// No volume Subnet found in the  list. So return error
	userErr := userError.GetUserError(string("SubnetFindFailedWithZoneAndSubnetID"), errors.New("no subnet found"), subnetRequest.Zone, subnetRequest.SubnetIDList)
	vpcs.Logger.Error("Subnet not found", zap.Error(err))
	return "", userErr
}
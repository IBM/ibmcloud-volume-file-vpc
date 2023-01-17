/*******************************************************************************
 * IBM Confidential
 * OCO Source Materials
 * IBM Cloud Kubernetes Service, 5737-D43
 * (C) Copyright IBM Corp. 2023 All Rights Reserved.
 * The source code for this program is not published or otherwise divested of
 * its trade secrets, irrespective of what has been deposited with
 * the U.S. Copyright Office.
 ******************************************************************************/

// Package vpcfilevolume ...
package vpcfilevolume

import (
	"strconv"
	"time"

	"github.com/IBM/ibmcloud-volume-file-vpc/common/vpcclient/client"
	"github.com/IBM/ibmcloud-volume-file-vpc/common/vpcclient/models"
	util "github.com/IBM/ibmcloud-volume-interface/lib/utils"
	"go.uber.org/zap"
)

// ListSecurityGroups GETs /security_groups
func (vs *FileShareService) ListSecurityGroups(limit int, start string, filters *models.ListSecurityGroupFilters, ctxLogger *zap.Logger) (*models.SecurityGroupList, error) {
	ctxLogger.Debug("Entry Backend ListSecurityGroups")
	defer ctxLogger.Debug("Exit Backend ListSecurityGroups")

	defer util.TimeTracker("ListSecurityGroups", time.Now())

	operation := &client.Operation{
		Name:        "ListSecurityGroups",
		Method:      "GET",
		PathPattern: securityGroups,
	}

	var securityGroups models.SecurityGroupList
	var apiErr models.Error

	request := vs.client.NewRequest(operation)
	ctxLogger.Info("Equivalent curl command", zap.Reflect("URL", request.URL()), zap.Reflect("Operation", operation))

	req := request.JSONSuccess(&securityGroups).JSONError(&apiErr)

	if limit > 0 {
		req.AddQueryValue("limit", strconv.Itoa(limit))
	}

	if start != "" {
		req.AddQueryValue("start", start)
	}

	if filters != nil {
		if filters.ResourceGroupID != "" {
			req.AddQueryValue("resource_group.id", filters.ResourceGroupID)
		}
		if filters.VPCID != "" {
			req.AddQueryValue("vpc.id", filters.VPCID)
		}
	}

	ctxLogger.Info("Equivalent curl command", zap.Reflect("URL", req.URL()))

	_, err := req.Invoke()
	if err != nil {
		return nil, err
	}

	return &securityGroups, nil
}

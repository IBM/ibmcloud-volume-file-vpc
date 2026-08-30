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
	"context"
	"net/http"

	"github.com/IBM/ibmcloud-volume-file-vpc/common/catalog"
)

// HTTPDoer is the minimal interface required from an HTTP client so that it
// can be replaced with a test double. *http.Client satisfies this interface.
// Re-exported here so callers of this package need not import common/catalog.
type HTTPDoer = catalog.HTTPDoer

// CatalogBand is a type alias for catalog.CatalogBand, re-exported so callers
// of this package need not import common/catalog directly.
type CatalogBand = catalog.CatalogBand

// FetchCapacityBands fetches the capacity/IOPS bands for the named VPC file
// volume profile (e.g. "dp2") from the armada-storage-api proxy endpoint,
// returning them ordered from the smallest capacity band to the largest.
//
// profile is the profile name to query (e.g. "dp2", "rfs").
//
// iksBaseURL is the IKS private token-exchange base URL already configured for
// this cluster (conf.VPCConfig.IKSTokenExchangePrivateURL), e.g.
// "https://us-south.containers.cloud.ibm.com".
//
// Pass nil for httpClient to use http.DefaultClient.
// This is the only function in this package that performs network I/O.
// The caller decides when to invoke it and is responsible for caching the
// returned slice (e.g. once at driver startup).
//
// Returns an error if the endpoint is unreachable, returns a non-2xx status,
// or the response contains no valid bands.
func FetchCapacityBands(ctx context.Context, httpClient HTTPDoer, iksBaseURL string, profile string) ([]catalog.CatalogBand, error) {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	return catalog.NewCatalogClient(httpClient, iksBaseURL).FetchVolumeProfileBands(ctx, profile)
}

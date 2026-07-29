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
	"fmt"
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

// CapacityRoundoff is the interface the CSI driver uses to determine the
// minimum capacity (GiB) that satisfies a requested IOPS value for the dp2
// profile.
//
// Implementations must be safe for concurrent use after construction.
type CapacityRoundoff interface {
	// GetMinCapacityForIops returns the minimum share capacity in GiB that
	// satisfies the requested IOPS according to the dp2 catalog bands.
	//
	// It scans the bands (ordered from the smallest capacity band to the
	// largest) and returns the CapMin of the first band whose IOPSMax is >=
	// requestedIops.
	//
	// Returns an error if no band covers the requested IOPS.
	GetMinCapacityForIops(requestedIops int) (int, error)
}

// capacityRoundoff is the production implementation of CapacityRoundoff.
// It is a pure algorithm over a fixed band slice; it never touches the network.
type capacityRoundoff struct {
	bands []catalog.CatalogBand
}

// FetchCapacityBandsDP2 fetches the dp2 capacity/IOPS bands from the IBM
// Global Catalog API using the supplied HTTP client and returns them as a
// slice ordered from the smallest capacity band to the largest.
//
// referenceURL is any IBM Cloud endpoint URL already in use by the caller
// (e.g. conf.VPCConfig.G2TokenExchangeURL). It is passed to
// catalog.EndpointForEnv to select the correct stage or production Global
// Catalog endpoint: URLs containing "test" or "stage" resolve to the staging
// catalog, all others resolve to the production catalog.
//
// Pass nil for httpClient to use http.DefaultClient.
// This is the only function in this package that performs network I/O.
// The caller decides when to call it and is responsible for caching the
// returned slice (e.g. once at driver startup).
//
// Returns an error if the catalog is unreachable, returns a non-2xx status,
// or contains no valid bands.
func FetchCapacityBandsDP2(httpClient HTTPDoer, referenceURL string) ([]catalog.CatalogBand, error) {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	return catalog.NewCatalogClient(httpClient, referenceURL).FetchCatalogBandsDP2()
}

// NewCapacityRoundoff constructs a CapacityRoundoff from a pre-fetched slice
// of dp2 catalog bands.
//
// The caller is responsible for fetching the bands (via FetchCapacityBandsDP2)
// and for deciding when to refresh them. This keeps the service a pure
// algorithm with no I/O — the driver can re-create it on any refresh cycle
// without this function ever making an HTTP call.
//
// Returns an error if bands is empty.
func NewCapacityRoundoff(bands []catalog.CatalogBand) (CapacityRoundoff, error) {
	if len(bands) == 0 {
		return nil, fmt.Errorf("provider: cannot create CapacityRoundoff with empty bands slice")
	}
	return &capacityRoundoff{bands: bands}, nil
}

// GetMinCapacityForIops satisfies CapacityRoundoff.
// It scans the band slice and returns the CapMin of the first band whose
// IOPSMax >= requestedIops.
func (s *capacityRoundoff) GetMinCapacityForIops(requestedIops int) (int, error) {
	for _, band := range s.bands {
		if band.IOPSMax >= requestedIops {
			return band.CapMin, nil
		}
	}
	return 0, fmt.Errorf("provider: no dp2 catalog band covers iops=%d", requestedIops)
}

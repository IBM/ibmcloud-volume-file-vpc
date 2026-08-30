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

// Package catalog provides a minimal HTTP client for fetching VPC file volume
// profile capacity-to-IOPS bands from the armada-storage-api proxy endpoint.
// Authentication is handled by armada-storage-api itself; the library only
// needs network reachability to the IKS private endpoint.
package catalog

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// volumeProfileBasePath is the path prefix for the volume profile endpoint on
// armada-storage-api. The profile name is appended as a query parameter.
// IKSTokenExchangePrivateURL is a bare host (e.g. "https://us-south.containers.cloud.ibm.com")
// so the full /v2/storage prefix must be included here.
const volumeProfileBasePath = "/v2/storage/vpc/volumeProfile?profile="

// CatalogBand represents a single capacity/IOPS band for a VPC file volume profile.
// Each band defines the inclusive GiB capacity range and the inclusive IOPS
// range that are valid together.
type CatalogBand struct {
	// CapMin is the minimum share size (GiB) for this band.
	CapMin int64
	// CapMax is the maximum share size (GiB) for this band.
	CapMax int64
	// IOPSMin is the minimum IOPS value allowed for this band.
	IOPSMin int64
	// IOPSMax is the maximum IOPS value allowed for this band.
	IOPSMax int64
}

// VolumeProfileBand is one element in the bands returned by armada-storage-api.
// It is the canonical shared type used across the call chain:
// IKSVolumeService → IksVpcSession → file/provider → CSI driver's VolumeProfileBandsFetcher.
//
// Using a single type alias throughout ensures the interface type assertion
// in the driver succeeds at runtime.
type VolumeProfileBand struct {
	CapacityMin int64 `json:"capacityMin"`
	CapacityMax int64 `json:"capacityMax"`
	IOPSMin     int64 `json:"iopsMin"`
	IOPSMax     int64 `json:"iopsMax"`
}

// volumeProfileResponse mirrors the JSON envelope returned by armada-storage-api
// GET /v2/storage/vpc/volumeProfile?profile=<name>.
// The response uses a config_validation array where each entry has a "capacity"
// object and optional metric objects (e.g. "iops" for dp2, "throughput" for rfs).
type volumeProfileResponse struct {
	ID               string                  `json:"id"`
	ConfigValidation []configValidationEntry `json:"config_validation"`
}

// configValidationEntry is one entry in the config_validation array.
type configValidationEntry struct {
	Capacity capacityRange `json:"capacity"`
	IOPS     *metricRange  `json:"iops,omitempty"`
}

type capacityRange struct {
	Min int64 `json:"min"`
	Max int64 `json:"max"`
}

type metricRange struct {
	Min int64 `json:"min"`
	Max int64 `json:"max"`
}

// HTTPDoer is the minimal interface required from an HTTP client so it can be
// replaced with a test double in unit tests.
type HTTPDoer interface {
	Do(req *http.Request) (*http.Response, error)
}

// CatalogClient fetches and parses VPC file volume profile capacity/IOPS bands
// from the armada-storage-api proxy endpoint.
// Construct one via NewCatalogClient.
type CatalogClient struct {
	// baseURL is the bare host base URL, e.g. "https://us-south.containers.cloud.ibm.com".
	// volumeProfileBasePath already includes the /v2/storage prefix.
	baseURL    string
	httpClient HTTPDoer
}

// NewCatalogClient returns a CatalogClient that calls armada-storage-api using
// iksBaseURL as the base. iksBaseURL is the IKS private token-exchange bare host URL
// (conf.VPCConfig.IKSTokenExchangePrivateURL), e.g. "https://us-south.containers.cloud.ibm.com".
// Pass nil for httpClient to use http.DefaultClient.
func NewCatalogClient(httpClient HTTPDoer, iksBaseURL string) *CatalogClient {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	return &CatalogClient{
		baseURL:    strings.TrimRight(iksBaseURL, "/"),
		httpClient: httpClient,
	}
}

// FetchVolumeProfileBands retrieves the capacity/IOPS bands for the named VPC
// file volume profile (e.g. "dp2") from armada-storage-api, returning them
// ordered from the smallest capacity band to the largest (as returned by the API).
//
// The endpoint GET /v2/storage/vpc/volumeProfile?profile=<name> returns a
// config_validation array where each entry contains a "capacity" object
// (with min/max in GiB) and an optional "iops" object (with min/max).
// Entries without an "iops" field (e.g. throughput-only bands) are skipped.
//
// Returns a non-nil error if the HTTP request fails, the response status is
// not 2xx, the body cannot be decoded, or the API returns no IOPS bands.
func (c *CatalogClient) FetchVolumeProfileBands(ctx context.Context, profile string) ([]CatalogBand, error) {
	url := c.baseURL + volumeProfileBasePath + profile
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("volume profile: build request: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("volume profile: HTTP request to %s: %w", url, err)
	}
	defer resp.Body.Close() //nolint:errcheck

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("volume profile: read response body: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return nil, fmt.Errorf("volume profile: unexpected status %d from %s: %s", resp.StatusCode, url, string(body))
	}

	var parsed volumeProfileResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("volume profile: decode response: %w", err)
	}

	if len(parsed.ConfigValidation) == 0 {
		return nil, fmt.Errorf("volume profile: %q profile returned no bands", profile)
	}

	var bands []CatalogBand
	for _, entry := range parsed.ConfigValidation {
		if entry.IOPS == nil {
			// skip entries without iops (e.g. throughput-only bands)
			continue
		}
		bands = append(bands, CatalogBand{
			CapMin:  entry.Capacity.Min,
			CapMax:  entry.Capacity.Max,
			IOPSMin: entry.IOPS.Min,
			IOPSMax: entry.IOPS.Max,
		})
	}

	if len(bands) == 0 {
		return nil, fmt.Errorf("volume profile: %q profile returned no bands", profile)
	}

	return bands, nil
}

// ParseVolumeProfileBands decodes a raw armada-storage-api
// GET /v2/storage/vpc/volumeProfile response body and returns the
// capacity/IOPS bands as []VolumeProfileBand.
//
// It is exported so that callers (e.g. IKSVolumeService) that issue the HTTP
// request themselves via a different transport can re-use this parsing logic
// without duplicating the JSON struct definitions.
func ParseVolumeProfileBands(body []byte, profile string) ([]VolumeProfileBand, error) {
	var parsed volumeProfileResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("volume profile: decode response: %w", err)
	}

	if len(parsed.ConfigValidation) == 0 {
		return nil, fmt.Errorf("GetVolumeProfileBands: no bands returned for profile %q", profile)
	}

	bands := make([]VolumeProfileBand, 0, len(parsed.ConfigValidation))
	for _, entry := range parsed.ConfigValidation {
		if entry.IOPS == nil {
			continue
		}
		bands = append(bands, VolumeProfileBand{
			CapacityMin: entry.Capacity.Min,
			CapacityMax: entry.Capacity.Max,
			IOPSMin:     entry.IOPS.Min,
			IOPSMax:     entry.IOPS.Max,
		})
	}

	if len(bands) == 0 {
		return nil, fmt.Errorf("GetVolumeProfileBands: no iops bands found for profile %q", profile)
	}
	return bands, nil
}

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

// Package catalog provides a minimal HTTP client for fetching dp2 volume profile
// capacity-to-IOPS bands from the armada-storage-api proxy endpoint.
// Authentication is handled by armada-storage-api itself; the library only
// needs network reachability to the IKS private endpoint.
package catalog

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// dp2VolumeProfilePath is the path appended to the armada-storage-api base URL
// that reaches the dp2 volume profile bands endpoint. The full path includes
// /v2/storage because IKSTokenExchangePrivateURL is a bare host
// (e.g. "https://us-south.containers.cloud.ibm.com") with no path prefix.
const dp2VolumeProfilePath = "/v2/storage/vpc/getVolumeProfiles?profile=dp2"

// CatalogBand represents a single capacity/IOPS band from the DP2 profile.
// Each band defines the inclusive GiB capacity range and the inclusive IOPS
// range that are valid together.
type CatalogBand struct {
	// CapMin is the minimum share size (GiB) for this band.
	CapMin int
	// CapMax is the maximum share size (GiB) for this band.
	CapMax int
	// IOPSMin is the minimum IOPS value allowed for this band.
	IOPSMin int
	// IOPSMax is the maximum IOPS value allowed for this band.
	IOPSMax int
}

// dp2VolumeProfileResponse mirrors the JSON returned by armada-storage-api
// GET /v2/storage/vpc/getVolumeProfiles?profile=dp2.
type dp2VolumeProfileResponse struct {
	Bands []dp2VolumeProfileBand `json:"bands"`
}

// dp2VolumeProfileBand is one element in the bands array returned by armada-storage-api.
// Field names match the JSON tags used by armada-storage-api globalcatalog.CatalogBand.
type dp2VolumeProfileBand struct {
	CapacityMin int64 `json:"capacityMin"`
	CapacityMax int64 `json:"capacityMax"`
	IOPSMin     int64 `json:"iopsMin"`
	IOPSMax     int64 `json:"iopsMax"`
}

// HTTPDoer is the minimal interface required from an HTTP client so it can be
// replaced with a test double in unit tests.
type HTTPDoer interface {
	Do(req *http.Request) (*http.Response, error)
}

// CatalogClient fetches and parses dp2 volume profile capacity/IOPS bands from
// armada-storage-api.
// Construct one via NewCatalogClient or NewCatalogClientWithBaseURL.
type CatalogClient struct {
	// baseURL is the bare host base URL, e.g. "https://us-south.containers.cloud.ibm.com".
	// dp2VolumeProfilePath already includes the /v2/storage prefix.
	baseURL    string
	httpClient HTTPDoer
}

// NewCatalogClient returns a CatalogClient that calls armada-storage-api using
// iksBaseURL as the base. iksBaseURL is the IKS private token-exchange bare host URL
// (conf.VPCConfig.IKSTokenExchangePrivateURL), e.g. "https://us-south.containers.cloud.ibm.com".
// dp2VolumeProfilePath provides the full /v2/storage/vpc/... suffix.
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

// NewCatalogClientWithBaseURL constructs a CatalogClient with an explicit base
// URL. Intended for unit testing where the caller controls the full URL.
func NewCatalogClientWithBaseURL(httpClient HTTPDoer, baseURL string) *CatalogClient {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	return &CatalogClient{
		baseURL:    strings.TrimRight(baseURL, "/"),
		httpClient: httpClient,
	}
}

// FetchVolumeProfileBandsDP2 retrieves the dp2 volume profile capacity/IOPS bands from
// armada-storage-api and returns them ordered from the smallest capacity band
// to the largest (as returned by the API).
//
// Returns a non-nil error if the HTTP request fails, the response status is
// not 2xx, the body cannot be decoded, or the API returns no bands.
func (c *CatalogClient) FetchVolumeProfileBandsDP2() ([]CatalogBand, error) {
	url := c.baseURL + dp2VolumeProfilePath
	req, err := http.NewRequest(http.MethodGet, url, nil)
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

	var parsed dp2VolumeProfileResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("volume profile: decode response: %w", err)
	}

	if len(parsed.Bands) == 0 {
		return nil, fmt.Errorf("volume profile: dp2 volume profile returned no bands")
	}

	bands := make([]CatalogBand, len(parsed.Bands))
	for i, b := range parsed.Bands {
		bands[i] = CatalogBand{
			CapMin:  int(b.CapacityMin),
			CapMax:  int(b.CapacityMax),
			IOPSMin: int(b.IOPSMin),
			IOPSMax: int(b.IOPSMax),
		}
	}
	return bands, nil
}

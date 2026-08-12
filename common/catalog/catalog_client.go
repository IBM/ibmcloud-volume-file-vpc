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

// Package catalog provides a minimal HTTP client for fetching dp2 profile
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

// dp2CatalogPath is the path appended to the armada-storage-api base URL that
// reaches the DP2 catalog proxy endpoint registered under /v2/storage/vpc/.
const dp2CatalogPath = "/vpc/getVolumeProfiles/dp2"

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

// dp2CatalogResponse mirrors the JSON returned by armada-storage-api
// GET /v2/storage/vpc/getCatalog/dp2.
type dp2CatalogResponse struct {
	Bands []dp2CatalogBand `json:"bands"`
}

// dp2CatalogBand is one element in the bands array returned by armada-storage-api.
// Field names match the JSON tags used by armada-storage-api globalcatalog.CatalogBand.
type dp2CatalogBand struct {
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

// CatalogClient fetches and parses dp2 capacity/IOPS bands from the
// armada-storage-api catalog proxy endpoint.
// Construct one via NewCatalogClient or NewCatalogClientWithBaseURL.
type CatalogClient struct {
	// baseURL is the armada-storage-api base URL including /v2/storage,
	// e.g. "https://us-south.containers.cloud.ibm.com/v2/storage".
	baseURL    string
	httpClient HTTPDoer
}

// NewCatalogClient returns a CatalogClient that calls armada-storage-api using
// iksBaseURL as the base. iksBaseURL is the IKS private token-exchange URL
// (conf.VPCConfig.IKSTokenExchangePrivateURL) which points at the correct
// armada-storage-api endpoint for the cluster's environment (stage/prod).
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

// FetchCatalogBandsDP2 retrieves the dp2 capacity/IOPS bands from
// armada-storage-api and returns them ordered from the smallest capacity band
// to the largest (as returned by the API).
//
// Returns a non-nil error if the HTTP request fails, the response status is
// not 2xx, the body cannot be decoded, or the API returns no bands.
func (c *CatalogClient) FetchCatalogBandsDP2() ([]CatalogBand, error) {
	url := c.baseURL + dp2CatalogPath
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("catalog: build request: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("catalog: HTTP request to %s: %w", url, err)
	}
	defer resp.Body.Close() //nolint:errcheck

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("catalog: read response body: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return nil, fmt.Errorf("catalog: unexpected status %d from %s: %s", resp.StatusCode, url, string(body))
	}

	var parsed dp2CatalogResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("catalog: decode response: %w", err)
	}

	if len(parsed.Bands) == 0 {
		return nil, fmt.Errorf("catalog: dp2 catalog returned no bands")
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

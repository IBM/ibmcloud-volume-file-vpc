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
// capacity-to-IOPS bands from the IBM Global Catalog API.
// No authentication is required — the endpoint is public.
// This package has no business logic and no caching; it only fetches and
// parses raw band data. Higher-level logic lives in common/file.
package catalog

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

const (
	// CatalogDP2ProdURL is the private IBM Global Catalog endpoint for the dp2
	// file-share profile used on production clusters.
	CatalogDP2ProdURL = "https://private.globalcatalog.cloud.ibm.com/api/v1/dp2"

	// CatalogDP2StageURL is the private IBM Global Catalog endpoint for the dp2
	// file-share profile used on staging (test) clusters.
	CatalogDP2StageURL = "https://private.globalcatalog.test.cloud.ibm.com/api/v1/dp2"
)

// EndpointForEnv returns the correct Global Catalog dp2 endpoint URL for the
// environment inferred from referenceURL.
func EndpointForEnv(referenceURL string) string {
	if strings.Contains(referenceURL, "test") || strings.Contains(referenceURL, "stage") {
		return CatalogDP2StageURL
	}
	return CatalogDP2ProdURL
}

// CatalogBand represents a single capacity/IOPS band from the IBM Global
// Catalog dp2 config_validation array. Each band defines the inclusive GiB
// capacity range and the inclusive IOPS range that are valid together.
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

// dp2Response is the minimal subset of the IBM Global Catalog JSON document
// required to extract the config_validation bands.
type dp2Response struct {
	Metadata struct {
		Other struct {
			Profile struct {
				ConfigValidation []struct {
					Capacity struct {
						Min   int    `json:"min"`
						Max   int    `json:"max"`
						Units string `json:"units"`
					} `json:"capacity"`
					Iops struct {
						Min  int    `json:"min"`
						Max  int    `json:"max"`
						Unit string `json:"unit"`
					} `json:"iops"`
				} `json:"config_validation"`
			} `json:"profile"`
		} `json:"other"`
	} `json:"metadata"`
}

// HTTPDoer is the minimal interface required from an HTTP client so it can be
// replaced with a test double in unit tests.
type HTTPDoer interface {
	Do(req *http.Request) (*http.Response, error)
}

// CatalogClient fetches and parses dp2 capacity/IOPS bands from the IBM Global
// Catalog API. Construct one via NewCatalogClient or NewCatalogClientWithURL.
type CatalogClient struct {
	url        string
	httpClient HTTPDoer
}

// NewCatalogClient returns a CatalogClient that calls the IBM Global Catalog
// dp2 endpoint selected for the given environment reference URL (see
// EndpointForEnv). Pass nil to use http.DefaultClient.
func NewCatalogClient(httpClient HTTPDoer, referenceURL string) *CatalogClient {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	return NewCatalogClientWithURL(httpClient, EndpointForEnv(referenceURL))
}

// NewCatalogClientWithURL returns a CatalogClient that calls the supplied
// catalog URL. Pass nil to use http.DefaultClient. Intended for unit testing.
func NewCatalogClientWithURL(httpClient HTTPDoer, url string) *CatalogClient {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	return &CatalogClient{
		url:        url,
		httpClient: httpClient,
	}
}

// FetchCatalogBandsDP2 retrieves the dp2 config_validation bands from the IBM
// Global Catalog API and returns them ordered from the smallest capacity band
// to the largest (as they appear in the catalog response).
//
// Returns a non-nil error if the HTTP request fails, the response status is
// not 2xx, the body cannot be decoded, any entry is malformed, or the catalog
// returns no bands.
func (c *CatalogClient) FetchCatalogBandsDP2() ([]CatalogBand, error) {
	req, err := http.NewRequest(http.MethodGet, c.url, nil)
	if err != nil {
		return nil, fmt.Errorf("catalog: build request: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("catalog: HTTP request to %s: %w", c.url, err)
	}
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return nil, fmt.Errorf("catalog: unexpected status %d from %s", resp.StatusCode, c.url)
	}

	var parsed dp2Response
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return nil, fmt.Errorf("catalog: decode response: %w", err)
	}

	raw := parsed.Metadata.Other.Profile.ConfigValidation
	if len(raw) == 0 {
		return nil, fmt.Errorf("catalog: dp2 catalog returned no config_validation bands")
	}

	bands := make([]CatalogBand, 0, len(raw))
	for i, entry := range raw {
		if entry.Capacity.Min <= 0 || entry.Capacity.Max <= 0 || entry.Iops.Max <= 0 {
			return nil, fmt.Errorf("catalog: invalid config_validation entry at index %d: capacity=[%d,%d] iops.max=%d",
				i, entry.Capacity.Min, entry.Capacity.Max, entry.Iops.Max)
		}
		bands = append(bands, CatalogBand{
			CapMin:  entry.Capacity.Min,
			CapMax:  entry.Capacity.Max,
			IOPSMin: entry.Iops.Min,
			IOPSMax: entry.Iops.Max,
		})
	}
	return bands, nil
}

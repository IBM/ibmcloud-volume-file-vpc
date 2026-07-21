/**
 * Copyright 2026 IBM Corp.
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

// Package catalog provides a thin HTTP client for the IBM Global Catalog API
// that fetches the dp2 capacity-to-IOPS validation bands.
// Business logic (round-off, caching) belongs in the provider layer; only the
// raw fetch and the shared data types are defined here.
package catalog

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"time"
)

const (
	// DefaultCatalogEndpoint is the IBM Global Catalog URL for the dp2
	// file-share profile, including the metadata needed to read validation bands.
	DefaultCatalogEndpoint = "https://globalcatalog.cloud.ibm.com/api/v1/dp2?include=metadata.other"

	defaultCatalogTimeout  = 60 * time.Second
	maxCatalogResponseSize = 4 << 20 // 4 MiB
)

// BandRange is an inclusive [Min, Max] integer interval used inside a Band.
type BandRange struct {
	Min int64 `json:"min"`
	Max int64 `json:"max"`
}

// Band is one row of the dp2 capacity-to-IOPS validation table published by
// the IBM Global Catalog. A share whose IOPS falls within [IOPS.Min, IOPS.Max]
// must have at least Capacity.Min GiB of storage.
type Band struct {
	Capacity BandRange `json:"capacity"`
	IOPS     BandRange `json:"iops"`
}

// catalogEntry mirrors the JSON structure returned by the Global Catalog API.
type catalogEntry struct {
	Metadata struct {
		Other struct {
			Profile struct {
				ConfigValidation []Band `json:"config_validation"`
			} `json:"profile"`
		} `json:"other"`
	} `json:"metadata"`
}

// Client is a stateless HTTP client for the IBM Global Catalog dp2 endpoint.
// It performs a fresh network call on every FetchBands invocation; callers
// that need caching should wrap it (e.g. the provider layer).
type Client struct {
	httpClient *http.Client
	endpoint   string
}

// NewClient creates a Client that talks to DefaultCatalogEndpoint.
// A nil httpClient is replaced with a default that has a 60-second timeout.
func NewClient(httpClient *http.Client) *Client {
	return NewClientWithEndpoint(httpClient, DefaultCatalogEndpoint)
}

// NewClientWithEndpoint creates a Client with a custom endpoint.
// Useful for private endpoints and unit tests.
func NewClientWithEndpoint(httpClient *http.Client, endpoint string) *Client {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: defaultCatalogTimeout}
	}
	if endpoint == "" {
		endpoint = DefaultCatalogEndpoint
	}
	return &Client{httpClient: httpClient, endpoint: endpoint}
}

// FetchBands performs a single HTTP GET to the catalog endpoint, validates the
// response, and returns the bands sorted ascending by Capacity.Min.
// A fresh HTTP request is made on every call.
func (c *Client) FetchBands(ctx context.Context) ([]Band, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create dp2 catalog request: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch dp2 catalog: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to fetch dp2 catalog: unexpected HTTP status %s", resp.Status)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxCatalogResponseSize+1))
	if err != nil {
		return nil, fmt.Errorf("failed to read dp2 catalog response: %w", err)
	}
	if len(body) > maxCatalogResponseSize {
		return nil, fmt.Errorf("dp2 catalog response exceeds %d bytes", maxCatalogResponseSize)
	}

	var entry catalogEntry
	if err := json.Unmarshal(body, &entry); err != nil {
		return nil, fmt.Errorf("failed to decode dp2 catalog response: %w", err)
	}

	bands := entry.Metadata.Other.Profile.ConfigValidation
	if len(bands) == 0 {
		return nil, fmt.Errorf("dp2 catalog response contains no capacity-to-IOPS validation bands")
	}
	for i, b := range bands {
		if b.Capacity.Min <= 0 || b.Capacity.Max < b.Capacity.Min {
			return nil, fmt.Errorf("dp2 catalog band %d contains an invalid capacity range", i)
		}
		if b.IOPS.Min <= 0 || b.IOPS.Max < b.IOPS.Min {
			return nil, fmt.Errorf("dp2 catalog band %d contains an invalid IOPS range", i)
		}
	}

	sort.Slice(bands, func(i, j int) bool {
		return bands[i].Capacity.Min < bands[j].Capacity.Min
	})
	return bands, nil
}

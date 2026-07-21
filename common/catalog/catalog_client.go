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

// Package catalog provides a client for the IBM Global Catalog API used to
// resolve dp2 capacity-to-IOPS validation bands for fixed-IOPS file shares.
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
	// DefaultCatalogEndpoint returns the dp2 entry together with the metadata
	// that contains its capacity-to-IOPS validation bands.
	DefaultCatalogEndpoint = "https://globalcatalog.cloud.ibm.com/api/v1/dp2?include=metadata.other"
	// 30-second fallback timeout
	defaultCatalogTimeout = 60 * time.Second
	// 4 MiB maximum response size
	maxCatalogResponseSize = 4 << 20
)

// BandRange is an inclusive [Min, Max] integer interval used inside a Band.
type BandRange struct {
	Min int64 `json:"min"`
	Max int64 `json:"max"`
}

// Band is one row of the dp2 capacity-to-IOPS validation table published by
// the IBM Global Catalog. A share whose IOPS falls in [IOPS.Min, IOPS.Max]
// must have at least Capacity.Min GiB of storage.
type Band struct {
	Capacity BandRange `json:"capacity"`
	IOPS     BandRange `json:"iops"`
}

// CapacityRoundoffService resolves and applies dp2 capacity-to-IOPS constraints.
type CapacityRoundoffService interface {
	FetchBands(ctx context.Context) ([]Band, error)
	GetMinimumCapacityForIOPS(ctx context.Context, requestedIOPS int64) (int64, error)
	RoundUpCapacityForIOPS(ctx context.Context, requestedCapacityGiB, requestedIOPS int64) (int64, error)
}

// CatalogClient fetches the dp2 validation bands from the IBM Global Catalog.
type CatalogClient struct {
	httpClient *http.Client
	endpoint   string
}

type catalogEntry struct {
	Metadata struct {
		Other struct {
			Profile struct {
				ConfigValidation []Band `json:"config_validation"`
			} `json:"profile"`
		} `json:"other"`
	} `json:"metadata"`
}

var _ CapacityRoundoffService = &CatalogClient{}

// FetchBands fetches the full dp2 capacity-to-IOPS band table from the IBM
func (c *CatalogClient) FetchBands(ctx context.Context) ([]Band, error) {
	return c.fetchBands(ctx)
}

// NewCatalogClient creates a client for the public IBM Global Catalog dp2
func NewCatalogClient(httpClient *http.Client) *CatalogClient {
	return NewCatalogClientWithEndpoint(httpClient, DefaultCatalogEndpoint)
}

// NewCatalogClientWithEndpoint creates a catalog client with a custom
// endpoint. It is useful for private endpoints and tests.
func NewCatalogClientWithEndpoint(httpClient *http.Client, endpoint string) *CatalogClient {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: defaultCatalogTimeout}
	}
	if endpoint == "" {
		endpoint = DefaultCatalogEndpoint
	}

	return &CatalogClient{
		httpClient: httpClient,
		endpoint:   endpoint,
	}
}

// GetMinimumCapacityForIOPS returns the smallest dp2 capacity, in GiB, whose
// catalog band supports the requested IOPS value.
// A fresh HTTP request is made on every call; wrap with CachingCatalogClient
// in the CSI driver to avoid repeated fetches.
func (c *CatalogClient) GetMinimumCapacityForIOPS(ctx context.Context, requestedIOPS int64) (int64, error) {
	if requestedIOPS <= 0 {
		return 0, fmt.Errorf("requested IOPS must be greater than zero: %d", requestedIOPS)
	}

	bands, err := c.FetchBands(ctx)
	if err != nil {
		return 0, err
	}

	return minimumCapacityFromBands(bands, requestedIOPS)
}

// RoundUpCapacityForIOPS returns the requested capacity unchanged when it is
// already valid, or the catalog-derived minimum capacity when it is too small.
func (c *CatalogClient) RoundUpCapacityForIOPS(ctx context.Context, requestedCapacityGiB, requestedIOPS int64) (int64, error) {
	if requestedCapacityGiB <= 0 {
		return 0, fmt.Errorf("requested capacity must be greater than zero: %d GiB", requestedCapacityGiB)
	}

	minimumCapacityGiB, err := c.GetMinimumCapacityForIOPS(ctx, requestedIOPS)
	if err != nil {
		return 0, err
	}
	if requestedCapacityGiB < minimumCapacityGiB {
		return minimumCapacityGiB, nil
	}
	return requestedCapacityGiB, nil
}

// minimumCapacityFromBands is a pure helper shared by CatalogClient and the
// driver's CachingCatalogClient to scan a sorted band slice.
func minimumCapacityFromBands(bands []Band, requestedIOPS int64) (int64, error) {
	for _, band := range bands {
		if requestedIOPS >= band.IOPS.Min && requestedIOPS <= band.IOPS.Max {
			return band.Capacity.Min, nil
		}
	}
	return 0, fmt.Errorf("no dp2 catalog band covers iops=%d", requestedIOPS)
}

func (c *CatalogClient) fetchBands(ctx context.Context) ([]Band, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create dp2 catalog request: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	response, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch dp2 catalog: %w", err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to fetch dp2 catalog: unexpected HTTP status %s", response.Status)
	}

	body, err := io.ReadAll(io.LimitReader(response.Body, maxCatalogResponseSize+1))
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
	for index, band := range bands {
		if band.Capacity.Min <= 0 || band.Capacity.Max < band.Capacity.Min {
			return nil, fmt.Errorf("dp2 catalog band %d contains an invalid capacity range", index)
		}
		if band.IOPS.Min <= 0 || band.IOPS.Max < band.IOPS.Min {
			return nil, fmt.Errorf("dp2 catalog band %d contains an invalid IOPS range", index)
		}
	}

	sort.Slice(bands, func(i, j int) bool {
		return bands[i].Capacity.Min < bands[j].Capacity.Min
	})
	return bands, nil
}

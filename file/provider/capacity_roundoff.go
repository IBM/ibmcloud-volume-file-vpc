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

// Package provider ...
package provider

import (
	"context"
	"fmt"
	"sync"

	"github.com/IBM/ibmcloud-volume-file-vpc/common/catalog"
	"go.uber.org/zap"
)

// CapacityRoundoffService is the interface the CSI driver uses to resolve the
// minimum valid capacity for a given IOPS value and to adjust a user-requested
// capacity upwards when it is too small for the requested IOPS.
//
// The interface is defined here (in the provider layer) so that the driver
// imports only this package and never reaches into common/catalog directly.
//
//go:generate counterfeiter -o fakes/capacity_roundoff_service.go --fake-name CapacityRoundoffService . CapacityRoundoffService
type CapacityRoundoffService interface {
	// RoundUpCapacityForIOPS returns requestedCapacityGiB unchanged when it is
	// already sufficient for requestedIOPS, or the catalog-derived minimum when
	// it is too small. Returns an error when requestedIOPS is outside all
	// catalog bands or the catalog is temporarily unavailable.
	RoundUpCapacityForIOPS(ctx context.Context, requestedCapacityGiB, requestedIOPS int64) (int64, error)
}

// capacityRoundoffService is the production implementation of
// CapacityRoundoffService. It wraps a catalog.Client and caches the full band
// table after the first successful HTTP fetch so that all subsequent calls
// scan in memory with no additional network I/O.
type capacityRoundoffService struct {
	client *catalog.Client
	logger *zap.Logger

	mu    sync.Mutex
	bands []catalog.Band // nil until first successful fetch
}

// NewCapacityRoundoffService creates a CapacityRoundoffService backed by the
// supplied catalog.Client. Pass nil for httpClient to use a default HTTP client
// with a 60-second timeout, and an empty endpoint to use the IBM production
// Global Catalog URL.
//
// The service is safe for concurrent use. Band data is fetched lazily on the
// first RoundUpCapacityForIOPS call and cached for the lifetime of the object.
func NewCapacityRoundoffService(client *catalog.Client, logger *zap.Logger) CapacityRoundoffService {
	return &capacityRoundoffService{
		client: client,
		logger: logger,
	}
}

// RoundUpCapacityForIOPS implements CapacityRoundoffService.
func (s *capacityRoundoffService) RoundUpCapacityForIOPS(ctx context.Context, requestedCapacityGiB, requestedIOPS int64) (int64, error) {
	if requestedCapacityGiB <= 0 {
		return 0, fmt.Errorf("requested capacity must be greater than zero: %d GiB", requestedCapacityGiB)
	}
	if requestedIOPS <= 0 {
		return 0, fmt.Errorf("requested IOPS must be greater than zero: %d", requestedIOPS)
	}

	bands, err := s.getBands(ctx)
	if err != nil {
		return 0, err
	}

	minCap, err := minimumCapacityForIOPS(bands, requestedIOPS)
	if err != nil {
		return 0, err
	}

	if requestedCapacityGiB < minCap {
		if s.logger != nil {
			s.logger.Info("RoundUpCapacityForIOPS: rounding up capacity to meet IOPS requirement",
				zap.Int64("requestedCapacityGiB", requestedCapacityGiB),
				zap.Int64("adjustedCapacityGiB", minCap),
				zap.Int64("requestedIOPS", requestedIOPS),
			)
		}
		return minCap, nil
	}
	return requestedCapacityGiB, nil
}

// getBands returns the cached band table, fetching from the catalog on the
// first call. Errors are not cached so transient outages self-heal.
func (s *capacityRoundoffService) getBands(ctx context.Context) ([]catalog.Band, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(s.bands) > 0 {
		return s.bands, nil
	}

	bands, err := s.client.FetchBands(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch dp2 capacity bands: %w", err)
	}

	s.bands = bands
	if s.logger != nil {
		s.logger.Info("RoundUpCapacityForIOPS: dp2 capacity bands cached", zap.Int("bandCount", len(bands)))
	}
	return s.bands, nil
}

// minimumCapacityForIOPS returns the Capacity.Min of the first band in bands
// whose IOPS range contains requestedIOPS.
func minimumCapacityForIOPS(bands []catalog.Band, requestedIOPS int64) (int64, error) {
	for _, b := range bands {
		if requestedIOPS >= b.IOPS.Min && requestedIOPS <= b.IOPS.Max {
			return b.Capacity.Min, nil
		}
	}
	return 0, fmt.Errorf("no dp2 catalog band covers iops=%d", requestedIOPS)
}

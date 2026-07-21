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
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/IBM/ibmcloud-volume-file-vpc/common/catalog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// catalogFixture matches the IBM Global Catalog JSON shape. Bands are
// deliberately out of order to verify that sorting inside FetchBands works.
//
//	Band 0 (after sort): capacity 10–39 GiB,    iops 100–1 000
//	Band 1:              capacity 40–79 GiB,    iops 100–2 000
//	Band 2:              capacity 80–99 GiB,    iops 100–4 000
//	Band 3:              capacity 100–499 GiB,  iops 100–6 000
//	Band 4:              capacity 500–999 GiB,  iops 100–10 000
//	Band 5:              capacity 1 000–1 999 GiB, iops 100–20 000
//	Band 6:              capacity 16 000–32 000 GiB, iops 2 000–96 000
const catalogFixture = `{
  "metadata": {
    "other": {
      "profile": {
        "config_validation": [
          {"capacity":{"min":100,"max":499},  "iops":{"min":100,"max":6000}},
          {"capacity":{"min":10,"max":39},    "iops":{"min":100,"max":1000}},
          {"capacity":{"min":40,"max":79},    "iops":{"min":100,"max":2000}},
          {"capacity":{"min":80,"max":99},    "iops":{"min":100,"max":4000}},
          {"capacity":{"min":500,"max":999},  "iops":{"min":100,"max":10000}},
          {"capacity":{"min":1000,"max":1999},"iops":{"min":100,"max":20000}},
          {"capacity":{"min":16000,"max":32000},"iops":{"min":2000,"max":96000}}
        ]
      }
    }
  }
}`

// newCatalogServer returns an httptest.Server and a catalog.Client pointed at
// it. The server always replies with the given status and body.
func newCatalogServer(t *testing.T, status int, body string) (*httptest.Server, *catalog.Client) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return srv, catalog.NewClientWithEndpoint(srv.Client(), srv.URL)
}

// newService is a convenience helper that builds a CapacityRoundoffService
// backed by a test catalog server.
func newService(t *testing.T, status int, body string) (CapacityRoundoffService, *httptest.Server) {
	t.Helper()
	logger, teardown := GetTestLogger(t)
	t.Cleanup(teardown)
	srv, client := newCatalogServer(t, status, body)
	return NewCapacityRoundoffService(client, logger), srv
}

// ---------------------------------------------------------------------------
// NewCapacityRoundoffService constructor
// ---------------------------------------------------------------------------

func TestNewCapacityRoundoffService_NotNil(t *testing.T) {
	logger, teardown := GetTestLogger(t)
	defer teardown()

	_, client := newCatalogServer(t, http.StatusOK, catalogFixture)
	svc := NewCapacityRoundoffService(client, logger)
	assert.NotNil(t, svc)
}

// ---------------------------------------------------------------------------
// RoundUpCapacityForIOPS — input validation
// ---------------------------------------------------------------------------

func TestRoundUpCapacityForIOPS_ZeroCapacityReturnsError(t *testing.T) {
	svc, _ := newService(t, http.StatusOK, catalogFixture)

	_, err := svc.RoundUpCapacityForIOPS(context.Background(), 0, 1000)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "requested capacity must be greater than zero")
}

func TestRoundUpCapacityForIOPS_NegativeCapacityReturnsError(t *testing.T) {
	svc, _ := newService(t, http.StatusOK, catalogFixture)

	_, err := svc.RoundUpCapacityForIOPS(context.Background(), -5, 1000)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "requested capacity must be greater than zero")
}

func TestRoundUpCapacityForIOPS_ZeroIOPSReturnsError(t *testing.T) {
	svc, _ := newService(t, http.StatusOK, catalogFixture)

	_, err := svc.RoundUpCapacityForIOPS(context.Background(), 100, 0)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "requested IOPS must be greater than zero")
}

// ---------------------------------------------------------------------------
// RoundUpCapacityForIOPS — round-up logic
// ---------------------------------------------------------------------------

func TestRoundUpCapacityForIOPS_CapacityAlreadySufficient(t *testing.T) {
	// Band: iops 100–1000 requires capacity >= 10 GiB. Requesting 20 GiB is fine.
	svc, _ := newService(t, http.StatusOK, catalogFixture)

	got, err := svc.RoundUpCapacityForIOPS(context.Background(), 20, 500)
	require.NoError(t, err)
	assert.Equal(t, int64(20), got, "capacity should not be changed when already sufficient")
}

func TestRoundUpCapacityForIOPS_CapacityExactlyAtMinimum(t *testing.T) {
	// Band: iops 100–1000 requires capacity >= 10 GiB. Requesting exactly 10 GiB.
	svc, _ := newService(t, http.StatusOK, catalogFixture)

	got, err := svc.RoundUpCapacityForIOPS(context.Background(), 10, 500)
	require.NoError(t, err)
	assert.Equal(t, int64(10), got, "capacity exactly at minimum should not be rounded up")
}

func TestRoundUpCapacityForIOPS_IOPSRequiresHigherCapacity(t *testing.T) {
	// iops=7000 falls into band capacity 500–999 (iops 100–10000, capacity.min=500).
	// Requesting 100 GiB should be rounded up to 500 GiB.
	svc, _ := newService(t, http.StatusOK, catalogFixture)

	got, err := svc.RoundUpCapacityForIOPS(context.Background(), 100, 7000)
	require.NoError(t, err)
	assert.Equal(t, int64(500), got, "capacity should be rounded up to band minimum")
}

func TestRoundUpCapacityForIOPS_IOPSAtBandBoundary_Min(t *testing.T) {
	// iops=100 (the minimum across all bands) matches the first band (cap.min=10).
	svc, _ := newService(t, http.StatusOK, catalogFixture)

	got, err := svc.RoundUpCapacityForIOPS(context.Background(), 5, 100)
	require.NoError(t, err)
	assert.Equal(t, int64(10), got, "capacity should be rounded up to 10 GiB (first band minimum)")
}

func TestRoundUpCapacityForIOPS_IOPSAtBandBoundary_Max(t *testing.T) {
	// iops=10000 (max of band capacity 500–999). Requesting 100 GiB → round up to 500 GiB.
	svc, _ := newService(t, http.StatusOK, catalogFixture)

	got, err := svc.RoundUpCapacityForIOPS(context.Background(), 100, 10000)
	require.NoError(t, err)
	assert.Equal(t, int64(500), got, "capacity should be rounded up to 500 GiB (band max boundary)")
}

func TestRoundUpCapacityForIOPS_HighIOPS_LargeCapacityBand(t *testing.T) {
	// iops=50000 falls in band capacity 16000–32000 (iops 2000–96000). cap.min=16000.
	svc, _ := newService(t, http.StatusOK, catalogFixture)

	got, err := svc.RoundUpCapacityForIOPS(context.Background(), 1000, 50000)
	require.NoError(t, err)
	assert.Equal(t, int64(16000), got, "capacity should be rounded up to 16000 GiB for high-IOPS band")
}

// ---------------------------------------------------------------------------
// RoundUpCapacityForIOPS — IOPS outside all bands
// ---------------------------------------------------------------------------

func TestRoundUpCapacityForIOPS_IOPSBelowAllBands(t *testing.T) {
	// iops=50 is below the minimum (100) of all bands.
	svc, _ := newService(t, http.StatusOK, catalogFixture)

	_, err := svc.RoundUpCapacityForIOPS(context.Background(), 100, 50)
	require.Error(t, err)
	assert.True(t, strings.Contains(err.Error(), "no dp2 catalog band covers iops=50"),
		"unexpected error: %v", err)
}

func TestRoundUpCapacityForIOPS_IOPSAboveAllBands(t *testing.T) {
	// iops=200000 is above the maximum (96000) of all bands.
	svc, _ := newService(t, http.StatusOK, catalogFixture)

	_, err := svc.RoundUpCapacityForIOPS(context.Background(), 100, 200000)
	require.Error(t, err)
	assert.True(t, strings.Contains(err.Error(), "no dp2 catalog band covers iops=200000"),
		"unexpected error: %v", err)
}

// ---------------------------------------------------------------------------
// RoundUpCapacityForIOPS — catalog fetch failure
// ---------------------------------------------------------------------------

func TestRoundUpCapacityForIOPS_CatalogFetchHTTPError(t *testing.T) {
	svc, _ := newService(t, http.StatusServiceUnavailable, "")

	_, err := svc.RoundUpCapacityForIOPS(context.Background(), 100, 1000)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to fetch dp2 capacity bands")
}

func TestRoundUpCapacityForIOPS_CatalogFetchMalformedJSON(t *testing.T) {
	svc, _ := newService(t, http.StatusOK, `{bad json}`)

	_, err := svc.RoundUpCapacityForIOPS(context.Background(), 100, 1000)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to fetch dp2 capacity bands")
}

// ---------------------------------------------------------------------------
// Caching — catalog is only fetched once
// ---------------------------------------------------------------------------

func TestRoundUpCapacityForIOPS_BandsCachedAfterFirstCall(t *testing.T) {
	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		callCount++
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(catalogFixture))
	}))
	defer srv.Close()

	logger, teardown := GetTestLogger(t)
	defer teardown()

	client := catalog.NewClientWithEndpoint(srv.Client(), srv.URL)
	svc := NewCapacityRoundoffService(client, logger)

	// Three consecutive calls — all should succeed and only one HTTP fetch occurs.
	for i := 0; i < 3; i++ {
		_, err := svc.RoundUpCapacityForIOPS(context.Background(), 100, 1000)
		require.NoError(t, err, "call %d failed", i)
	}

	assert.Equal(t, 1, callCount, "catalog HTTP endpoint should be called only once (bands are cached)")
}

func TestRoundUpCapacityForIOPS_CacheNotPopulatedOnFetchError(t *testing.T) {
	// First call → catalog returns 503 (fetch fails, bands NOT cached).
	// Second call → catalog returns 200 (should succeed because errors are not cached).
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		if calls == 1 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(catalogFixture))
	}))
	defer srv.Close()

	logger, teardown := GetTestLogger(t)
	defer teardown()

	client := catalog.NewClientWithEndpoint(srv.Client(), srv.URL)
	svc := NewCapacityRoundoffService(client, logger)

	// First call: should fail.
	_, err := svc.RoundUpCapacityForIOPS(context.Background(), 100, 1000)
	require.Error(t, err, "expected error on first call (503)")

	// Second call: should succeed (self-heal after transient error).
	got, err := svc.RoundUpCapacityForIOPS(context.Background(), 100, 1000)
	require.NoError(t, err, "expected success on second call")
	assert.Equal(t, int64(100), got)
}

// ---------------------------------------------------------------------------
// Concurrency — no data race under parallel calls
// ---------------------------------------------------------------------------

func TestRoundUpCapacityForIOPS_ConcurrentCallsNoPanic(t *testing.T) {
	svc, _ := newService(t, http.StatusOK, catalogFixture)

	const goroutines = 20
	var wg sync.WaitGroup
	wg.Add(goroutines)

	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			_, _ = svc.RoundUpCapacityForIOPS(context.Background(), 100, 1000)
		}()
	}
	wg.Wait()
}

// ---------------------------------------------------------------------------
// minimumCapacityForIOPS (package-internal unit tests)
// ---------------------------------------------------------------------------

func TestMinimumCapacityForIOPS_MatchesFirstContainingBand(t *testing.T) {
	bands := []catalog.Band{
		{Capacity: catalog.BandRange{Min: 10, Max: 39}, IOPS: catalog.BandRange{Min: 100, Max: 1000}},
		{Capacity: catalog.BandRange{Min: 40, Max: 79}, IOPS: catalog.BandRange{Min: 100, Max: 2000}},
	}

	// iops=1500 falls only in the second band.
	min, err := minimumCapacityForIOPS(bands, 1500)
	require.NoError(t, err)
	assert.Equal(t, int64(40), min)
}

func TestMinimumCapacityForIOPS_NoBandCoversIOPS(t *testing.T) {
	bands := []catalog.Band{
		{Capacity: catalog.BandRange{Min: 10, Max: 39}, IOPS: catalog.BandRange{Min: 100, Max: 1000}},
	}

	_, err := minimumCapacityForIOPS(bands, 99999)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no dp2 catalog band covers iops=99999")
}

func TestMinimumCapacityForIOPS_EmptyBands(t *testing.T) {
	_, err := minimumCapacityForIOPS(nil, 500)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no dp2 catalog band covers iops=500")
}

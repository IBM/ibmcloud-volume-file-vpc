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

package catalog

import (
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// dp2BandsJSON is a minimal representation of the IBM Global Catalog dp2 API
// response containing the config_validation bands used across tests.
const dp2BandsJSON = `{
  "metadata": {
    "other": {
      "profile": {
        "config_validation": [
          {"capacity": {"min": 10,    "max": 39,    "units": "gb"}, "iops": {"min": 100, "max": 1000,  "unit": "iops"}},
          {"capacity": {"min": 40,    "max": 79,    "units": "gb"}, "iops": {"min": 100, "max": 2000,  "unit": "iops"}},
          {"capacity": {"min": 80,    "max": 99,    "units": "gb"}, "iops": {"min": 100, "max": 4000,  "unit": "iops"}},
          {"capacity": {"min": 100,   "max": 499,   "units": "gb"}, "iops": {"min": 100, "max": 6000,  "unit": "iops"}},
          {"capacity": {"min": 500,   "max": 999,   "units": "gb"}, "iops": {"min": 100, "max": 10000, "unit": "iops"}},
          {"capacity": {"min": 1000,  "max": 1999,  "units": "gb"}, "iops": {"min": 100, "max": 20000, "unit": "iops"}},
          {"capacity": {"min": 2000,  "max": 3999,  "units": "gb"}, "iops": {"min": 200, "max": 40000, "unit": "iops"}},
          {"capacity": {"min": 4000,  "max": 7999,  "units": "gb"}, "iops": {"min": 300, "max": 40000, "unit": "iops"}},
          {"capacity": {"min": 8000,  "max": 15999, "units": "gb"}, "iops": {"min": 500, "max": 64000, "unit": "iops"}},
          {"capacity": {"min": 16000, "max": 32000, "units": "gb"}, "iops": {"min": 2000,"max": 96000, "unit": "iops"}}
        ]
      }
    }
  }
}`

// fakeHTTPClient is an HTTPDoer that returns the configured response without
// making any actual network calls.
type fakeHTTPClient struct {
	statusCode int
	body       string
	err        error
}

func (f *fakeHTTPClient) Do(_ *http.Request) (*http.Response, error) {
	if f.err != nil {
		return nil, f.err
	}
	return &http.Response{
		StatusCode: f.statusCode,
		Body:       io.NopCloser(strings.NewReader(f.body)),
	}, nil
}

// expectedBands mirrors what FetchCatalogBandsDP2 should return for dp2BandsJSON.
var expectedBands = []CatalogBand{
	{CapMin: 10, CapMax: 39, IOPSMin: 100, IOPSMax: 1000},
	{CapMin: 40, CapMax: 79, IOPSMin: 100, IOPSMax: 2000},
	{CapMin: 80, CapMax: 99, IOPSMin: 100, IOPSMax: 4000},
	{CapMin: 100, CapMax: 499, IOPSMin: 100, IOPSMax: 6000},
	{CapMin: 500, CapMax: 999, IOPSMin: 100, IOPSMax: 10000},
	{CapMin: 1000, CapMax: 1999, IOPSMin: 100, IOPSMax: 20000},
	{CapMin: 2000, CapMax: 3999, IOPSMin: 200, IOPSMax: 40000},
	{CapMin: 4000, CapMax: 7999, IOPSMin: 300, IOPSMax: 40000},
	{CapMin: 8000, CapMax: 15999, IOPSMin: 500, IOPSMax: 64000},
	{CapMin: 16000, CapMax: 32000, IOPSMin: 2000, IOPSMax: 96000},
}

// ---- FetchCatalogBandsDP2 ----------------------------------------------------

func TestFetchCatalogBandsDP2_Success(t *testing.T) {
	client := NewCatalogClientWithURL(&fakeHTTPClient{
		statusCode: http.StatusOK,
		body:       dp2BandsJSON,
	}, "http://fake")

	bands, err := client.FetchCatalogBandsDP2()
	require.NoError(t, err)
	require.Len(t, bands, 10)
	assert.Equal(t, CatalogBand{CapMin: 10, CapMax: 39, IOPSMin: 100, IOPSMax: 1000}, bands[0])
	assert.Equal(t, CatalogBand{CapMin: 16000, CapMax: 32000, IOPSMin: 2000, IOPSMax: 96000}, bands[9])
}

func TestFetchCatalogBandsDP2_ParsesAllBands(t *testing.T) {
	client := NewCatalogClientWithURL(&fakeHTTPClient{
		statusCode: http.StatusOK,
		body:       dp2BandsJSON,
	}, "http://fake")

	bands, err := client.FetchCatalogBandsDP2()
	require.NoError(t, err)
	require.Equal(t, expectedBands, bands)
}

func TestFetchCatalogBandsDP2_HTTPError(t *testing.T) {
	client := NewCatalogClientWithURL(&fakeHTTPClient{err: io.ErrUnexpectedEOF}, "http://fake")
	_, err := client.FetchCatalogBandsDP2()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "HTTP request")
}

func TestFetchCatalogBandsDP2_Non2xxStatus(t *testing.T) {
	client := NewCatalogClientWithURL(&fakeHTTPClient{
		statusCode: http.StatusServiceUnavailable,
		body:       `{"error":"unavailable"}`,
	}, "http://fake")
	_, err := client.FetchCatalogBandsDP2()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "503")
}

func TestFetchCatalogBandsDP2_InvalidJSON(t *testing.T) {
	client := NewCatalogClientWithURL(&fakeHTTPClient{
		statusCode: http.StatusOK,
		body:       `not-json`,
	}, "http://fake")
	_, err := client.FetchCatalogBandsDP2()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "decode response")
}

func TestFetchCatalogBandsDP2_EmptyBands(t *testing.T) {
	client := NewCatalogClientWithURL(&fakeHTTPClient{
		statusCode: http.StatusOK,
		body:       `{"metadata":{"other":{"profile":{"config_validation":[]}}}}`,
	}, "http://fake")
	_, err := client.FetchCatalogBandsDP2()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no config_validation bands")
}

func TestFetchCatalogBandsDP2_MalformedEntry(t *testing.T) {
	// An entry with capacity.min=0 must be rejected.
	const malformedJSON = `{
  "metadata":{"other":{"profile":{"config_validation":[
    {"capacity":{"min":0,"max":39,"units":"gb"},"iops":{"min":100,"max":1000,"unit":"iops"}}
  ]}}}}`
	client := NewCatalogClientWithURL(&fakeHTTPClient{
		statusCode: http.StatusOK,
		body:       malformedJSON,
	}, "http://fake")
	_, err := client.FetchCatalogBandsDP2()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid config_validation entry")
}

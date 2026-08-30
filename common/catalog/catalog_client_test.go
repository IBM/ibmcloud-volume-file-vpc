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
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// dp2CatalogJSON is a minimal armada-storage-api GET /v2/storage/vpc/volumeProfile?profile=dp2
// response in the config_validation format.
const dp2CatalogJSON = `{
  "id": "dp2",
  "config_validation": [
    {"capacity": {"min": 10,    "max": 39},    "iops": {"min": 100,  "max": 1000}},
    {"capacity": {"min": 40,    "max": 79},    "iops": {"min": 100,  "max": 2000}},
    {"capacity": {"min": 80,    "max": 99},    "iops": {"min": 100,  "max": 4000}},
    {"capacity": {"min": 100,   "max": 499},   "iops": {"min": 100,  "max": 6000}},
    {"capacity": {"min": 500,   "max": 999},   "iops": {"min": 100,  "max": 10000}},
    {"capacity": {"min": 1000,  "max": 1999},  "iops": {"min": 100,  "max": 20000}},
    {"capacity": {"min": 2000,  "max": 3999},  "iops": {"min": 200,  "max": 40000}},
    {"capacity": {"min": 4000,  "max": 7999},  "iops": {"min": 300,  "max": 40000}},
    {"capacity": {"min": 8000,  "max": 15999}, "iops": {"min": 500,  "max": 64000}},
    {"capacity": {"min": 16000, "max": 32000}, "iops": {"min": 2000, "max": 96000}}
  ]
}`

// fakeHTTPClient is an HTTPDoer that returns the configured response without
// making any actual network calls.
type fakeHTTPClient struct {
	statusCode  int
	body        string
	err         error
	capturedURL string
}

func (f *fakeHTTPClient) Do(req *http.Request) (*http.Response, error) {
	f.capturedURL = req.URL.String()
	if f.err != nil {
		return nil, f.err
	}
	return &http.Response{
		StatusCode: f.statusCode,
		Body:       io.NopCloser(strings.NewReader(f.body)),
	}, nil
}

// expectedDP2Bands mirrors what FetchVolumeProfileBands("dp2") should return for dp2CatalogJSON.
var expectedDP2Bands = []CatalogBand{
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

// ---- URL construction --------------------------------------------------------

func TestFetchVolumeProfileBands_URLAppendsProfileParam(t *testing.T) {
	// IKSTokenExchangePrivateURL is a bare host — the client must assemble
	// the full /v2/storage/vpc/volumeProfile?profile=dp2 path itself.
	fake := &fakeHTTPClient{statusCode: http.StatusOK, body: dp2CatalogJSON}
	client := NewCatalogClient(fake, "https://us-south.containers.cloud.ibm.com")
	_, err := client.FetchVolumeProfileBands(context.Background(), "dp2")
	require.NoError(t, err)
	assert.Equal(t, "https://us-south.containers.cloud.ibm.com/v2/storage/vpc/volumeProfile?profile=dp2", fake.capturedURL)
}

func TestFetchVolumeProfileBands_TrailingSlashStripped(t *testing.T) {
	fake := &fakeHTTPClient{statusCode: http.StatusOK, body: dp2CatalogJSON}
	// Base URL has a trailing slash; it must be stripped before appending the path.
	client := NewCatalogClient(fake, "https://us-south.containers.cloud.ibm.com/")
	_, err := client.FetchVolumeProfileBands(context.Background(), "dp2")
	require.NoError(t, err)
	assert.Equal(t, "https://us-south.containers.cloud.ibm.com/v2/storage/vpc/volumeProfile?profile=dp2", fake.capturedURL)
}

func TestFetchVolumeProfileBands_ProfileNameInURL(t *testing.T) {
	// Confirm the profile name is correctly appended for a different profile.
	fake := &fakeHTTPClient{statusCode: http.StatusOK, body: `{"id":"rfs","config_validation":[{"capacity":{"min":1,"max":32000},"iops":{"min":100,"max":48000}}]}`}
	client := NewCatalogClient(fake, "https://us-south.containers.cloud.ibm.com")
	_, err := client.FetchVolumeProfileBands(context.Background(), "rfs")
	require.NoError(t, err)
	assert.Equal(t, "https://us-south.containers.cloud.ibm.com/v2/storage/vpc/volumeProfile?profile=rfs", fake.capturedURL)
}

// ---- Success -----------------------------------------------------------------

func TestFetchVolumeProfileBands_Success(t *testing.T) {
	client := NewCatalogClient(&fakeHTTPClient{
		statusCode: http.StatusOK,
		body:       dp2CatalogJSON,
	}, "http://fake")

	bands, err := client.FetchVolumeProfileBands(context.Background(), "dp2")
	require.NoError(t, err)
	require.Len(t, bands, 10)
	assert.Equal(t, CatalogBand{CapMin: 10, CapMax: 39, IOPSMin: 100, IOPSMax: 1000}, bands[0])
	assert.Equal(t, CatalogBand{CapMin: 16000, CapMax: 32000, IOPSMin: 2000, IOPSMax: 96000}, bands[9])
}

func TestFetchVolumeProfileBands_ParsesAllBands(t *testing.T) {
	client := NewCatalogClient(&fakeHTTPClient{
		statusCode: http.StatusOK,
		body:       dp2CatalogJSON,
	}, "http://fake")

	bands, err := client.FetchVolumeProfileBands(context.Background(), "dp2")
	require.NoError(t, err)
	require.Equal(t, expectedDP2Bands, bands)
}

// ---- Error paths -------------------------------------------------------------

func TestFetchVolumeProfileBands_HTTPError(t *testing.T) {
	client := NewCatalogClient(&fakeHTTPClient{err: io.ErrUnexpectedEOF}, "http://fake")
	_, err := client.FetchVolumeProfileBands(context.Background(), "dp2")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "HTTP request")
}

func TestFetchVolumeProfileBands_Non2xxStatus(t *testing.T) {
	client := NewCatalogClient(&fakeHTTPClient{
		statusCode: http.StatusServiceUnavailable,
		body:       `{"error":"unavailable"}`,
	}, "http://fake")
	_, err := client.FetchVolumeProfileBands(context.Background(), "dp2")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "503")
}

func TestFetchVolumeProfileBands_InvalidJSON(t *testing.T) {
	client := NewCatalogClient(&fakeHTTPClient{
		statusCode: http.StatusOK,
		body:       `not-json`,
	}, "http://fake")
	_, err := client.FetchVolumeProfileBands(context.Background(), "dp2")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "decode response")
}

func TestFetchVolumeProfileBands_EmptyBands(t *testing.T) {
	client := NewCatalogClient(&fakeHTTPClient{
		statusCode: http.StatusOK,
		body:       `{"id":"dp2","config_validation":[]}`,
	}, "http://fake")
	_, err := client.FetchVolumeProfileBands(context.Background(), "dp2")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no bands")
}

func TestFetchVolumeProfileBands_NullBands(t *testing.T) {
	client := NewCatalogClient(&fakeHTTPClient{
		statusCode: http.StatusOK,
		body:       `{}`,
	}, "http://fake")
	_, err := client.FetchVolumeProfileBands(context.Background(), "dp2")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no bands")
}

func TestFetchVolumeProfileBands_EntriesWithoutIopsSkipped(t *testing.T) {
	// Entries without an "iops" field are skipped; if ALL entries lack iops
	// the function must return an error rather than an empty slice.
	client := NewCatalogClient(&fakeHTTPClient{
		statusCode: http.StatusOK,
		body:       `{"id":"rfs","config_validation":[{"capacity":{"min":1,"max":32000}}]}`,
	}, "http://fake")
	_, err := client.FetchVolumeProfileBands(context.Background(), "rfs")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no bands")
}

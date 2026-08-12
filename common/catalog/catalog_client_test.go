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

// armadaCatalogJSON is a minimal armada-storage-api GET /v2/storage/vpc/getVolumeProfiles/dp2
// response containing the bands used across tests.
const armadaCatalogJSON = `{
  "bands": [
    {"capacityMin": 10,    "capacityMax": 39,    "iopsMin": 100,  "iopsMax": 1000},
    {"capacityMin": 40,    "capacityMax": 79,    "iopsMin": 100,  "iopsMax": 2000},
    {"capacityMin": 80,    "capacityMax": 99,    "iopsMin": 100,  "iopsMax": 4000},
    {"capacityMin": 100,   "capacityMax": 499,   "iopsMin": 100,  "iopsMax": 6000},
    {"capacityMin": 500,   "capacityMax": 999,   "iopsMin": 100,  "iopsMax": 10000},
    {"capacityMin": 1000,  "capacityMax": 1999,  "iopsMin": 100,  "iopsMax": 20000},
    {"capacityMin": 2000,  "capacityMax": 3999,  "iopsMin": 200,  "iopsMax": 40000},
    {"capacityMin": 4000,  "capacityMax": 7999,  "iopsMin": 300,  "iopsMax": 40000},
    {"capacityMin": 8000,  "capacityMax": 15999, "iopsMin": 500,  "iopsMax": 64000},
    {"capacityMin": 16000, "capacityMax": 32000, "iopsMin": 2000, "iopsMax": 96000}
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

// expectedBands mirrors what FetchCatalogBandsDP2 should return for armadaCatalogJSON.
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

// ---- URL construction --------------------------------------------------------

func TestFetchCatalogBandsDP2_URLAppendsCatalogPath(t *testing.T) {
	fake := &fakeHTTPClient{statusCode: http.StatusOK, body: armadaCatalogJSON}
	client := NewCatalogClientWithBaseURL(fake, "https://us-south.containers.cloud.ibm.com/v2/storage")
	_, err := client.FetchCatalogBandsDP2()
	require.NoError(t, err)
	assert.Equal(t, "https://us-south.containers.cloud.ibm.com/v2/storage/vpc/getVolumeProfiles/dp2", fake.capturedURL)
}

func TestFetchCatalogBandsDP2_TrailingSlashStripped(t *testing.T) {
	fake := &fakeHTTPClient{statusCode: http.StatusOK, body: armadaCatalogJSON}
	// Base URL has a trailing slash; it must be stripped before appending the path.
	client := NewCatalogClientWithBaseURL(fake, "https://us-south.containers.cloud.ibm.com/v2/storage/")
	_, err := client.FetchCatalogBandsDP2()
	require.NoError(t, err)
	assert.Equal(t, "https://us-south.containers.cloud.ibm.com/v2/storage/vpc/getVolumeProfiles/dp2", fake.capturedURL)
}

// ---- Success -----------------------------------------------------------------

func TestFetchCatalogBandsDP2_Success(t *testing.T) {
	client := NewCatalogClientWithBaseURL(&fakeHTTPClient{
		statusCode: http.StatusOK,
		body:       armadaCatalogJSON,
	}, "http://fake/v2/storage")

	bands, err := client.FetchCatalogBandsDP2()
	require.NoError(t, err)
	require.Len(t, bands, 10)
	assert.Equal(t, CatalogBand{CapMin: 10, CapMax: 39, IOPSMin: 100, IOPSMax: 1000}, bands[0])
	assert.Equal(t, CatalogBand{CapMin: 16000, CapMax: 32000, IOPSMin: 2000, IOPSMax: 96000}, bands[9])
}

func TestFetchCatalogBandsDP2_ParsesAllBands(t *testing.T) {
	client := NewCatalogClientWithBaseURL(&fakeHTTPClient{
		statusCode: http.StatusOK,
		body:       armadaCatalogJSON,
	}, "http://fake/v2/storage")

	bands, err := client.FetchCatalogBandsDP2()
	require.NoError(t, err)
	require.Equal(t, expectedBands, bands)
}

// ---- Error paths -------------------------------------------------------------

func TestFetchCatalogBandsDP2_HTTPError(t *testing.T) {
	client := NewCatalogClientWithBaseURL(&fakeHTTPClient{err: io.ErrUnexpectedEOF}, "http://fake/v2/storage")
	_, err := client.FetchCatalogBandsDP2()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "HTTP request")
}

func TestFetchCatalogBandsDP2_Non2xxStatus(t *testing.T) {
	client := NewCatalogClientWithBaseURL(&fakeHTTPClient{
		statusCode: http.StatusServiceUnavailable,
		body:       `{"error":"unavailable"}`,
	}, "http://fake/v2/storage")
	_, err := client.FetchCatalogBandsDP2()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "503")
}

func TestFetchCatalogBandsDP2_InvalidJSON(t *testing.T) {
	client := NewCatalogClientWithBaseURL(&fakeHTTPClient{
		statusCode: http.StatusOK,
		body:       `not-json`,
	}, "http://fake/v2/storage")
	_, err := client.FetchCatalogBandsDP2()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "decode response")
}

func TestFetchCatalogBandsDP2_EmptyBands(t *testing.T) {
	client := NewCatalogClientWithBaseURL(&fakeHTTPClient{
		statusCode: http.StatusOK,
		body:       `{"bands":[]}`,
	}, "http://fake/v2/storage")
	_, err := client.FetchCatalogBandsDP2()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no bands")
}

func TestFetchCatalogBandsDP2_NullBands(t *testing.T) {
	client := NewCatalogClientWithBaseURL(&fakeHTTPClient{
		statusCode: http.StatusOK,
		body:       `{}`,
	}, "http://fake/v2/storage")
	_, err := client.FetchCatalogBandsDP2()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no bands")
}

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

package catalog

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// catalogResponse is a representative dp2 band fixture in Global Catalog JSON format.
const catalogResponse = `{
  "metadata": {
    "other": {
      "profile": {
        "config_validation": [
          {"capacity":{"min":100,"max":499},"iops":{"min":100,"max":6000}},
          {"capacity":{"min":10,"max":39}, "iops":{"min":100,"max":1000}},
          {"capacity":{"min":40,"max":79}, "iops":{"min":100,"max":2000}},
          {"capacity":{"min":80,"max":99}, "iops":{"min":100,"max":4000}},
          {"capacity":{"min":500,"max":999},"iops":{"min":100,"max":10000}},
          {"capacity":{"min":1000,"max":1999},"iops":{"min":100,"max":20000}},
          {"capacity":{"min":16000,"max":32000},"iops":{"min":2000,"max":96000}}
        ]
      }
    }
  }
}`

// newTestServer returns an httptest.Server that always responds with body and
// a Client pointed at it.
func newTestServer(t *testing.T, status int, body string) (*httptest.Server, *Client) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, "application/json", r.Header.Get("Accept"))
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return srv, NewClientWithEndpoint(srv.Client(), srv.URL)
}

// ---------------------------------------------------------------------------
// FetchBands — happy path
// ---------------------------------------------------------------------------

func TestFetchBands_ReturnsSortedBands(t *testing.T) {
	_, client := newTestServer(t, http.StatusOK, catalogResponse)

	bands, err := client.FetchBands(context.Background())
	require.NoError(t, err)
	require.Len(t, bands, 7, "fixture has 7 bands")

	for i := 1; i < len(bands); i++ {
		assert.LessOrEqual(t, bands[i-1].Capacity.Min, bands[i].Capacity.Min,
			"bands must be sorted ascending by Capacity.Min")
	}
}

func TestFetchBands_SpotCheckValues(t *testing.T) {
	_, client := newTestServer(t, http.StatusOK, catalogResponse)

	bands, err := client.FetchBands(context.Background())
	require.NoError(t, err)

	assert.Equal(t, int64(10), bands[0].Capacity.Min, "smallest band starts at 10 GiB")
	assert.Equal(t, int64(32000), bands[len(bands)-1].Capacity.Max, "largest band ends at 32000 GiB")
}

func TestFetchBands_SetsAcceptHeader(t *testing.T) {
	var capturedAccept string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedAccept = r.Header.Get("Accept")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(catalogResponse))
	}))
	defer srv.Close()

	_, err := NewClientWithEndpoint(srv.Client(), srv.URL).FetchBands(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "application/json", capturedAccept)
}

// ---------------------------------------------------------------------------
// FetchBands — error cases
// ---------------------------------------------------------------------------

func TestFetchBands_HTTPError(t *testing.T) {
	_, client := newTestServer(t, http.StatusServiceUnavailable, "")

	_, err := client.FetchBands(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unexpected HTTP status 503 Service Unavailable")
}

func TestFetchBands_MalformedJSON(t *testing.T) {
	_, client := newTestServer(t, http.StatusOK, `{`)

	_, err := client.FetchBands(context.Background())
	require.Error(t, err)
	assert.True(t, strings.Contains(err.Error(), "failed to decode dp2 catalog response"),
		"unexpected error: %v", err)
}

func TestFetchBands_EmptyBands(t *testing.T) {
	_, client := newTestServer(t, http.StatusOK, `{}`)

	_, err := client.FetchBands(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "contains no capacity-to-IOPS validation bands")
}

func TestFetchBands_InvalidCapacityRange(t *testing.T) {
	body := `{"metadata":{"other":{"profile":{"config_validation":[
		{"capacity":{"min":100,"max":10},"iops":{"min":100,"max":1000}}
	]}}}}`
	_, client := newTestServer(t, http.StatusOK, body)

	_, err := client.FetchBands(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "contains an invalid capacity range")
}

func TestFetchBands_InvalidIOPSRange(t *testing.T) {
	body := `{"metadata":{"other":{"profile":{"config_validation":[
		{"capacity":{"min":10,"max":39},"iops":{"min":1000,"max":100}}
	]}}}}`
	_, client := newTestServer(t, http.StatusOK, body)

	_, err := client.FetchBands(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "contains an invalid IOPS range")
}

// ---------------------------------------------------------------------------
// NewClient / NewClientWithEndpoint
// ---------------------------------------------------------------------------

func TestNewClient_NilHTTPClient(t *testing.T) {
	c := NewClient(nil)
	assert.NotNil(t, c)
	assert.Equal(t, DefaultCatalogEndpoint, c.endpoint)
}

func TestNewClientWithEndpoint_EmptyEndpointFallsBackToDefault(t *testing.T) {
	c := NewClientWithEndpoint(nil, "")
	assert.Equal(t, DefaultCatalogEndpoint, c.endpoint)
}

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

package client

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const catalogResponse = `{
  "metadata": {
    "other": {
      "profile": {
        "config_validation": [
          {"capacity":{"min":100,"max":499},"iops":{"min":100,"max":6000}},
          {"capacity":{"min":10,"max":39},"iops":{"min":100,"max":1000}},
          {"capacity":{"min":40,"max":79},"iops":{"min":100,"max":2000}},
          {"capacity":{"min":80,"max":99},"iops":{"min":100,"max":4000}},
          {"capacity":{"min":500,"max":999},"iops":{"min":100,"max":10000}},
          {"capacity":{"min":1000,"max":1999},"iops":{"min":100,"max":20000}},
          {"capacity":{"min":16000,"max":32000},"iops":{"min":2000,"max":96000}}
        ]
      }
    }
  }
}`

func TestCatalogClientGetMinimumCapacityForIOPS(t *testing.T) {
	var requestCount atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		requestCount.Add(1)
		assert.Equal(t, http.MethodGet, request.Method)
		assert.Equal(t, "application/json", request.Header.Get("Accept"))
		response.Header().Set("Content-Type", "application/json")
		_, _ = response.Write([]byte(catalogResponse))
	}))
	defer server.Close()

	client := NewCatalogClientWithEndpoint(server.Client(), server.URL)
	testCases := []struct {
		name             string
		requestedIOPS    int64
		expectedCapacity int64
		expectedError    string
	}{
		{
			name:             "first band",
			requestedIOPS:    100,
			expectedCapacity: 10,
		}, {
			name:             "first band maximum",
			requestedIOPS:    1000,
			expectedCapacity: 10,
		}, {
			name:             "second band",
			requestedIOPS:    1001,
			expectedCapacity: 40,
		}, {
			name:             "3000 IOPS",
			requestedIOPS:    3000,
			expectedCapacity: 80,
		}, {
			name:             "above 6000 IOPS",
			requestedIOPS:    6001,
			expectedCapacity: 500,
		}, {
			name:             "maximum supported IOPS",
			requestedIOPS:    96000,
			expectedCapacity: 16000,
		}, {
			name:          "below catalog minimum",
			requestedIOPS: 99,
			expectedError: "no dp2 catalog band covers iops=99",
		}, {
			name:          "above catalog maximum",
			requestedIOPS: 999999,
			expectedError: "no dp2 catalog band covers iops=999999",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			capacity, err := client.GetMinimumCapacityForIOPS(context.Background(), testCase.requestedIOPS)
			if testCase.expectedError != "" {
				require.EqualError(t, err, testCase.expectedError)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, testCase.expectedCapacity, capacity)
		})
	}

	assert.Equal(t, int32(1), requestCount.Load(), "the successful catalog response must be cached")
}

func TestCatalogClientRoundUpCapacityForIOPS(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		response.Header().Set("Content-Type", "application/json")
		_, _ = response.Write([]byte(catalogResponse))
	}))
	defer server.Close()

	client := NewCatalogClientWithEndpoint(server.Client(), server.URL)
	testCases := []struct {
		name             string
		requestedGiB     int64
		requestedIOPS    int64
		expectedCapacity int64
	}{
		{
			name:             "round up below minimum",
			requestedGiB:     20,
			requestedIOPS:    3000,
			expectedCapacity: 80,
		}, {
			name:             "keep exact minimum",
			requestedGiB:     80,
			requestedIOPS:    3000,
			expectedCapacity: 80,
		}, {
			name:             "keep capacity above minimum",
			requestedGiB:     200,
			requestedIOPS:    3000,
			expectedCapacity: 200,
		}, {
			name:             "round up high IOPS",
			requestedGiB:     50,
			requestedIOPS:    20000,
			expectedCapacity: 1000,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			capacity, err := client.RoundUpCapacityForIOPS(context.Background(), testCase.requestedGiB, testCase.requestedIOPS)
			require.NoError(t, err)
			assert.Equal(t, testCase.expectedCapacity, capacity)
		})
	}
}

func TestCatalogClientErrors(t *testing.T) {
	testCases := []struct {
		name          string
		status        int
		body          string
		expectedError string
	}{
		{
			name:          "catalog unavailable",
			status:        http.StatusServiceUnavailable,
			expectedError: "unexpected HTTP status 503 Service Unavailable",
		}, {
			name:          "malformed response",
			status:        http.StatusOK,
			body:          `{`,
			expectedError: "failed to decode dp2 catalog response",
		}, {
			name:          "empty bands",
			status:        http.StatusOK,
			body:          `{}`,
			expectedError: "contains no capacity-to-IOPS validation bands",
		}, {
			name:          "invalid band",
			status:        http.StatusOK,
			body:          `{"metadata":{"other":{"profile":{"config_validation":[{"capacity":{"min":100,"max":10},"iops":{"min":100,"max":1000}}]}}}}`,
			expectedError: "contains an invalid capacity range",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
				response.WriteHeader(testCase.status)
				_, _ = response.Write([]byte(testCase.body))
			}))
			defer server.Close()

			client := NewCatalogClientWithEndpoint(server.Client(), server.URL)
			_, err := client.GetMinimumCapacityForIOPS(context.Background(), 1000)
			require.Error(t, err)
			assert.True(t, strings.Contains(err.Error(), testCase.expectedError), "unexpected error: %v", err)
		})
	}
}

func TestCatalogClientRejectsInvalidInputsBeforeFetching(t *testing.T) {
	var requestCount atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		requestCount.Add(1)
	}))
	defer server.Close()

	client := NewCatalogClientWithEndpoint(server.Client(), server.URL)

	_, err := client.GetMinimumCapacityForIOPS(context.Background(), 0)
	require.EqualError(t, err, "requested IOPS must be greater than zero: 0")
	_, err = client.RoundUpCapacityForIOPS(context.Background(), 0, 1000)
	require.EqualError(t, err, "requested capacity must be greater than zero: 0 GiB")
	assert.Equal(t, int32(0), requestCount.Load())
}

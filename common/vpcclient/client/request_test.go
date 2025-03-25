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

package client

import (
	"net/http"
	"net/url"
	"testing"

	"github.com/IBM/ibmcloud-volume-file-vpc/common/vpcclient/client/payload"
	"github.com/stretchr/testify/assert"
)

func TestRequest_path(t *testing.T) {
	tests := []struct {
		name       string
		operation  *Operation
		pathParams map[string]string
		expected   string
	}{
		{
			name: "simple path",
			operation: &Operation{
				Name:        "test",
				Method:      http.MethodGet,
				PathPattern: "/test",
			},
			pathParams: map[string]string{},
			expected:   "/test",
		},
		{
			name: "path with params",
			operation: &Operation{
				Name:        "test",
				Method:      http.MethodGet,
				PathPattern: "/test/{id}",
			},
			pathParams: map[string]string{"id": "123"},
			expected:   "/test/123",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &Request{
				operation:  tt.operation,
				pathParams: tt.pathParams,
			}
			assert.Equal(t, tt.expected, r.path())
		})
	}
}

func TestRequest_URL(t *testing.T) {
	tests := []struct {
		name        string
		baseURL     string
		operation   *Operation
		pathParams  map[string]string
		queryValues url.Values
		expected    string
	}{
		{
			name:    "simple URL",
			baseURL: "https://example.com",
			operation: &Operation{
				Name:        "test",
				Method:      http.MethodGet,
				PathPattern: "/test",
			},
			pathParams:  map[string]string{},
			queryValues: url.Values{},
			expected:    "https://example.com/test",
		},
		{
			name:    "URL with params and query",
			baseURL: "https://example.com",
			operation: &Operation{
				Name:        "test",
				Method:      http.MethodGet,
				PathPattern: "/test/{id}",
			},
			pathParams:  map[string]string{"id": "123"},
			queryValues: url.Values{"param1": []string{"value1"}},
			expected:    "https://example.com/test/123?param1=value1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &Request{
				baseURL:     tt.baseURL,
				operation:   tt.operation,
				pathParams:  tt.pathParams,
				queryValues: tt.queryValues,
			}
			assert.Equal(t, tt.expected, r.URL())
		})
	}
}

func TestRequest_PathParameter(t *testing.T) {
	tests := []struct {
		name       string
		operation  *Operation
		pathParams map[string]string
		key        string
		value      string
		expected   map[string]string
	}{
		{
			name: "add path parameter",
			operation: &Operation{
				Name:        "test",
				Method:      http.MethodGet,
				PathPattern: "/test/{id}",
			},
			pathParams: map[string]string{},
			key:        "id",
			value:      "123",
			expected:   map[string]string{"id": "123"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &Request{
				operation:  tt.operation,
				pathParams: tt.pathParams,
			}
			r.PathParameter(tt.key, tt.value)
			assert.Equal(t, tt.expected[tt.key], r.pathParams[tt.key])
		})
	}
}

func TestRequest_AddQueryValue(t *testing.T) {
	tests := []struct {
		name        string
		operation   *Operation
		queryValues url.Values
		key         string
		value       string
		expected    url.Values
	}{
		{
			name:        "add query value",
			queryValues: url.Values{},
			key:         "param1",
			value:       "value1",
			expected:    url.Values{"param1": []string{"value1"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &Request{
				operation: tt.operation,
			}
			r.AddQueryValue(tt.key, tt.value)
			assert.Equal(t, tt.expected, r.queryValues)
		})
	}
}

func TestRequest_SetQueryValue(t *testing.T) {
	tests := []struct {
		name        string
		operation   *Operation
		queryValues url.Values
		key         string
		value       string
		expected    url.Values
	}{
		{
			name: "set query value",
			queryValues: url.Values{
				"param1": []string{"value1"},
			},
			key:      "param1",
			value:    "value2",
			expected: url.Values{"param1": []string{"value2"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &Request{
				operation: tt.operation,
			}
			r.SetQueryValue(tt.key, tt.value)
			assert.Equal(t, tt.expected, r.queryValues)
		})
	}
}

func TestRequest_SetHeader(t *testing.T) {
	tests := []struct {
		name      string
		operation *Operation
		headers   http.Header
		key       string
		value     string
		expected  http.Header
	}{
		{
			name:     "set header",
			key:      "X-Custom-Header",
			value:    "custom-value",
			expected: http.Header{"X-Custom-Header": []string{"custom-value"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &Request{
				operation: tt.operation,
				headers:   http.Header{},
			}
			r.SetHeader(tt.key, tt.value)
			assert.Equal(t, tt.expected, r.headers)
		})
	}
}

func TestRequest_JSONBody(t *testing.T) {
	tests := []struct {
		name      string
		operation *Operation
		p         interface{}
		expected  *payload.JSONBodyProvider
	}{
		{
			name: "set JSON body",
			operation: &Operation{
				Name:        "test",
				Method:      http.MethodPost,
				PathPattern: "/test",
			},
			p: &TestStruct{
				Field1: "value1",
			},
			expected: payload.NewJSONBodyProvider(&TestStruct{
				Field1: "value1",
			}),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &Request{
				operation: tt.operation,
			}
			r.JSONBody(tt.p)
			assert.Equal(t, tt.expected, r.bodyProvider)
		})
	}
}

type TestStruct struct {
	Field1 string
}

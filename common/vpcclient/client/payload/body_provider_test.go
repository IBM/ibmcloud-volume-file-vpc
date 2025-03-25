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
package payload

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewJSONBodyProvider(t *testing.T) {
	payload := map[string]interface{}{}
	provider := NewJSONBodyProvider(payload)
	assert.Equal(t, "application/json", provider.ContentType())
	assert.NotNil(t, provider)
}

func TestJSONBodyProvider_Body(t *testing.T) {
	payload := map[string]interface{}{"key": "value"}
	provider := NewJSONBodyProvider(payload)
	_, err := provider.Body()
	assert.NoError(t, err)
}

func TestNewMultipartFileBody(t *testing.T) {
	contents := bytes.NewReader([]byte("test contents"))
	body := NewMultipartFileBody("test_file", contents)
	assert.NotEqual(t, "", body.ContentType())
	assert.NotNil(t, body)
}

func TestMultipartFileBody_Body(t *testing.T) {
	contents := bytes.NewReader([]byte("test contents"))
	body := NewMultipartFileBody("test_file", contents)
	_, err := body.Body()
	assert.NoError(t, err)
}

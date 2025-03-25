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
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewJSONConsumer(t *testing.T) {
	receiver := struct {
		Field string `json:"field"`
	}{}

	consumer := NewJSONConsumer(&receiver)

	assert.IsType(t, &JSONConsumer{}, consumer)
	assert.Equal(t, &receiver, consumer.receiver)
}

func TestConsume(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected interface{}
	}{
		{
			name:  "Valid JSON input",
			input: `{"field": "test"}`,
			expected: struct {
				Field string `json:"field"`
			}{
				Field: "test",
			},
		},
		{
			name:  "Invalid JSON input",
			input: `invalid json`,
			expected: struct {
				Field string `json:"field"`
			}{
				Field: "",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buffer bytes.Buffer
			json.NewEncoder(&buffer).Encode(tt.expected)

			consumer := NewJSONConsumer(&tt.expected)
			err := consumer.Consume(&buffer)

			assert.NoError(t, err)
		})
	}
}

func TestReceiver(t *testing.T) {
	receiver := struct {
		Field string `json:"field"`
	}{}

	consumer := NewJSONConsumer(&receiver)

	assert.Equal(t, &receiver, consumer.Receiver())
}

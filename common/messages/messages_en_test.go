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

package messages

import (
	"testing"
)

func TestInitMessages(t *testing.T) {
	// Call the function under test
	messages := InitMessages()

	// Assert that the returned value is not nil
	if messages == nil {
		t.Errorf("InitMessages should not return nil")
	}

	// Assert that the map is not empty
	if len(messages) == 0 {
		t.Errorf("InitMessages should return a non-empty map")
	}

	// Assert that the keys in the map are as expected
	expectedKeys := []string{"AuthenticationFailed", "ErrorRequiredFieldMissing", "FailedToPlaceOrder"} // Replace with actual expected keys
	for _, key := range expectedKeys {
		if _, exists := messages[key]; !exists {
			t.Errorf("InitMessages should contain key: %s", key)
		}
	}
}

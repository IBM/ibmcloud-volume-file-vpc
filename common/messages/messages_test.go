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
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestGetUserErr tests the GetUserErr function.
func TestGetUserErr(t *testing.T) {
	MessagesEn = InitMessages()
	// Define test cases
	testCases := []struct {
		name           string
		code           string
		err            error
		expectedErrMsg string
	}{
		{
			name:           "No error",
			code:           "NO_ERROR",
			err:            nil,
			expectedErrMsg: "",
		},
		{
			name:           "Error with message",
			code:           "FailedToPlaceOrder",
			err:            fmt.Errorf("Backend error"),
			expectedErrMsg: "{Code:FailedToPlaceOrder, Description:Failed to create file share with the storage provider.Backend error, RC:500}",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := GetUserErr(tc.code, tc.err)

			if tc.expectedErrMsg != "" {
				assert.Equal(t, tc.expectedErrMsg, err.Error())
			} else {
				assert.Nil(t, err)
			}

		})
	}
}

// TestGetUserError tests the GetUserError function.
func TestGetUserError(t *testing.T) {
	MessagesEn = InitMessages()
	// Define test cases
	testCases := []struct {
		name           string
		code           string
		err            error
		arg            string
		expectedErrMsg string
	}{
		{
			name:           "Error with message",
			code:           "FailedToPlaceOrder",
			err:            fmt.Errorf("Backend error"),
			arg:            "",
			expectedErrMsg: "{Code:FailedToPlaceOrder, Description:Failed to create file share with the storage provider.Backend error, RC:500}",
		},
		{
			name:           "Error with message and arguments",
			code:           "VolumeNotInValidState",
			err:            fmt.Errorf("Volume stuck in pending state"),
			arg:            "volume-id",
			expectedErrMsg: "{Code:VolumeNotInValidState, Description:File share volume-id did not get valid (available) status within timeout period..Volume stuck in pending state, RC:500}",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var err error
			if tc.arg != "" {
				err = GetUserError(tc.code, tc.err, tc.arg)
			} else {
				err = GetUserError(tc.code, tc.err)
			}

			if tc.expectedErrMsg != "" {
				assert.Equal(t, tc.expectedErrMsg, err.Error())
			} else {
				assert.Nil(t, err)
			}
		})
	}
}

/**
 * Copyright 2021 IBM Corp.
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

// Package provider ...
package provider

import (
	"errors"
	"os"
	"testing"
	"time"

	"github.com/IBM/ibmcloud-volume-file-vpc/common/vpcclient/models"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func TestSetRetryParameters(t *testing.T) {
	SetRetryParameters(2, 5)
	assert.Equal(t, maxRetryAttempt, 2)
	assert.Equal(t, maxRetryGap, 5)
}

func GetTestContextLogger() (*zap.Logger, zap.AtomicLevel) {
	consoleDebugging := zapcore.Lock(os.Stdout)
	consoleErrors := zapcore.Lock(os.Stderr)
	encoderConfig := zap.NewProductionEncoderConfig()
	encoderConfig.TimeKey = "ts"
	encoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	traceLevel := zap.NewAtomicLevel()
	traceLevel.SetLevel(zap.InfoLevel)
	core := zapcore.NewTee(
		zapcore.NewCore(zapcore.NewJSONEncoder(encoderConfig), consoleDebugging, zap.LevelEnablerFunc(func(lvl zapcore.Level) bool {
			return (lvl >= traceLevel.Level()) && (lvl < zapcore.ErrorLevel)
		})),
		zapcore.NewCore(zapcore.NewJSONEncoder(encoderConfig), consoleErrors, zap.LevelEnablerFunc(func(lvl zapcore.Level) bool {
			return lvl >= zapcore.ErrorLevel
		})),
	)
	logger := zap.New(core, zap.AddCaller())
	return logger, traceLevel
}

func TestRetry(t *testing.T) {
	// Setup new style zap logger
	logger, _ := GetTestContextLogger()
	SetRetryParameters(2, 5)
	var err error
	var attempt int
	err = retry(logger, func() error {
		logger.Info("Testing retry with successful attempt")
		if attempt == 2 {
			err = nil
		} else {
			errCode := models.ErrorCode("validation_invalid_name")
			errItem := models.ErrorItem{
				Code: errCode,
			}

			err = &models.Error{
				Errors: []models.ErrorItem{errItem},
			}
		}
		return err
	})

	err = retry(logger, func() error {
		logger.Info("Testing retry with unsuccessful attempt")
		errCode := models.ErrorCode("wrong_code")
		errItem := models.ErrorItem{
			Code: errCode,
		}

		err = &models.Error{
			Errors: []models.ErrorItem{errItem},
		}
		return err
	})
}

func TestSkipRetry(t *testing.T) {
	errCode := models.ErrorCode("bad_field")
	errItem := models.ErrorItem{
		Code: errCode,
	}

	err := &models.Error{
		Errors: []models.ErrorItem{errItem},
	}

	skip := skipRetry(err)
	assert.Equal(t, skip, true)

	errCode = models.ErrorCode("wrong_code")
	errItem = models.ErrorItem{
		Code: errCode,
	}

	err = &models.Error{
		Errors: []models.ErrorItem{errItem},
	}

	skip = skipRetry(err)
	assert.Equal(t, skip, false)
}

func TestRetryWithError(t *testing.T) {
	maxRetryAttempt = 2
	maxRetryGap = 20

	// Setup new style zap logger
	logger, _ := GetTestContextLogger()
	var err error
	err = retry(logger, func() error {
		logger.Info("Testing retry with error")
		err = errors.New("trace Code:, testerr Please check ")
		return err
	})
}

func TestFromProviderToLibVolume(t *testing.T) {
	// Setup new style zap logger
	logger, _ := GetTestContextLogger()

	timeNow := time.Now()
	vpcVolume := &models.Share{
		ID:        "Test Volume ID",
		Name:      "Test Volume",
		Size:      int64(10),
		Iops:      int64(1000),
		CreatedAt: &timeNow,
		Zone: &models.Zone{
			Name: "Test Zone",
		},
	}
	providerVolume := FromProviderToLibVolume(vpcVolume, logger)
	assert.NotNil(t, providerVolume)
}

func TestToInt(t *testing.T) {
	value := ToInt("519")
	assert.Equal(t, value, 519)
	value = ToInt("wrong value")
	assert.Equal(t, value, 0)
}

func TestToInt64(t *testing.T) {
	value := ToInt64("519")
	assert.Equal(t, value, int64(519))
	value = ToInt64("wrong value")
	assert.Equal(t, value, int64(0))
}

func TestIsValidVolumeIDFormat(t *testing.T) {
	returnValue := IsValidVolumeIDFormat("test-id")
	assert.Equal(t, returnValue, false)
	returnValue = IsValidVolumeIDFormat("34c3ad36-34d9-4d3a-8463-5a176c75801c")
	assert.Equal(t, returnValue, true)
}

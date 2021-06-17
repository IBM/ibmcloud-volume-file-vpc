package utils

import (
	"os"

	uid "github.com/satori/go.uuid"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"golang.org/x/net/context"
)

// GetContextLogger ...
func GetContextLogger(ctx context.Context, isDebug bool) (*zap.Logger, string) {
	return GetContextLoggerWithRequestID(ctx, isDebug, nil)
}

// GetContextLoggerWithRequestID  adds existing requestID in the logger
// The Existing requestID might be comming from ControllerPublishVolume etc
func GetContextLoggerWithRequestID(ctx context.Context, isDebug bool, requestIDIn *string) (*zap.Logger, string) {
	consoleDebugging := zapcore.Lock(os.Stdout)
	consoleErrors := zapcore.Lock(os.Stderr)
	encoderConfig := zap.NewProductionEncoderConfig()
	encoderConfig.TimeKey = "ts"
	encoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	traceLevel := zap.NewAtomicLevel()
	if isDebug {
		traceLevel.SetLevel(zap.DebugLevel)
	} else {
		traceLevel.SetLevel(zap.InfoLevel)
	}

	core := zapcore.NewTee(
		zapcore.NewCore(zapcore.NewJSONEncoder(encoderConfig), consoleDebugging, zap.LevelEnablerFunc(func(lvl zapcore.Level) bool {
			return (lvl >= traceLevel.Level()) && (lvl < zapcore.ErrorLevel)
		})),
		zapcore.NewCore(zapcore.NewJSONEncoder(encoderConfig), consoleErrors, zap.LevelEnablerFunc(func(lvl zapcore.Level) bool {
			return lvl >= zapcore.ErrorLevel
		})),
	)
	logger := zap.New(core, zap.AddCaller())
	// generating a unique request ID so that logs can be filter
	if requestIDIn == nil {
		// Generate New RequestID if not provided
		requestID := uid.NewV4().String()
		requestIDIn = &requestID
	}
	logger = logger.With(zap.String("RequestID", *requestIDIn))
	return logger, *requestIDIn + " "
}

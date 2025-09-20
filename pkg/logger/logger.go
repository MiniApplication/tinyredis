package logger

import (
	"github.com/hsn/tiny-redis/pkg/config"
	"go.uber.org/zap"
)

// SetUp initializes the logger with zap
func SetUp(cfg *config.Config) error {
	// Initialize zap logger
	if err := InitZapLogger(cfg); err != nil {
		return err
	}

	return nil
}

// Disable disables logging
func Disable() {
	if Logger != nil {
		Logger = zap.NewNop()
		Sugar = Logger.Sugar()
	}
}

// Debug logs a debug message with optional fields
func Debug(args ...interface{}) {
	if Sugar != nil {
		Sugar.Debug(args...)
	}
}

// Debugf logs a formatted debug message
func Debugf(template string, args ...interface{}) {
	if Sugar != nil {
		Sugar.Debugf(template, args...)
	}
}

// DebugWithFields logs a debug message with structured fields
func DebugWithFields(msg string, fields ...zap.Field) {
	if Logger != nil {
		Logger.Debug(msg, fields...)
	}
}

// Info logs an info message with optional fields
func Info(args ...interface{}) {
	if Sugar != nil {
		Sugar.Info(args...)
	}
}

// Infof logs a formatted info message
func Infof(template string, args ...interface{}) {
	if Sugar != nil {
		Sugar.Infof(template, args...)
	}
}

// InfoWithFields logs an info message with structured fields
func InfoWithFields(msg string, fields ...zap.Field) {
	if Logger != nil {
		Logger.Info(msg, fields...)
	}
}

// Warning logs a warning message with optional fields
func Warning(args ...interface{}) {
	if Sugar != nil {
		Sugar.Warn(args...)
	}
}

// Warnf logs a formatted warning message
func Warnf(template string, args ...interface{}) {
	if Sugar != nil {
		Sugar.Warnf(template, args...)
	}
}

// WarnWithFields logs a warning message with structured fields
func WarnWithFields(msg string, fields ...zap.Field) {
	if Logger != nil {
		Logger.Warn(msg, fields...)
	}
}

// Error logs an error message with optional fields
func Error(args ...interface{}) {
	if Sugar != nil {
		Sugar.Error(args...)
	}
}

// Errorf logs a formatted error message
func Errorf(template string, args ...interface{}) {
	if Sugar != nil {
		Sugar.Errorf(template, args...)
	}
}

// ErrorWithFields logs an error message with structured fields
func ErrorWithFields(msg string, fields ...zap.Field) {
	if Logger != nil {
		Logger.Error(msg, fields...)
	}
}

// Panic logs a panic message and panics
func Panic(args ...interface{}) {
	if Sugar != nil {
		Sugar.Panic(args...)
	}
}

// Panicf logs a formatted panic message and panics
func Panicf(template string, args ...interface{}) {
	if Sugar != nil {
		Sugar.Panicf(template, args...)
	}
}

// PanicWithFields logs a panic message with structured fields and panics
func PanicWithFields(msg string, fields ...zap.Field) {
	if Logger != nil {
		Logger.Panic(msg, fields...)
	}
}

// Fatal logs a fatal message and exits the program
func Fatal(args ...interface{}) {
	if Sugar != nil {
		Sugar.Fatal(args...)
	}
}

// Fatalf logs a formatted fatal message and exits the program
func Fatalf(template string, args ...interface{}) {
	if Sugar != nil {
		Sugar.Fatalf(template, args...)
	}
}

// FatalWithFields logs a fatal message with structured fields and exits
func FatalWithFields(msg string, fields ...zap.Field) {
	if Logger != nil {
		Logger.Fatal(msg, fields...)
	}
}

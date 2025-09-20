package logger

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/hsn/tiny-redis/pkg/config"
)

var (
	// Sugar provides a more ergonomic API for common logging patterns
	Sugar *zap.SugaredLogger
	// Logger is the underlying zap logger for structured logging
	Logger *zap.Logger
)

// InitZapLogger initializes the zap logger with the given configuration
func InitZapLogger(cfg *config.Config) error {
	var zapLevel zapcore.Level
	switch cfg.LogLevel {
	case "debug":
		zapLevel = zapcore.DebugLevel
	case "info":
		zapLevel = zapcore.InfoLevel
	case "warning", "warn":
		zapLevel = zapcore.WarnLevel
	case "error":
		zapLevel = zapcore.ErrorLevel
	case "panic":
		zapLevel = zapcore.PanicLevel
	default:
		zapLevel = zapcore.InfoLevel
	}

	// Create log directory if it doesn't exist
	if cfg.LogDir != "" {
		if err := os.MkdirAll(cfg.LogDir, 0755); err != nil {
			return fmt.Errorf("failed to create log directory: %w", err)
		}
	}

	// Build encoder config
	encoderConfig := zapcore.EncoderConfig{
		TimeKey:        "time",
		LevelKey:       "level",
		NameKey:        "logger",
		CallerKey:      "caller",
		FunctionKey:    zapcore.OmitKey,
		MessageKey:     "msg",
		StacktraceKey:  "stacktrace",
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeLevel:    zapcore.LowercaseLevelEncoder,
		EncodeTime:     customTimeEncoder,
		EncodeDuration: zapcore.SecondsDurationEncoder,
		EncodeCaller:   zapcore.ShortCallerEncoder,
	}

	// Create core for console output
	consoleEncoder := zapcore.NewConsoleEncoder(encoderConfig)
	consoleCore := zapcore.NewCore(
		consoleEncoder,
		zapcore.Lock(os.Stdout),
		zapLevel,
	)

	cores := []zapcore.Core{consoleCore}

	// Add file output if log directory is specified
	if cfg.LogDir != "" {
		logFile := filepath.Join(cfg.LogDir, "redis.log")
		file, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			return fmt.Errorf("failed to open log file: %w", err)
		}

		// File encoder (JSON for better parsing)
		fileEncoderConfig := encoderConfig
		fileEncoder := zapcore.NewJSONEncoder(fileEncoderConfig)
		fileCore := zapcore.NewCore(
			fileEncoder,
			zapcore.Lock(file),
			zapLevel,
		)
		cores = append(cores, fileCore)
	}

	// Combine cores
	core := zapcore.NewTee(cores...)

	// Build logger with caller information
	Logger = zap.New(core,
		zap.AddCaller(),
		zap.AddCallerSkip(1), // Skip one level to show the actual caller
		zap.AddStacktrace(zapcore.ErrorLevel),
	)

	Sugar = Logger.Sugar()

	return nil
}

// customTimeEncoder formats time in a readable way
func customTimeEncoder(t time.Time, enc zapcore.PrimitiveArrayEncoder) {
	enc.AppendString(t.Format("2006-01-02 15:04:05.000"))
}

// Sync flushes any buffered log entries
func Sync() error {
	if Logger != nil {
		return Logger.Sync()
	}
	return nil
}

// With creates a child logger with the given fields
func With(fields ...zap.Field) *zap.Logger {
	if Logger == nil {
		return zap.NewNop()
	}
	return Logger.With(fields...)
}

// WithSugar creates a sugared child logger with the given fields
func WithSugar(keysAndValues ...interface{}) *zap.SugaredLogger {
	if Sugar == nil {
		return zap.NewNop().Sugar()
	}
	return Sugar.With(keysAndValues...)
}

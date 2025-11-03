package logger

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/hsn0918/tinyredis/pkg/config"
)

var (
	loggerMu sync.RWMutex
	logger   *slog.Logger
	logFile  *os.File
)

// SetUp initializes the slog-based logger using application configuration.
func SetUp(cfg *config.Config) error {
	newLogger, file, err := buildLogger(cfg)
	if err != nil {
		return err
	}

	loggerMu.Lock()
	defer loggerMu.Unlock()

	if logFile != nil {
		_ = logFile.Close()
		logFile = nil
	}

	logger = newLogger
	logFile = file
	slog.SetDefault(logger)
	return nil
}

// Disable disables logging by routing all logs to /dev/null.
func Disable() {
	loggerMu.Lock()
	defer loggerMu.Unlock()

	logger = slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{}))
	if logFile != nil {
		_ = logFile.Close()
		logFile = nil
	}
}

// Sync flushes any buffered log entries for the current log file.
func Sync() error {
	loggerMu.RLock()
	defer loggerMu.RUnlock()
	if logFile != nil {
		return logFile.Sync()
	}
	return nil
}

// With returns a new logger pre-populated with the provided attributes.
func With(attrs ...slog.Attr) *slog.Logger {
	return getLogger().With(convertAttrs(attrs)...)
}

// WithValues returns a new logger using key/value pairs (kept for API parity).
func WithValues(args ...any) *slog.Logger {
	return getLogger().With(args...)
}

// Debug logs a debug message.
func Debug(args ...interface{}) {
	logMessage(slog.LevelDebug, fmt.Sprint(args...))
}

// Debugf logs a formatted debug message.
func Debugf(template string, args ...interface{}) {
	logMessage(slog.LevelDebug, fmt.Sprintf(template, args...))
}

// DebugWithFields logs a debug message with structured fields.
func DebugWithFields(msg string, attrs ...slog.Attr) {
	logAttrs(slog.LevelDebug, msg, attrs...)
}

// Info logs an info message.
func Info(args ...interface{}) {
	logMessage(slog.LevelInfo, fmt.Sprint(args...))
}

// Infof logs a formatted info message.
func Infof(template string, args ...interface{}) {
	logMessage(slog.LevelInfo, fmt.Sprintf(template, args...))
}

// InfoWithFields logs an info message with structured fields.
func InfoWithFields(msg string, attrs ...slog.Attr) {
	logAttrs(slog.LevelInfo, msg, attrs...)
}

// Warning logs a warning message.
func Warning(args ...interface{}) {
	logMessage(slog.LevelWarn, fmt.Sprint(args...))
}

// Warnf logs a formatted warning message.
func Warnf(template string, args ...interface{}) {
	logMessage(slog.LevelWarn, fmt.Sprintf(template, args...))
}

// WarnWithFields logs a warning message with structured fields.
func WarnWithFields(msg string, attrs ...slog.Attr) {
	logAttrs(slog.LevelWarn, msg, attrs...)
}

// Error logs an error message.
func Error(args ...interface{}) {
	logMessage(slog.LevelError, fmt.Sprint(args...))
}

// Errorf logs a formatted error message.
func Errorf(template string, args ...interface{}) {
	logMessage(slog.LevelError, fmt.Sprintf(template, args...))
}

// ErrorWithFields logs an error message with structured fields.
func ErrorWithFields(msg string, attrs ...slog.Attr) {
	logAttrs(slog.LevelError, msg, attrs...)
}

// Panic logs a panic message and panics.
func Panic(args ...interface{}) {
	msg := fmt.Sprint(args...)
	logMessage(slog.LevelError, msg)
	panic(msg)
}

// Panicf logs a formatted panic message and panics.
func Panicf(template string, args ...interface{}) {
	msg := fmt.Sprintf(template, args...)
	logMessage(slog.LevelError, msg)
	panic(msg)
}

// PanicWithFields logs a panic message with structured fields and panics.
func PanicWithFields(msg string, attrs ...slog.Attr) {
	logAttrs(slog.LevelError, msg, attrs...)
	panic(msg)
}

// Fatal logs a fatal message and exits the program.
func Fatal(args ...interface{}) {
	msg := fmt.Sprint(args...)
	logMessage(slog.LevelError, msg)
	os.Exit(1)
}

// Fatalf logs a formatted fatal message and exits the program.
func Fatalf(template string, args ...interface{}) {
	msg := fmt.Sprintf(template, args...)
	logMessage(slog.LevelError, msg)
	os.Exit(1)
}

// FatalWithFields logs a fatal message with structured fields and exits.
func FatalWithFields(msg string, attrs ...slog.Attr) {
	logAttrs(slog.LevelError, msg, attrs...)
	os.Exit(1)
}

func logMessage(level slog.Level, msg string) {
	getLogger().Log(context.Background(), level, msg)
}

func logAttrs(level slog.Level, msg string, attrs ...slog.Attr) {
	getLogger().LogAttrs(context.Background(), level, msg, attrs...)
}

func getLogger() *slog.Logger {
	loggerMu.RLock()
	l := logger
	loggerMu.RUnlock()
	if l != nil {
		return l
	}
	return slog.Default()
}

func convertAttrs(attrs []slog.Attr) []any {
	if len(attrs) == 0 {
		return nil
	}
	out := make([]any, len(attrs))
	for i, attr := range attrs {
		out[i] = attr
	}
	return out
}

func buildLogger(cfg *config.Config) (*slog.Logger, *os.File, error) {
	level := parseLevel(cfg.LogLevel)
	opts := &slog.HandlerOptions{
		Level:     level,
		AddSource: true,
	}

	handlers := []slog.Handler{slog.NewTextHandler(os.Stdout, opts)}

	var file *os.File
	if cfg.LogDir != "" {
		if err := os.MkdirAll(cfg.LogDir, 0o755); err != nil {
			return nil, nil, fmt.Errorf("failed to create log directory: %w", err)
		}
		logPath := filepath.Join(cfg.LogDir, "redis.log")
		f, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to open log file: %w", err)
		}
		file = f
		handlers = append(handlers, slog.NewJSONHandler(f, opts))
	}

	handler := multiHandlerFrom(handlers)

	if cfg.LogSamplingEnabled {
		handler = newSamplingHandler(handler, cfg.LogSamplingInterval, cfg.LogSamplingInitial, cfg.LogSamplingThereafter)
	}

	logger := slog.New(handler)
	if cfg != nil && strings.TrimSpace(cfg.NodeID) != "" {
		logger = logger.With("node_id", strings.TrimSpace(cfg.NodeID))
	}
	return logger, file, nil
}

func parseLevel(lvl string) slog.Level {
	switch strings.ToLower(lvl) {
	case "debug":
		return slog.LevelDebug
	case "info":
		return slog.LevelInfo
	case "warning", "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	case "panic", "fatal":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

func multiHandlerFrom(handlers []slog.Handler) slog.Handler {
	switch len(handlers) {
	case 0:
		return slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{})
	case 1:
		return handlers[0]
	default:
		return &multiHandler{handlers: handlers}
	}
}

type multiHandler struct {
	handlers []slog.Handler
}

func (h *multiHandler) Enabled(ctx context.Context, level slog.Level) bool {
	for _, handler := range h.handlers {
		if handler.Enabled(ctx, level) {
			return true
		}
	}
	return false
}

func (h *multiHandler) Handle(ctx context.Context, record slog.Record) error {
	var result error
	for idx, handler := range h.handlers {
		rec := record
		if idx < len(h.handlers)-1 {
			rec = record.Clone()
		}
		if err := handler.Handle(ctx, rec); err != nil && result == nil {
			result = err
		}
	}
	return result
}

func (h *multiHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	next := make([]slog.Handler, len(h.handlers))
	for i, handler := range h.handlers {
		next[i] = handler.WithAttrs(attrs)
	}
	return &multiHandler{handlers: next}
}

func (h *multiHandler) WithGroup(name string) slog.Handler {
	next := make([]slog.Handler, len(h.handlers))
	for i, handler := range h.handlers {
		next[i] = handler.WithGroup(name)
	}
	return &multiHandler{handlers: next}
}

type samplingHandler struct {
	next       slog.Handler
	interval   time.Duration
	initial    int
	thereafter int
	mu         sync.Mutex
	statPerKey map[string]*samplingState
}

type samplingState struct {
	windowStart time.Time
	count       int
}

func newSamplingHandler(next slog.Handler, interval time.Duration, initial, thereafter int) slog.Handler {
	if interval <= 0 || initial <= 0 || thereafter <= 0 {
		return next
	}
	return &samplingHandler{
		next:       next,
		interval:   interval,
		initial:    initial,
		thereafter: thereafter,
		statPerKey: make(map[string]*samplingState),
	}
}

func (h *samplingHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.next.Enabled(ctx, level)
}

func (h *samplingHandler) Handle(ctx context.Context, record slog.Record) error {
	if !h.shouldLog(record) {
		return nil
	}
	return h.next.Handle(ctx, record)
}

func (h *samplingHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &samplingHandler{
		next:       h.next.WithAttrs(attrs),
		interval:   h.interval,
		initial:    h.initial,
		thereafter: h.thereafter,
		statPerKey: make(map[string]*samplingState),
	}
}

func (h *samplingHandler) WithGroup(name string) slog.Handler {
	return &samplingHandler{
		next:       h.next.WithGroup(name),
		interval:   h.interval,
		initial:    h.initial,
		thereafter: h.thereafter,
		statPerKey: make(map[string]*samplingState),
	}
}

func (h *samplingHandler) shouldLog(record slog.Record) bool {
	now := time.Now()
	key := fmt.Sprintf("%d:%s", record.Level, record.Message)

	h.mu.Lock()
	defer h.mu.Unlock()

	state, ok := h.statPerKey[key]
	if !ok || now.Sub(state.windowStart) >= h.interval {
		h.statPerKey[key] = &samplingState{
			windowStart: now,
			count:       1,
		}
		return true
	}

	state.count++
	if state.count <= h.initial {
		return true
	}

	if (state.count-h.initial)%h.thereafter == 0 {
		return true
	}
	return false
}

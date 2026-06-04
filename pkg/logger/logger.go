// Package logger provides a structured JSON logger wrapping log/slog (ADR-017).
// Technical logs go async to stdout → Docker → Promtail → Loki.
// Safety events use Event() which adds an event_type field for LogQL filtering.
package logger

import (
	"log/slog"
	"os"
	"strings"
)

// Logger wraps slog with a fixed service label and convenience methods.
type Logger struct {
	inner *slog.Logger
}

// New creates a Logger for the given service name.
// Level is configured via the LOG_LEVEL environment variable (debug|info|warn|error, default: info).
func New(service string) *Logger {
	handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: parseLevel(os.Getenv("LOG_LEVEL"))})
	return &Logger{inner: slog.New(handler).With("service", service)}
}

func (l *Logger) Info(msg string, args ...any)  { l.inner.Info(msg, args...) }
func (l *Logger) Warn(msg string, args ...any)  { l.inner.Warn(msg, args...) }
func (l *Logger) Error(msg string, args ...any) { l.inner.Error(msg, args...) }
func (l *Logger) Debug(msg string, args ...any) { l.inner.Debug(msg, args...) }

// Fatal logs at ERROR level and exits with code 1.
func (l *Logger) Fatal(msg string, args ...any) {
	l.inner.Error(msg, args...)
	os.Exit(1)
}

// Event logs a structured event with an event_type field.
// Use for all safety, audit, and session lifecycle events (ADR-017).
// The event_type label enables LogQL queries like:
//
//	{service="control-server"} | json | event_type="EMERGENCY_STOP"
func (l *Logger) Event(eventType string, msg string, args ...any) {
	all := make([]any, 0, len(args)+2)
	all = append(all, "event_type", eventType)
	all = append(all, args...)
	l.inner.Info(msg, all...)
}

func parseLevel(s string) slog.Level {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "debug":
		return slog.LevelDebug
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

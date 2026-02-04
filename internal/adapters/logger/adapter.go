// Package logger provides adapters for the logging interface.
package logger

import (
	"context"
)

// Logger defines the logging interface used throughout the application.
// External loggers that implement these methods can be wrapped with ZapAdapter.
type Logger interface {
	Info(ctx context.Context, msg string, fields map[string]any)
	Debug(ctx context.Context, msg string, fields map[string]any)
	Warn(ctx context.Context, msg string, fields map[string]any)
	Error(ctx context.Context, msg string, err error, fields map[string]any)
}

// ZapAdapter adapts a Logger to the application's logging interface.
type ZapAdapter struct {
	log Logger
}

// NewZapAdapter creates a new ZapAdapter wrapping the given logger.
func NewZapAdapter(log Logger) *ZapAdapter {
	return &ZapAdapter{log: log}
}

// Info logs an info message.
func (a *ZapAdapter) Info(ctx context.Context, msg string, fields map[string]any) {
	a.log.Info(ctx, msg, fields)
}

// Debug logs a debug message.
func (a *ZapAdapter) Debug(ctx context.Context, msg string, fields map[string]any) {
	a.log.Debug(ctx, msg, fields)
}

// Warn logs a warning message.
func (a *ZapAdapter) Warn(ctx context.Context, msg string, fields map[string]any) {
	a.log.Warn(ctx, msg, fields)
}

// Error logs an error message.
func (a *ZapAdapter) Error(ctx context.Context, msg string, err error, fields map[string]any) {
	a.log.Error(ctx, msg, err, fields)
}

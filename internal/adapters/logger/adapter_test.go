package logger

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

// mockLogger implements Logger interface for testing.
type mockLogger struct {
	infoCalled  bool
	debugCalled bool
	warnCalled  bool
	errorCalled bool
	lastMsg     string
	lastFields  map[string]any
	lastErr     error
}

func (m *mockLogger) Info(_ context.Context, msg string, fields map[string]any) {
	m.infoCalled = true
	m.lastMsg = msg
	m.lastFields = fields
}

func (m *mockLogger) Debug(_ context.Context, msg string, fields map[string]any) {
	m.debugCalled = true
	m.lastMsg = msg
	m.lastFields = fields
}

func (m *mockLogger) Warn(_ context.Context, msg string, fields map[string]any) {
	m.warnCalled = true
	m.lastMsg = msg
	m.lastFields = fields
}

func (m *mockLogger) Error(_ context.Context, msg string, err error, fields map[string]any) {
	m.errorCalled = true
	m.lastMsg = msg
	m.lastErr = err
	m.lastFields = fields
}

func TestNewZapAdapter(t *testing.T) {
	mock := &mockLogger{}
	adapter := NewZapAdapter(mock)

	assert.NotNil(t, adapter)
}

func TestZapAdapter_Info(t *testing.T) {
	mock := &mockLogger{}
	adapter := NewZapAdapter(mock)
	ctx := context.Background()
	fields := map[string]any{"key": "value"}

	adapter.Info(ctx, "test message", fields)

	assert.True(t, mock.infoCalled)
	assert.Equal(t, "test message", mock.lastMsg)
	assert.Equal(t, fields, mock.lastFields)
}

func TestZapAdapter_Debug(t *testing.T) {
	mock := &mockLogger{}
	adapter := NewZapAdapter(mock)
	ctx := context.Background()
	fields := map[string]any{"debug": true}

	adapter.Debug(ctx, "debug message", fields)

	assert.True(t, mock.debugCalled)
	assert.Equal(t, "debug message", mock.lastMsg)
	assert.Equal(t, fields, mock.lastFields)
}

func TestZapAdapter_Warn(t *testing.T) {
	mock := &mockLogger{}
	adapter := NewZapAdapter(mock)
	ctx := context.Background()
	fields := map[string]any{"warning": "test"}

	adapter.Warn(ctx, "warn message", fields)

	assert.True(t, mock.warnCalled)
	assert.Equal(t, "warn message", mock.lastMsg)
	assert.Equal(t, fields, mock.lastFields)
}

func TestZapAdapter_Error(t *testing.T) {
	mock := &mockLogger{}
	adapter := NewZapAdapter(mock)
	ctx := context.Background()
	testErr := assert.AnError
	fields := map[string]any{"error_context": "test"}

	adapter.Error(ctx, "error message", testErr, fields)

	assert.True(t, mock.errorCalled)
	assert.Equal(t, "error message", mock.lastMsg)
	assert.Equal(t, testErr, mock.lastErr)
	assert.Equal(t, fields, mock.lastFields)
}

package config

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func clearEnv(t *testing.T) {
	t.Helper()
	for _, key := range []string{EnvSlippyAPIURL, EnvSlippyAPIKey, EnvLogLevel, EnvLogAppName} {
		t.Setenv(key, "")
		os.Unsetenv(key)
	}
}

func TestLoad_MissingAPIURL(t *testing.T) {
	clearEnv(t)
	_, err := Load()
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrSlippyAPIURLRequired)
}

func TestLoad_MissingAPIKey(t *testing.T) {
	clearEnv(t)
	t.Setenv(EnvSlippyAPIURL, "http://slippy-api/v1")
	_, err := Load()
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrSlippyAPIKeyRequired)
}

func TestLoad_Defaults(t *testing.T) {
	clearEnv(t)
	t.Setenv(EnvSlippyAPIURL, "http://slippy-api/v1")
	t.Setenv(EnvSlippyAPIKey, "test-key")

	cfg, err := Load()
	require.NoError(t, err)

	assert.Equal(t, "http://slippy-api/v1", cfg.SlippyAPIURL)
	assert.Equal(t, "test-key", cfg.SlippyAPIKey)
	assert.Equal(t, DefaultLogLevel, cfg.LogLevel)
	assert.Equal(t, DefaultLogAppName, cfg.LogAppName)
}

func TestLoad_CustomLogSettings(t *testing.T) {
	clearEnv(t)
	t.Setenv(EnvSlippyAPIURL, "http://slippy-api/v1")
	t.Setenv(EnvSlippyAPIKey, "test-key")
	t.Setenv(EnvLogLevel, "debug")
	t.Setenv(EnvLogAppName, "custom-name")

	cfg, err := Load()
	require.NoError(t, err)

	assert.Equal(t, "debug", cfg.LogLevel)
	assert.Equal(t, "custom-name", cfg.LogAppName)
}

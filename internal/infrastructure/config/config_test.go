package config

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func clearEnv(t *testing.T) {
	t.Helper()
	for _, key := range []string{
		EnvSlippyAPIURL, EnvSlippyAPIKey, EnvSlippyAPIIPv4Only, EnvLogLevel, EnvLogAppName,
	} {
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

func TestLoad_InvalidAPIURL(t *testing.T) {
	cases := []struct {
		name  string
		value string
	}{
		{"no scheme", "slippy-api.example.com"},
		{"only scheme", "https://"},
		{"trailing newline", "http://slippy-api/v1\n"},
		{"embedded whitespace", "http://slippy api/v1"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			clearEnv(t)
			t.Setenv(EnvSlippyAPIURL, tc.value)
			t.Setenv(EnvSlippyAPIKey, "test-key")

			_, err := Load()
			require.Error(t, err)
			assert.Contains(t, err.Error(), EnvSlippyAPIURL)
			assert.Contains(t, err.Error(), "valid http")
		})
	}
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

func TestLoad_IPv4OnlyDefaultsToFalse(t *testing.T) {
	clearEnv(t)
	t.Setenv(EnvSlippyAPIURL, "http://slippy-api/v1")
	t.Setenv(EnvSlippyAPIKey, "test-key")

	cfg, err := Load()

	require.NoError(t, err)
	assert.False(t, cfg.SlippyAPIIPv4Only, "dual-stack must stay the default")
}

func TestLoad_IPv4OnlyParsesBooleans(t *testing.T) {
	cases := map[string]bool{
		"true":  true,
		"TRUE":  true,
		"1":     true,
		"false": false,
		"0":     false,
	}

	for value, want := range cases {
		t.Run(value, func(t *testing.T) {
			clearEnv(t)
			t.Setenv(EnvSlippyAPIURL, "http://slippy-api/v1")
			t.Setenv(EnvSlippyAPIKey, "test-key")
			t.Setenv(EnvSlippyAPIIPv4Only, value)

			cfg, err := Load()

			require.NoError(t, err)
			assert.Equal(t, want, cfg.SlippyAPIIPv4Only)
		})
	}
}

func TestLoad_IPv4OnlyRejectsNonBoolean(t *testing.T) {
	clearEnv(t)
	t.Setenv(EnvSlippyAPIURL, "http://slippy-api/v1")
	t.Setenv(EnvSlippyAPIKey, "test-key")
	t.Setenv(EnvSlippyAPIIPv4Only, "yes")

	_, err := Load()

	require.Error(t, err)
	assert.Contains(t, err.Error(), EnvSlippyAPIIPv4Only)
	assert.Contains(t, err.Error(), "boolean")
}

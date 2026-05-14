package config

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func clearEnv(t *testing.T) {
	t.Helper()
	for _, key := range []string{EnvSlippyAPIURL, EnvSlippyAPIKey, EnvK8sNamespace, EnvLogLevel, EnvLogAppName} {
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
		{"embedded whitespace", "http://slippy api/v1"},
		// Note: surrounding whitespace (including trailing newlines) is now
		// trimmed by resolveSlippyAPIURL — operators routinely have shell
		// snippets that append "\n". The url.Parse check still catches
		// internal whitespace and other malformed inputs.
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

// Resolver-level coverage (precedence, edge cases, typo errors, malformed
// override URL) lives in goLibMyCarrier/slippyapi. Tests here only pin
// the slippy-find-level integration: Load() surfaces resolver errors and
// the missing-URL sentinel.

func TestLoad_NamespaceFallback(t *testing.T) {
	clearEnv(t)
	t.Setenv(EnvK8sNamespace, "argo-events-test")
	t.Setenv(EnvSlippyAPIKey, "test-key")

	cfg, err := Load()
	require.NoError(t, err)
	assert.Equal(t, "https://slippy-api-test.api.mycarrier.tech/v1", cfg.SlippyAPIURL)
}

func TestLoad_UnknownNamespaceReturnsError(t *testing.T) {
	clearEnv(t)
	t.Setenv(EnvK8sNamespace, "argo-evenst") // typo
	t.Setenv(EnvSlippyAPIKey, "test-key")

	_, err := Load()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "K8S_NAMESPACE")
}

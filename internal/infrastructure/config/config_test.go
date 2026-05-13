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

func TestResolveSlippyAPIURL(t *testing.T) {
	cases := []struct {
		name      string
		explicit  string
		expSet    bool
		namespace string
		nsSet     bool
		wantURL   string
		wantErr   bool
	}{
		{name: "explicit wins", explicit: "http://override.example.com", expSet: true, namespace: "argo-events", nsSet: true, wantURL: "http://override.example.com"},
		{name: "explicit trimmed", explicit: "  http://override.example.com  ", expSet: true, wantURL: "http://override.example.com"},
		{name: "argo-events -> prod", namespace: "argo-events", nsSet: true, wantURL: "https://slippy-api.api.mycarrier.tech/v1"},
		{name: "argo-events-test -> non-prod", namespace: "argo-events-test", nsSet: true, wantURL: "https://slippy-api-test.api.mycarrier.tech/v1"},
		{name: "neither set", wantURL: ""},
		{name: "empty namespace treated as unset", namespace: "", nsSet: true, wantURL: ""},
		{name: "whitespace namespace treated as unset", namespace: "   ", nsSet: true, wantURL: ""},
		{name: "unknown namespace errors", namespace: "some-other-ns", nsSet: true, wantErr: true},
		{name: "case-sensitive match", namespace: "ARGO-EVENTS", nsSet: true, wantErr: true},
		{name: "typo namespace errors", namespace: "argo-evenst", nsSet: true, wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			clearEnv(t)
			if tc.expSet {
				t.Setenv(EnvSlippyAPIURL, tc.explicit)
			}
			if tc.nsSet {
				t.Setenv(EnvK8sNamespace, tc.namespace)
			}
			got, err := resolveSlippyAPIURL()
			if tc.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), "K8S_NAMESPACE")
				assert.Contains(t, err.Error(), tc.namespace)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.wantURL, got)
		})
	}
}

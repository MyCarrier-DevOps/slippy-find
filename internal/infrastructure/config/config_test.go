package config

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockVaultClient implements VaultClient interface for testing.
type mockVaultClient struct {
	secrets map[string]map[string]interface{}
	err     error
}

func (m *mockVaultClient) GetKVSecret(_ context.Context, path, _ string) (map[string]interface{}, error) {
	if m.err != nil {
		return nil, m.err
	}
	if secret, ok := m.secrets[path]; ok {
		return secret, nil
	}
	return nil, errors.New("secret not found")
}

// mockVaultClientFactory creates a factory that returns the provided mock client.
func mockVaultClientFactory(client VaultClient, err error) VaultClientFactory {
	return func(_ context.Context) (VaultClient, error) {
		if err != nil {
			return nil, err
		}
		return client, nil
	}
}

func TestLoad_MissingPipelineConfig(t *testing.T) {
	// Ensure pipeline config sources are not set
	os.Unsetenv(EnvPipelineConfig)
	os.Unsetenv(EnvVaultPipelineConfigPath)

	// Set required ClickHouse env vars to avoid that error
	setClickHouseEnvVars(t)

	// Act
	_, err := Load()

	// Assert
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrPipelineConfigRequired)
}

func TestLoad_PipelineConfigFileNotFound(t *testing.T) {
	// Set required env vars
	setClickHouseEnvVars(t)
	t.Setenv(EnvPipelineConfig, "/nonexistent/path/to/config.json")
	os.Unsetenv(EnvVaultPipelineConfigPath)

	// Act
	_, err := Load()

	// Assert
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrPipelineConfigNotFound)
}

func TestLoad_InvalidPipelineConfigJSON(t *testing.T) {
	// Create a temp file with invalid JSON
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "invalid.json")
	err := os.WriteFile(configPath, []byte("not valid json"), 0o644)
	require.NoError(t, err)

	// Set required env vars
	setClickHouseEnvVars(t)
	t.Setenv(EnvPipelineConfig, configPath)
	os.Unsetenv(EnvVaultPipelineConfigPath)

	// Act
	_, err = Load()

	// Assert
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrPipelineConfigInvalid)
}

func TestLoad_ValidConfig(t *testing.T) {
	// Create a temp file with valid pipeline config JSON
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "pipeline.json")
	validConfig := `{
		"version": "1",
		"name": "test-pipeline",
		"steps": [
			{"name": "push_parsed", "description": "Push parsed"}
		]
	}`
	err := os.WriteFile(configPath, []byte(validConfig), 0o644)
	require.NoError(t, err)

	// Set required env vars
	setClickHouseEnvVars(t)
	t.Setenv(EnvPipelineConfig, configPath)
	os.Unsetenv(EnvVaultPipelineConfigPath)

	// Act
	cfg, err := Load()

	// Assert
	require.NoError(t, err)
	require.NotNil(t, cfg)
	assert.NotNil(t, cfg.ClickHouse)
	assert.NotNil(t, cfg.PipelineConfig)
	assert.Equal(t, DefaultDatabase, cfg.Database)
}

func TestLoad_DefaultLogSettings(t *testing.T) {
	// Create a temp file with valid pipeline config JSON
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "pipeline.json")
	validConfig := `{
		"version": "1",
		"name": "test-pipeline",
		"steps": [
			{"name": "push_parsed", "description": "Push parsed"}
		]
	}`
	err := os.WriteFile(configPath, []byte(validConfig), 0o644)
	require.NoError(t, err)

	// Set required env vars, but not log settings
	setClickHouseEnvVars(t)
	t.Setenv(EnvPipelineConfig, configPath)
	os.Unsetenv(EnvVaultPipelineConfigPath)
	os.Unsetenv(EnvLogLevel)
	os.Unsetenv(EnvLogAppName)

	// Act
	cfg, err := Load()

	// Assert
	require.NoError(t, err)
	assert.Equal(t, DefaultLogLevel, cfg.LogLevel)
	assert.Equal(t, DefaultLogAppName, cfg.LogAppName)
}

func TestLoad_CustomLogSettings(t *testing.T) {
	// Create a temp file with valid pipeline config JSON
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "pipeline.json")
	validConfig := `{
		"version": "1",
		"name": "test-pipeline",
		"steps": [
			{"name": "push_parsed", "description": "Push parsed"}
		]
	}`
	err := os.WriteFile(configPath, []byte(validConfig), 0o644)
	require.NoError(t, err)

	// Set all env vars including custom log settings
	setClickHouseEnvVars(t)
	t.Setenv(EnvPipelineConfig, configPath)
	os.Unsetenv(EnvVaultPipelineConfigPath)
	t.Setenv(EnvLogLevel, "debug")
	t.Setenv(EnvLogAppName, "custom-app")

	// Act
	cfg, err := Load()

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "debug", cfg.LogLevel)
	assert.Equal(t, "custom-app", cfg.LogAppName)
}

func TestLoad_DefaultDatabase(t *testing.T) {
	// Create a temp file with valid pipeline config JSON
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "pipeline.json")
	validConfig := `{"version":"1","name":"test","steps":[{"name":"step1","description":"desc"}]}`
	err := os.WriteFile(configPath, []byte(validConfig), 0o644)
	require.NoError(t, err)

	// Set required env vars, but not database
	setClickHouseEnvVars(t)
	t.Setenv(EnvPipelineConfig, configPath)
	os.Unsetenv(EnvVaultPipelineConfigPath)
	os.Unsetenv(EnvDatabase)

	// Act
	cfg, err := Load()

	// Assert
	require.NoError(t, err)
	assert.Equal(t, DefaultDatabase, cfg.Database)
}

func TestLoad_CustomDatabase(t *testing.T) {
	// Create a temp file with valid pipeline config JSON
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "pipeline.json")
	validConfig := `{"version":"1","name":"test","steps":[{"name":"step1","description":"desc"}]}`
	err := os.WriteFile(configPath, []byte(validConfig), 0o644)
	require.NoError(t, err)

	// Set all env vars including custom database
	setClickHouseEnvVars(t)
	t.Setenv(EnvPipelineConfig, configPath)
	os.Unsetenv(EnvVaultPipelineConfigPath)
	t.Setenv(EnvDatabase, "production")

	// Act
	cfg, err := Load()

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "production", cfg.Database)
}

// Vault integration tests

func TestLoadWithVaultClient_VaultConfigAsJSONString(t *testing.T) {
	// Set required env vars
	setClickHouseEnvVars(t)
	t.Setenv(EnvVaultPipelineConfigPath, "ci/slippy/pipeline")
	os.Unsetenv(EnvPipelineConfig)

	// Create mock vault client with JSON string in "config" key
	mockClient := &mockVaultClient{
		secrets: map[string]map[string]interface{}{
			"ci/slippy/pipeline": {
				"config": `{"version":"1","name":"vault-pipeline","steps":[{"name":"push_parsed","description":"Push parsed"}]}`,
			},
		},
	}

	// Act
	cfg, err := LoadWithVaultClient(context.Background(), mockVaultClientFactory(mockClient, nil))

	// Assert
	require.NoError(t, err)
	require.NotNil(t, cfg)
	require.NotNil(t, cfg.PipelineConfig)
	assert.Equal(t, "vault-pipeline", cfg.PipelineConfig.Name)
}

func TestLoadWithVaultClient_VaultConfigAsDirectMapping(t *testing.T) {
	// Set required env vars
	setClickHouseEnvVars(t)
	t.Setenv(EnvVaultPipelineConfigPath, "ci/slippy/pipeline")
	os.Unsetenv(EnvPipelineConfig)

	// Create mock vault client with direct mapping
	mockClient := &mockVaultClient{
		secrets: map[string]map[string]interface{}{
			"ci/slippy/pipeline": {
				"version": "1",
				"name":    "direct-mapping-pipeline",
				"steps": []interface{}{
					map[string]interface{}{
						"name":        "push_parsed",
						"description": "Push parsed",
					},
				},
			},
		},
	}

	// Act
	cfg, err := LoadWithVaultClient(context.Background(), mockVaultClientFactory(mockClient, nil))

	// Assert
	require.NoError(t, err)
	require.NotNil(t, cfg)
	require.NotNil(t, cfg.PipelineConfig)
	assert.Equal(t, "direct-mapping-pipeline", cfg.PipelineConfig.Name)
}

func TestLoadWithVaultClient_VaultClientError(t *testing.T) {
	// Set required env vars
	setClickHouseEnvVars(t)
	t.Setenv(EnvVaultPipelineConfigPath, "ci/slippy/pipeline")
	os.Unsetenv(EnvPipelineConfig)

	// Create factory that returns an error
	factory := mockVaultClientFactory(nil, errors.New("vault connection failed"))

	// Act
	_, err := LoadWithVaultClient(context.Background(), factory)

	// Assert
	require.Error(t, err)
	assert.Contains(t, err.Error(), "vault connection failed")
}

func TestLoadWithVaultClient_VaultSecretNotFound(t *testing.T) {
	// Set required env vars
	setClickHouseEnvVars(t)
	t.Setenv(EnvVaultPipelineConfigPath, "nonexistent/path")
	os.Unsetenv(EnvPipelineConfig)

	// Create mock vault client with no secrets
	mockClient := &mockVaultClient{
		secrets: map[string]map[string]interface{}{},
	}

	// Act
	_, err := LoadWithVaultClient(context.Background(), mockVaultClientFactory(mockClient, nil))

	// Assert
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrVaultSecretNotFound)
}

func TestLoadWithVaultClient_VaultInvalidJSON(t *testing.T) {
	// Set required env vars
	setClickHouseEnvVars(t)
	t.Setenv(EnvVaultPipelineConfigPath, "ci/slippy/pipeline")
	os.Unsetenv(EnvPipelineConfig)

	// Create mock vault client with invalid JSON in config key
	mockClient := &mockVaultClient{
		secrets: map[string]map[string]interface{}{
			"ci/slippy/pipeline": {
				"config": "not valid json",
			},
		},
	}

	// Act
	_, err := LoadWithVaultClient(context.Background(), mockVaultClientFactory(mockClient, nil))

	// Assert
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrPipelineConfigInvalid)
}

func TestLoadWithVaultClient_CustomMount(t *testing.T) {
	// Set required env vars with custom mount
	setClickHouseEnvVars(t)
	t.Setenv(EnvVaultPipelineConfigPath, "ci/slippy/pipeline")
	t.Setenv(EnvVaultPipelineConfigMount, "custom-kv")
	os.Unsetenv(EnvPipelineConfig)

	// Create mock vault client
	mockClient := &mockVaultClient{
		secrets: map[string]map[string]interface{}{
			"ci/slippy/pipeline": {
				"config": `{"version":"1","name":"custom-mount-pipeline","steps":[]}`,
			},
		},
	}

	// Act
	cfg, err := LoadWithVaultClient(context.Background(), mockVaultClientFactory(mockClient, nil))

	// Assert
	require.NoError(t, err)
	require.NotNil(t, cfg)
	assert.Equal(t, "custom-mount-pipeline", cfg.PipelineConfig.Name)
}

func TestLoadWithVaultClient_FallsBackToFile(t *testing.T) {
	// Create a temp file with valid pipeline config JSON
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "pipeline.json")
	validConfig := `{
		"version": "1",
		"name": "file-fallback-pipeline",
		"steps": []
	}`
	err := os.WriteFile(configPath, []byte(validConfig), 0o644)
	require.NoError(t, err)

	// Set file path but not Vault path
	setClickHouseEnvVars(t)
	t.Setenv(EnvPipelineConfig, configPath)
	os.Unsetenv(EnvVaultPipelineConfigPath)

	// Act - should use file since Vault path not set
	cfg, err := LoadWithVaultClient(context.Background(), nil)

	// Assert
	require.NoError(t, err)
	require.NotNil(t, cfg)
	assert.Equal(t, "file-fallback-pipeline", cfg.PipelineConfig.Name)
}

func TestLoadPipelineConfigFromFile_ReadError(t *testing.T) {
	// Create a directory instead of a file to trigger a read error (not IsNotExist)
	tmpDir := t.TempDir()
	dirPath := filepath.Join(tmpDir, "not-a-file")
	err := os.Mkdir(dirPath, 0o755)
	require.NoError(t, err)

	// Set required env vars
	setClickHouseEnvVars(t)
	t.Setenv(EnvPipelineConfig, dirPath)
	os.Unsetenv(EnvVaultPipelineConfigPath)

	// Act
	_, err = Load()

	// Assert - should fail with read error, not "not found"
	require.Error(t, err)
	assert.NotErrorIs(t, err, ErrPipelineConfigNotFound)
	assert.Contains(t, err.Error(), "failed to read pipeline config")
}

func TestParsePipelineConfigFromVault_MarshalError(t *testing.T) {
	// Set required env vars
	setClickHouseEnvVars(t)
	t.Setenv(EnvVaultPipelineConfigPath, "ci/slippy/pipeline")
	os.Unsetenv(EnvPipelineConfig)

	// Create mock vault client with data that will fail unmarshal to PipelineConfig
	// (valid JSON but wrong structure for PipelineConfig)
	mockClient := &mockVaultClient{
		secrets: map[string]map[string]interface{}{
			"ci/slippy/pipeline": {
				// No "config" key, and invalid structure for direct mapping
				"invalid_field": make(chan int), // channels can't be marshaled to JSON
			},
		},
	}

	// Act
	_, err := LoadWithVaultClient(context.Background(), mockVaultClientFactory(mockClient, nil))

	// Assert
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrPipelineConfigInvalid)
}

// setClickHouseEnvVars sets the required ClickHouse environment variables for testing.
func setClickHouseEnvVars(t *testing.T) {
	t.Helper()
	t.Setenv("CLICKHOUSE_HOSTNAME", "localhost")
	t.Setenv("CLICKHOUSE_PORT", "9000")
	t.Setenv("CLICKHOUSE_USERNAME", "default")
	t.Setenv("CLICKHOUSE_PASSWORD", "testpassword")
	t.Setenv("CLICKHOUSE_DATABASE", "ci")
	t.Setenv("CLICKHOUSE_SKIP_VERIFY", "true")
}

func TestParseVaultPath(t *testing.T) {
	tests := []struct {
		name     string
		fullPath string
		wantPath string
		wantKey  string
	}{
		{
			name:     "path without key uses default",
			fullPath: "ci/slippy/pipeline",
			wantPath: "ci/slippy/pipeline",
			wantKey:  DefaultSecretKey,
		},
		{
			name:     "path with explicit key",
			fullPath: "DevOps/slippy/config#config",
			wantPath: "DevOps/slippy/config",
			wantKey:  "config",
		},
		{
			name:     "path with custom key",
			fullPath: "my/secret/path#mykey",
			wantPath: "my/secret/path",
			wantKey:  "mykey",
		},
		{
			name:     "path with multiple hash symbols uses last one",
			fullPath: "path/with#hash/in/name#actualkey",
			wantPath: "path/with#hash/in/name",
			wantKey:  "actualkey",
		},
		{
			name:     "path ending with hash only returns empty key",
			fullPath: "ci/slippy/pipeline#",
			wantPath: "ci/slippy/pipeline",
			wantKey:  "",
		},
		{
			name:     "simple path",
			fullPath: "secret",
			wantPath: "secret",
			wantKey:  DefaultSecretKey,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotPath, gotKey := parseVaultPath(tt.fullPath)
			assert.Equal(t, tt.wantPath, gotPath, "path mismatch")
			assert.Equal(t, tt.wantKey, gotKey, "key mismatch")
		})
	}
}

func TestLoadWithVaultClient_CustomKey(t *testing.T) {
	// Set required env vars with custom key syntax
	setClickHouseEnvVars(t)
	os.Unsetenv(EnvPipelineConfig)
	t.Setenv(EnvVaultPipelineConfigPath, "ci/slippy/pipeline#myconfig")

	// Create mock vault client with JSON string in custom key
	mockClient := &mockVaultClient{
		secrets: map[string]map[string]interface{}{
			"ci/slippy/pipeline": {
				"myconfig": `{"version":"1","name":"test-pipeline","steps":[{"name":"push_parsed","description":"Push parsed"}]}`,
			},
		},
	}

	// Act
	cfg, err := LoadWithVaultClient(context.Background(), mockVaultClientFactory(mockClient, nil))

	// Assert
	require.NoError(t, err)
	require.NotNil(t, cfg)
	assert.Equal(t, "test-pipeline", cfg.PipelineConfig.Name)
}

func TestLoadWithVaultClient_KeyNotFoundFallsBackToSecret(t *testing.T) {
	// When the specified key doesn't exist as a string, the code falls back
	// to treating the entire secret as the config. This test verifies that behavior.
	setClickHouseEnvVars(t)
	os.Unsetenv(EnvPipelineConfig)
	t.Setenv(EnvVaultPipelineConfigPath, "ci/slippy/pipeline#nonexistent")

	// The secret has a different key, but the entire secret IS a valid pipeline config
	mockClient := &mockVaultClient{
		secrets: map[string]map[string]interface{}{
			"ci/slippy/pipeline": {
				"version": "1",
				"name":    "fallback-pipeline",
				"steps": []interface{}{
					map[string]interface{}{
						"name":        "push_parsed",
						"description": "Push parsed",
					},
				},
			},
		},
	}

	// Act
	cfg, err := LoadWithVaultClient(context.Background(), mockVaultClientFactory(mockClient, nil))

	// Assert - should succeed by falling back to the entire secret
	require.NoError(t, err)
	require.NotNil(t, cfg)
	assert.Equal(t, "fallback-pipeline", cfg.PipelineConfig.Name)
}

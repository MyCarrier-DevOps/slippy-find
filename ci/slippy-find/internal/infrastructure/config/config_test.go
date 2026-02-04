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

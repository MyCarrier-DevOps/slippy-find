// Package config provides configuration loading for the slippy-find application.
// It handles loading ClickHouse configuration, pipeline configuration, and
// other application settings from environment variables and HashiCorp Vault.
package config

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"

	ch "github.com/MyCarrier-DevOps/goLibMyCarrier/clickhouse"
	"github.com/MyCarrier-DevOps/goLibMyCarrier/slippy"
	"github.com/MyCarrier-DevOps/goLibMyCarrier/vault"
)

// Environment variable names.
const (
	// EnvPipelineConfig is the path to the pipeline configuration JSON file (deprecated, use Vault).
	EnvPipelineConfig = "SLIPPY_PIPELINE_CONFIG"

	// EnvLogLevel is the log level (debug, info, error).
	EnvLogLevel = "LOG_LEVEL"

	// EnvLogAppName is the application name for log context.
	EnvLogAppName = "LOG_APP_NAME"

	// EnvVaultPipelineConfigPath is the path in Vault KV where pipeline config is stored.
	EnvVaultPipelineConfigPath = "VAULT_PIPELINE_CONFIG_PATH"

	// EnvVaultPipelineConfigMount is the Vault KV mount point (defaults to "secret").
	EnvVaultPipelineConfigMount = "VAULT_PIPELINE_CONFIG_MOUNT"
)

// Default values.
const (
	DefaultLogLevel           = "info"
	DefaultLogAppName         = "slippy-find"
	DefaultDatabase           = "ci"
	DefaultVaultPipelineMount = "secret"
)

// Configuration errors.
var (
	// ErrPipelineConfigRequired indicates pipeline config source is not available.
	ErrPipelineConfigRequired = errors.New(
		"pipeline configuration required: set VAULT_PIPELINE_CONFIG_PATH (with VAULT_ADDRESS, VAULT_ROLE_ID, VAULT_SECRET_ID) " +
			"or SLIPPY_PIPELINE_CONFIG for local file",
	)

	// ErrPipelineConfigNotFound indicates the pipeline config file does not exist.
	ErrPipelineConfigNotFound = errors.New("pipeline configuration file not found")

	// ErrPipelineConfigInvalid indicates the pipeline config is not valid JSON.
	ErrPipelineConfigInvalid = errors.New("pipeline configuration is not valid JSON")

	// ErrVaultClientFailed indicates failure to create or authenticate with Vault.
	ErrVaultClientFailed = errors.New("failed to create Vault client")

	// ErrVaultSecretNotFound indicates the secret was not found in Vault.
	ErrVaultSecretNotFound = errors.New("pipeline configuration not found in Vault")
)

// VaultClient defines the interface for Vault operations.
// This interface allows for dependency injection and testing.
type VaultClient interface {
	// GetKVSecret retrieves a secret from Vault's KV v2 secrets engine.
	GetKVSecret(ctx context.Context, path, mount string) (map[string]interface{}, error)
}

// VaultClientFactory creates a VaultClient using AppRole authentication.
// This is the default factory used in production.
type VaultClientFactory func(ctx context.Context) (VaultClient, error)

// DefaultVaultClientFactory creates a VaultClient using goLibMyCarrier/vault with AppRole auth.
func DefaultVaultClientFactory(ctx context.Context) (VaultClient, error) {
	// Load Vault configuration from environment variables
	// Uses: VAULT_ADDRESS, VAULT_ROLE_ID, VAULT_SECRET_ID
	vaultConfig, err := vault.VaultLoadConfig()
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrVaultClientFailed, err)
	}

	// Create client with AppRole authentication
	client, err := vault.CreateVaultClient(ctx, vaultConfig)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrVaultClientFailed, err)
	}

	return client, nil
}

// Config holds all application configuration.
type Config struct {
	// ClickHouse holds the ClickHouse connection configuration.
	ClickHouse *ch.ClickhouseConfig

	// PipelineConfig holds the pipeline step definitions.
	PipelineConfig *slippy.PipelineConfig

	// Database is the ClickHouse database name for slip storage.
	Database string

	// LogLevel is the logging level (debug, info, error).
	LogLevel string

	// LogAppName is the application name for log context.
	LogAppName string
}

// Load loads the application configuration from environment variables.
// Pipeline configuration is loaded from Vault (preferred) or local file (fallback).
//
// For Vault loading, requires:
//   - VAULT_ADDRESS: Vault server address
//   - VAULT_ROLE_ID: AppRole role ID
//   - VAULT_SECRET_ID: AppRole secret ID
//   - VAULT_PIPELINE_CONFIG_PATH: Path to the secret in Vault
//   - VAULT_PIPELINE_CONFIG_MOUNT: KV mount point (optional, defaults to "secret")
//
// For file loading (fallback):
//   - SLIPPY_PIPELINE_CONFIG: Path to local JSON file
//
// Returns ErrPipelineConfigRequired if no pipeline config source is available.
func Load() (*Config, error) {
	return LoadWithVaultClient(context.Background(), nil)
}

// LoadWithVaultClient loads configuration using the provided VaultClient factory.
// If vaultClientFactory is nil, DefaultVaultClientFactory is used.
// This function enables dependency injection for testing.
func LoadWithVaultClient(ctx context.Context, vaultClientFactory VaultClientFactory) (*Config, error) {
	// Load ClickHouse configuration
	chConfig, err := ch.ClickhouseLoadConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to load ClickHouse config: %w", err)
	}

	// Load pipeline configuration (try Vault first, then file fallback)
	pipelineConfig, err := loadPipelineConfigWithVault(ctx, vaultClientFactory)
	if err != nil {
		return nil, err
	}

	// Get log settings with defaults
	logLevel := os.Getenv(EnvLogLevel)
	if logLevel == "" {
		logLevel = DefaultLogLevel
	}

	logAppName := os.Getenv(EnvLogAppName)
	if logAppName == "" {
		logAppName = DefaultLogAppName
	}

	return &Config{
		ClickHouse:     chConfig,
		PipelineConfig: pipelineConfig,
		Database:       DefaultDatabase,
		LogLevel:       logLevel,
		LogAppName:     logAppName,
	}, nil
}

// loadPipelineConfigWithVault attempts to load pipeline config from Vault first,
// falling back to local file if Vault is not configured.
func loadPipelineConfigWithVault(
	ctx context.Context,
	vaultClientFactory VaultClientFactory,
) (*slippy.PipelineConfig, error) {
	// Check if Vault configuration is available
	vaultPath := os.Getenv(EnvVaultPipelineConfigPath)
	if vaultPath != "" {
		// Vault is configured, load from Vault
		return loadPipelineConfigFromVault(ctx, vaultClientFactory, vaultPath)
	}

	// Fall back to local file
	pipelineConfigPath := os.Getenv(EnvPipelineConfig)
	if pipelineConfigPath == "" {
		return nil, ErrPipelineConfigRequired
	}

	return loadPipelineConfigFromFile(pipelineConfigPath)
}

// loadPipelineConfigFromVault loads pipeline configuration from Vault KV v2.
func loadPipelineConfigFromVault(
	ctx context.Context,
	vaultClientFactory VaultClientFactory,
	path string,
) (*slippy.PipelineConfig, error) {
	// Use default factory if none provided
	if vaultClientFactory == nil {
		vaultClientFactory = DefaultVaultClientFactory
	}

	// Create Vault client
	client, err := vaultClientFactory(ctx)
	if err != nil {
		return nil, err
	}

	// Get mount point (default to "secret")
	mount := os.Getenv(EnvVaultPipelineConfigMount)
	if mount == "" {
		mount = DefaultVaultPipelineMount
	}

	// Read secret from Vault
	secretData, err := client.GetKVSecret(ctx, path, mount)
	if err != nil {
		return nil, fmt.Errorf("%w at path %s: %w", ErrVaultSecretNotFound, path, err)
	}

	// The pipeline config should be stored as a JSON string in a "config" key
	// or directly as the secret data
	return parsePipelineConfigFromVault(secretData)
}

// parsePipelineConfigFromVault parses pipeline config from Vault secret data.
// Supports two formats:
// 1. A "config" key containing JSON string
// 2. Direct mapping of pipeline config fields in the secret
func parsePipelineConfigFromVault(secretData map[string]interface{}) (*slippy.PipelineConfig, error) {
	// Try to get config as JSON string from "config" key
	if configStr, ok := secretData["config"].(string); ok {
		var config slippy.PipelineConfig
		if err := json.Unmarshal([]byte(configStr), &config); err != nil {
			return nil, fmt.Errorf("%w: %w", ErrPipelineConfigInvalid, err)
		}
		return &config, nil
	}

	// Try to marshal the entire secret data as pipeline config
	jsonData, err := json.Marshal(secretData)
	if err != nil {
		return nil, fmt.Errorf("%w: failed to marshal secret data: %w", ErrPipelineConfigInvalid, err)
	}

	var config slippy.PipelineConfig
	if err := json.Unmarshal(jsonData, &config); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrPipelineConfigInvalid, err)
	}

	return &config, nil
}

// loadPipelineConfigFromFile loads the pipeline configuration from the specified file path.
func loadPipelineConfigFromFile(path string) (*slippy.PipelineConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("%w: %s", ErrPipelineConfigNotFound, path)
		}
		return nil, fmt.Errorf("failed to read pipeline config: %w", err)
	}

	var config slippy.PipelineConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrPipelineConfigInvalid, err)
	}

	return &config, nil
}

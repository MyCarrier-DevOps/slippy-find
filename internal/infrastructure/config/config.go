// Package config provides configuration loading for the slippy-find application.
package config

import (
	"errors"
	"fmt"
	"net/url"
	"os"

	slippyapi "github.com/MyCarrier-DevOps/goLibMyCarrier/slippyapi"
)

// Environment variable names.
const (
	EnvSlippyAPIURL = "SLIPPY_API_URL"
	EnvSlippyAPIKey = "SLIPPY_API_KEY"
	EnvK8sNamespace = "K8S_NAMESPACE"
	EnvLogLevel     = "LOG_LEVEL"
	EnvLogAppName   = "LOG_APP_NAME"
)

// Default values.
const (
	DefaultLogLevel   = "info"
	DefaultLogAppName = "slippy-find"
)

// Configuration errors.
var (
	ErrSlippyAPIURLRequired = errors.New(
		"SLIPPY_API_URL is required (set explicitly, or set K8S_NAMESPACE to a known slippy-api cluster: argo-events / argo-events-test)",
	)
	ErrSlippyAPIKeyRequired = errors.New("SLIPPY_API_KEY is required")
)

// Config holds all application configuration.
type Config struct {
	// SlippyAPIURL is the base URL of the slippy-api service (e.g. "http://slippy-api/v1").
	SlippyAPIURL string

	// SlippyAPIKey is the Bearer token for authenticating read requests.
	SlippyAPIKey string

	// LogLevel is the logging level (debug, info, error).
	LogLevel string

	// LogAppName is the application name for log context.
	LogAppName string
}

// Load loads the application configuration from environment variables.
//
// SLIPPY_API_URL resolution is delegated to
// goLibMyCarrier/slippyapi.ResolveAPIURL: explicit SLIPPY_API_URL wins,
// otherwise K8S_NAMESPACE maps to a known cluster, and an unknown
// namespace returns an error. See that package for the full contract.
func Load() (*Config, error) {
	apiURL, err := slippyapi.ResolveAPIURL()
	if err != nil {
		return nil, err
	}
	if apiURL == "" {
		return nil, fmt.Errorf("%w", ErrSlippyAPIURLRequired)
	}
	if u, parseErr := url.Parse(apiURL); parseErr != nil || u.Scheme == "" || u.Host == "" {
		return nil, fmt.Errorf("%s must be a valid http(s):// URL, got %q", EnvSlippyAPIURL, apiURL)
	}

	apiKey := os.Getenv(EnvSlippyAPIKey)
	if apiKey == "" {
		return nil, fmt.Errorf("%w", ErrSlippyAPIKeyRequired)
	}

	logLevel := os.Getenv(EnvLogLevel)
	if logLevel == "" {
		logLevel = DefaultLogLevel
	}

	logAppName := os.Getenv(EnvLogAppName)
	if logAppName == "" {
		logAppName = DefaultLogAppName
	}

	return &Config{
		SlippyAPIURL: apiURL,
		SlippyAPIKey: apiKey,
		LogLevel:     logLevel,
		LogAppName:   logAppName,
	}, nil
}

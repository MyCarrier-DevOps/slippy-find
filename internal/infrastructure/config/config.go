// Package config provides configuration loading for the slippy-find application.
package config

import (
	"errors"
	"fmt"
	"net/url"
	"os"
)

// Environment variable names.
const (
	EnvSlippyAPIURL = "SLIPPY_API_URL"
	EnvSlippyAPIKey = "SLIPPY_API_KEY"
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
	ErrSlippyAPIURLRequired = errors.New("SLIPPY_API_URL is required")
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
func Load() (*Config, error) {
	apiURL := os.Getenv(EnvSlippyAPIURL)
	if apiURL == "" {
		return nil, fmt.Errorf("%w", ErrSlippyAPIURLRequired)
	}
	if u, err := url.Parse(apiURL); err != nil || u.Scheme == "" || u.Host == "" {
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

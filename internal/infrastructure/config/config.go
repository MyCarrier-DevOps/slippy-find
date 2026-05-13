// Package config provides configuration loading for the slippy-find application.
package config

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"strings"
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
	apiURL, err := resolveSlippyAPIURL()
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

// resolveSlippyAPIURL returns the slippy-api base URL.
//
// Resolution order:
//  1. SLIPPY_API_URL (explicit override — integration tests, local dev,
//     in-cluster URLs). Honored verbatim.
//  2. K8S_NAMESPACE mapped to the cluster's slippy-api URL.
//  3. ("", nil) when K8S_NAMESPACE is unset / empty / whitespace.
//     Load() then surfaces the existing ErrSlippyAPIURLRequired error.
//  4. ("", error) when K8S_NAMESPACE is non-empty but unknown —
//     operator typo / new cluster, fail fast.
//
// NOTE: this inline helper will be replaced by
// goLibMyCarrier/slippy.ResolveAPIURL once that helper is tagged
// (MyCarrier-DevOps/goLibMyCarrier#60).
func resolveSlippyAPIURL() (string, error) {
	if explicit := strings.TrimSpace(os.Getenv(EnvSlippyAPIURL)); explicit != "" {
		return explicit, nil
	}
	ns := strings.TrimSpace(os.Getenv(EnvK8sNamespace))
	switch ns {
	case "":
		return "", nil
	case "argo-events":
		return "https://slippy-api.api.mycarrier.tech/v1", nil
	case "argo-events-test":
		return "https://slippy-api-test.api.mycarrier.tech/v1", nil
	default:
		return "", fmt.Errorf(
			"slippy: K8S_NAMESPACE=%q is not a known slippy-api cluster; set %s explicitly to override",
			ns, EnvSlippyAPIURL,
		)
	}
}

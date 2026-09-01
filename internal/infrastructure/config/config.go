// Package config provides configuration loading for the slippy-find application.
package config

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"strconv"
)

// Environment variable names.
const (
	EnvSlippyAPIURL      = "SLIPPY_API_URL"
	EnvSlippyAPIKey      = "SLIPPY_API_KEY"
	EnvSlippyAPIIPv4Only = "SLIPPY_API_IPV4_ONLY"
	EnvLogLevel          = "LOG_LEVEL"
	EnvLogAppName        = "LOG_APP_NAME"
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

	// SlippyAPIIPv4Only forces slippy-api dials onto IPv4. Enable it on IPv4-only hosts
	// such as GitHub-hosted runners, where the AAAA leg of a dual-stack dial can only fail.
	SlippyAPIIPv4Only bool

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
	// Enforce what the message already claims. A scheme this check let through — "htp://"
	// — reaches the HTTP client, which reports "unsupported protocol scheme" as an
	// ordinary request error indistinguishable from a transient one.
	if u, err := url.Parse(apiURL); err != nil || u.Host == "" ||
		(u.Scheme != "http" && u.Scheme != "https") {
		return nil, fmt.Errorf("%s must be a valid http(s):// URL, got %q", EnvSlippyAPIURL, apiURL)
	}

	apiKey := os.Getenv(EnvSlippyAPIKey)
	if apiKey == "" {
		return nil, fmt.Errorf("%w", ErrSlippyAPIKeyRequired)
	}

	ipv4Only, err := loadBool(EnvSlippyAPIIPv4Only)
	if err != nil {
		return nil, err
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
		SlippyAPIURL:      apiURL,
		SlippyAPIKey:      apiKey,
		SlippyAPIIPv4Only: ipv4Only,
		LogLevel:          logLevel,
		LogAppName:        logAppName,
	}, nil
}

// loadBool reads an optional boolean environment variable. An unset or empty value is
// false; anything strconv.ParseBool rejects is a configuration error rather than a silent
// false, so a typo like SLIPPY_API_IPV4_ONLY=yes is reported instead of ignored.
func loadBool(name string) (bool, error) {
	raw := os.Getenv(name)
	if raw == "" {
		return false, nil
	}

	value, err := strconv.ParseBool(raw)
	if err != nil {
		return false, fmt.Errorf("%s must be a boolean (true/false), got %q", name, raw)
	}
	return value, nil
}

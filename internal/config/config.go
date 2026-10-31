// Package config loads the exporter configuration from environment variables.
package config

import (
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"time"

	"github.com/d0ugal/promexporter/config"
)

// Config represents the application configuration
type Config struct {
	config.BaseConfig

	SLZB SLZBConfig `yaml:"slzb"`
}

// SLZBConfig represents the SLZB device configuration.
//
// Username and Password are the HTTP Basic auth credentials of the device web
// UI. They are optional: devices without a web password configured accept
// unauthenticated requests.
type SLZBConfig struct {
	APIURL   string        `yaml:"api_url"`
	Interval time.Duration `yaml:"interval"`
	Username string        `yaml:"username"`
	Password string        `yaml:"password"`
}

// redactedPassword replaces a non-empty password wherever the config is
// rendered, so the secret never reaches logs or traces.
const redactedPassword = "[REDACTED]"

// HasAuth reports whether both Basic auth credentials are set. A lone username
// or password is treated as "no auth", since the device rejects half a pair.
func (c SLZBConfig) HasAuth() bool {
	return c.Username != "" && c.Password != ""
}

// maskedPassword returns the placeholder for a set password and an empty
// string otherwise, so readers can still tell whether a password is configured.
func (c SLZBConfig) maskedPassword() string {
	if c.Password == "" {
		return ""
	}

	return redactedPassword
}

// String renders the config with the password redacted. fmt calls it for %v
// and %+v, including when SLZBConfig is nested in Config, which keeps an
// accidental Printf of the whole config from leaking the secret.
func (c SLZBConfig) String() string {
	return fmt.Sprintf("{APIURL:%s Interval:%s Username:%s Password:%s}",
		c.APIURL, c.Interval, c.Username, c.maskedPassword())
}

// LogValue implements slog.LogValuer so that logging the config through slog
// redacts the password as well.
func (c SLZBConfig) LogValue() slog.Value {
	return slog.GroupValue(
		slog.String("api_url", c.APIURL),
		slog.Duration("interval", c.Interval),
		slog.String("username", c.Username),
		slog.String("password", c.maskedPassword()),
	)
}

// LoadConfig loads configuration from environment variables with defaults.
func LoadConfig() (*Config, error) {
	cfg := &Config{}

	// Load base configuration from environment
	baseConfig := &config.BaseConfig{}

	// Server configuration
	if host := os.Getenv("SLZB_EXPORTER_SERVER_HOST"); host != "" {
		baseConfig.Server.Host = host
	} else {
		baseConfig.Server.Host = "0.0.0.0"
	}

	if portStr := os.Getenv("SLZB_EXPORTER_SERVER_PORT"); portStr != "" {
		if port, err := strconv.Atoi(portStr); err == nil {
			baseConfig.Server.Port = port
		} else {
			baseConfig.Server.Port = 9110
		}
	} else {
		baseConfig.Server.Port = 9110
	}

	// Logging configuration
	if level := os.Getenv("SLZB_EXPORTER_LOG_LEVEL"); level != "" {
		baseConfig.Logging.Level = level
	} else {
		baseConfig.Logging.Level = "info"
	}

	if format := os.Getenv("SLZB_EXPORTER_LOG_FORMAT"); format != "" {
		baseConfig.Logging.Format = format
	} else {
		baseConfig.Logging.Format = "json"
	}

	// Metrics configuration
	baseConfig.Metrics.Collection.DefaultInterval = config.Duration{}
	baseConfig.Metrics.Collection.DefaultIntervalSet = false

	cfg.BaseConfig = *baseConfig

	// Apply generic environment variables (TRACING_ENABLED, PROFILING_ENABLED, etc.)
	// These are handled by promexporter and are shared across all exporters
	if err := config.ApplyGenericEnvVars(&cfg.BaseConfig); err != nil {
		return nil, fmt.Errorf("failed to apply generic environment variables: %w", err)
	}

	// SLZB configuration
	if apiURL := os.Getenv("SLZB_EXPORTER_SLZB_API_URL"); apiURL != "" {
		cfg.SLZB.APIURL = apiURL
	} else {
		cfg.SLZB.APIURL = "http://slzb-device.local"
	}

	if intervalStr := os.Getenv("SLZB_EXPORTER_METRICS_DEFAULT_INTERVAL"); intervalStr != "" {
		if interval, err := time.ParseDuration(intervalStr); err == nil {
			cfg.SLZB.Interval = interval
		} else {
			cfg.SLZB.Interval = 10 * time.Second
		}
	} else {
		cfg.SLZB.Interval = 10 * time.Second
	}

	loadSLZBAuth(&cfg.SLZB)

	return cfg, nil
}

// loadSLZBAuth reads the optional device Basic auth credentials from the
// environment. Both values are taken verbatim (no trimming), since spaces may
// be part of a password.
func loadSLZBAuth(c *SLZBConfig) {
	c.Username = os.Getenv("SLZB_EXPORTER_SLZB_USERNAME")
	c.Password = os.Getenv("SLZB_EXPORTER_SLZB_PASSWORD")
}

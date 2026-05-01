package config

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

// Config holds the entire application configuration.
type Config struct {
	Server     ServerConfig     `yaml:"server"`
	Collector  CollectorConfig  `yaml:"collector"`
	Storage    StorageConfig    `yaml:"storage"`
	Auth       AuthConfig       `yaml:"auth"`
	GeoIP      GeoIPConfig      `yaml:"geoip"`
	Alerts     AlertsConfig     `yaml:"alerts"`
	Prometheus PrometheusConfig `yaml:"prometheus"`
	Logging    LoggingConfig    `yaml:"logging"`
}

type ServerConfig struct {
	Listen          string `yaml:"listen"`
	ReadTimeoutSec  int    `yaml:"read_timeout_sec"`
	WriteTimeoutSec int    `yaml:"write_timeout_sec"`
}

type CollectorConfig struct {
	InterfaceIntervalMs int      `yaml:"interface_interval_ms"`
	PortIntervalMs      int      `yaml:"port_interval_ms"`
	EBPFEnabled         bool     `yaml:"ebpf_enabled"`
	EBPFInterfaces      []string `yaml:"ebpf_interfaces"`
}

type StorageConfig struct {
	MemoryWindowHours int    `yaml:"memory_window_hours"`
	SQLitePath        string `yaml:"sqlite_path"`
	RetentionDays     int    `yaml:"retention_days"`
}

type AuthConfig struct {
	Enabled       bool   `yaml:"enabled"`
	JWTSecret     string `yaml:"jwt_secret"`
	AdminUsername string `yaml:"admin_username"`
	AdminPassword string `yaml:"admin_password"`
	SessionHours  int    `yaml:"session_hours"`
}

type GeoIPConfig struct {
	Enabled   bool   `yaml:"enabled"`
	CountryDB string `yaml:"country_db"`
	ASNDB     string `yaml:"asn_db"`
}

type AlertsConfig struct {
	Enabled   bool   `yaml:"enabled"`
	RulesFile string `yaml:"rules_file"`
}

type PrometheusConfig struct {
	Enabled bool   `yaml:"enabled"`
	Path    string `yaml:"path"`
}

type LoggingConfig struct {
	Level  string `yaml:"level"`
	Format string `yaml:"format"`
}

// Default returns a config with sensible defaults — used when no config file is present.
func Default() *Config {
	return &Config{
		Server: ServerConfig{
			Listen:          ":1234",
			ReadTimeoutSec:  15,
			WriteTimeoutSec: 15,
		},
		Collector: CollectorConfig{
			InterfaceIntervalMs: 1000,
			PortIntervalMs:      5000,
			EBPFEnabled:         true,
			EBPFInterfaces:      []string{"auto"},
		},
		Storage: StorageConfig{
			MemoryWindowHours: 24,
			SQLitePath:        "",
			RetentionDays:     30,
		},
		Auth: AuthConfig{
			Enabled:       false,
			JWTSecret:     "",
			AdminUsername: "admin",
			AdminPassword: "admin",
			SessionHours:  24,
		},
		Alerts: AlertsConfig{
			Enabled: true,
		},
		Prometheus: PrometheusConfig{
			Enabled: false,
			Path:    "/metrics",
		},
		Logging: LoggingConfig{
			Level:  "info",
			Format: "text",
		},
	}
}

// Load reads a YAML config file. If path is empty, returns defaults.
func Load(path string) (*Config, error) {
	cfg := Default()
	if path == "" {
		return cfg, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil // missing file → use defaults
		}
		return nil, fmt.Errorf("read config: %w", err)
	}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	return cfg, nil
}

// InterfaceInterval returns sampling interval as a duration.
func (c *CollectorConfig) InterfaceInterval() time.Duration {
	return time.Duration(c.InterfaceIntervalMs) * time.Millisecond
}

// PortInterval returns port sampling interval as a duration.
func (c *CollectorConfig) PortInterval() time.Duration {
	return time.Duration(c.PortIntervalMs) * time.Millisecond
}

// SessionDuration returns JWT session lifetime.
func (a *AuthConfig) SessionDuration() time.Duration {
	if a.SessionHours <= 0 {
		return 24 * time.Hour
	}
	return time.Duration(a.SessionHours) * time.Hour
}

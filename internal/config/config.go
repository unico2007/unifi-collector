// Package config loads and validates application configuration from a YAML
// file with environment-variable overrides. Load returns a fully-populated,
// immutable *Config; there is no package-level global state.
package config

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/viper"
)

// Config is the root configuration object, injected into the application.
type Config struct {
	Server     ServerConfig               `mapstructure:"server"`
	Logging    LoggingConfig              `mapstructure:"logging"`
	UniFi      UniFiConfig                `mapstructure:"unifi"`
	Kerio      KerioConfig                `mapstructure:"kerio"`
	Loki       LokiConfig                 `mapstructure:"loki"`
	Syslog     SyslogConfig               `mapstructure:"syslog"`
	Collectors map[string]CollectorConfig `mapstructure:"collectors"`
}

// KerioConfig holds connection settings for an optional Kerio Control firewall
// (a second vendor). It is only used when Enabled is true; its interfaces are
// surfaced through the shared "devices" collector alongside UniFi devices.
type KerioConfig struct {
	Enabled   bool          `mapstructure:"enabled"`
	URL       string        `mapstructure:"url"`
	Username  string        `mapstructure:"username"`
	Password  string        `mapstructure:"password"`
	VerifyTLS bool          `mapstructure:"verify_tls"`
	Timeout   time.Duration `mapstructure:"timeout"`
}

// SyslogConfig configures the push-based syslog receiver.
type SyslogConfig struct {
	Enabled bool   `mapstructure:"enabled"`
	UDPAddr string `mapstructure:"udp_addr"`
	TCPAddr string `mapstructure:"tcp_addr"`
	Vendor  string `mapstructure:"vendor"`
	Site    string `mapstructure:"site"`
}

// ServerConfig configures the HTTP server that exposes /metrics, /healthz, /readyz.
type ServerConfig struct {
	Addr         string        `mapstructure:"addr"`
	ReadTimeout  time.Duration `mapstructure:"read_timeout"`
	WriteTimeout time.Duration `mapstructure:"write_timeout"`
}

// LoggingConfig controls the logger.
type LoggingConfig struct {
	Level  string `mapstructure:"level"`
	Format string `mapstructure:"format"`
}

// UniFiConfig holds vendor-specific connection settings. Future vendors get
// their own sibling section (e.g. a "qnap" block) — this stays UniFi-only.
type UniFiConfig struct {
	URL                 string        `mapstructure:"url"`
	Username            string        `mapstructure:"username"`
	Password            string        `mapstructure:"password"`
	Site                string        `mapstructure:"site"`
	VerifyTLS           bool          `mapstructure:"verify_tls"`
	Timeout             time.Duration `mapstructure:"timeout"`
	MaxRetries          int           `mapstructure:"max_retries"`
	AuthFailureCooldown time.Duration `mapstructure:"auth_failure_cooldown"`
}

// LokiConfig configures the Loki push exporter.
type LokiConfig struct {
	Enabled   bool          `mapstructure:"enabled"`
	URL       string        `mapstructure:"url"`
	BatchSize int           `mapstructure:"batch_size"`
	BatchWait time.Duration `mapstructure:"batch_wait"`
	Tenant    string        `mapstructure:"tenant"` // X-Scope-OrgID, optional
}

// CollectorConfig is the per-collector runtime setting. Keyed by collector
// name in the map, so adding a collector never requires a struct change.
type CollectorConfig struct {
	Enabled  bool          `mapstructure:"enabled"`
	Interval time.Duration `mapstructure:"interval"`
}

// Load reads configuration from path (may be empty to rely on defaults + env),
// applies defaults, binds env overrides, and validates the result.
func Load(path string) (*Config, error) {
	v := viper.New()
	setDefaults(v)

	if path != "" {
		v.SetConfigFile(path)
		if err := v.ReadInConfig(); err != nil {
			return nil, fmt.Errorf("config: reading %q: %w", path, err)
		}
	}

	// Env overrides: COLLECTOR_UNIFI_PASSWORD, COLLECTOR_LOKI_URL, ...
	v.SetEnvPrefix("COLLECTOR")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("config: unmarshal: %w", err)
	}
	if err := cfg.validate(); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func setDefaults(v *viper.Viper) {
	v.SetDefault("server.addr", ":8080")
	v.SetDefault("server.read_timeout", 15*time.Second)
	v.SetDefault("server.write_timeout", 15*time.Second)

	v.SetDefault("logging.level", "info")
	v.SetDefault("logging.format", "json")

	v.SetDefault("unifi.site", "default")
	v.SetDefault("unifi.verify_tls", false)
	v.SetDefault("unifi.timeout", 10*time.Second)
	v.SetDefault("unifi.max_retries", 3)
	v.SetDefault("unifi.auth_failure_cooldown", time.Minute)

	v.SetDefault("kerio.enabled", false)
	v.SetDefault("kerio.verify_tls", false)
	v.SetDefault("kerio.timeout", 10*time.Second)

	v.SetDefault("loki.enabled", true)
	v.SetDefault("loki.batch_size", 100)
	v.SetDefault("loki.batch_wait", 5*time.Second)

	v.SetDefault("syslog.enabled", false)
	v.SetDefault("syslog.udp_addr", ":1514")
	v.SetDefault("syslog.vendor", "unifi")
	v.SetDefault("syslog.site", "default")
}

func (c *Config) validate() error {
	if c.UniFi.URL == "" {
		return fmt.Errorf("config: unifi.url is required")
	}
	if c.UniFi.Username == "" || c.UniFi.Password == "" {
		return fmt.Errorf("config: unifi.username and unifi.password are required")
	}
	if c.Kerio.Enabled {
		if c.Kerio.URL == "" {
			return fmt.Errorf("config: kerio.url is required when kerio is enabled")
		}
		if c.Kerio.Username == "" || c.Kerio.Password == "" {
			return fmt.Errorf("config: kerio.username and kerio.password are required when kerio is enabled")
		}
	}
	if c.Loki.Enabled && c.Loki.URL == "" {
		return fmt.Errorf("config: loki.url is required when loki is enabled")
	}
	for name, cc := range c.Collectors {
		if cc.Enabled && cc.Interval <= 0 {
			return fmt.Errorf("config: collector %q has invalid interval %s", name, cc.Interval)
		}
	}
	return nil
}

package config_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/murad/unifi-collector/internal/config"
)

func writeConfig(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return path
}

func TestLoad_AppliesDefaults(t *testing.T) {
	path := writeConfig(t, `
unifi:
  url: "https://ctrl"
  username: "u"
  password: "p"
loki:
  enabled: false
collectors:
  devices: { enabled: true, interval: 30s }
`)
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Server.Addr != ":8080" {
		t.Errorf("default addr = %q, want :8080", cfg.Server.Addr)
	}
	if cfg.UniFi.Timeout != 10*time.Second {
		t.Errorf("default unifi timeout = %v, want 10s", cfg.UniFi.Timeout)
	}
	if cfg.UniFi.AuthFailureCooldown != time.Minute {
		t.Errorf("default auth failure cooldown = %v, want 1m", cfg.UniFi.AuthFailureCooldown)
	}
	if cfg.UniFi.Site != "default" {
		t.Errorf("default site = %q, want default", cfg.UniFi.Site)
	}
	if !cfg.Collectors["devices"].Enabled {
		t.Error("devices collector should be enabled")
	}
}

func TestLoad_EnvOverride(t *testing.T) {
	path := writeConfig(t, `
unifi:
  url: "https://ctrl"
  username: "u"
  password: "filepw"
loki:
  enabled: false
`)
	t.Setenv("COLLECTOR_UNIFI_PASSWORD", "envpw")

	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.UniFi.Password != "envpw" {
		t.Errorf("password = %q, want envpw (env override)", cfg.UniFi.Password)
	}
}

func TestLoad_ValidationErrors(t *testing.T) {
	cases := map[string]string{
		"missing url":      "unifi:\n  username: u\n  password: p\n",
		"missing creds":    "unifi:\n  url: https://x\n",
		"loki without url": "unifi:\n  url: https://x\n  username: u\n  password: p\nloki:\n  enabled: true\n",
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := config.Load(writeConfig(t, body)); err == nil {
				t.Errorf("%s: expected validation error, got nil", name)
			}
		})
	}
}

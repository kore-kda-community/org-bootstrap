package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

const (
	// DefaultName is used when no path is provided.
	DefaultName = "config.toml"
	// EnvConfigPath lets callers override the config location.
	EnvConfigPath = "ORG_BOOTSTRAP_CONFIG"
)

// Load reads config from path. Resolution order when path is empty:
//  1. $ORG_BOOTSTRAP_CONFIG
//  2. $XDG_CONFIG_HOME/org-bootstrap/config.toml
//  3. ./config.toml
//
// Defaults are applied, then the result is validated.
func Load(path string) (*Config, error) {
	resolved := resolvePath(path)

	cfg := &Config{}
	if _, err := toml.DecodeFile(resolved, cfg); err != nil {
		if errors.Is(err, os.ErrNotExist) && resolved == DefaultName {
			return nil, fmt.Errorf(
				"no config found; copy configs/example.toml to %s or set $%s",
				DefaultName, EnvConfigPath)
		}
		return nil, fmt.Errorf("parse %s: %w", resolved, err)
	}

	ApplyDefaults(cfg)
	if err := Validate(cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

// resolvePath returns the effective config path.
func resolvePath(path string) string {
	if path != "" {
		return path
	}
	if env := os.Getenv(EnvConfigPath); env != "" {
		return env
	}
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, "org-bootstrap", DefaultName)
	}
	return DefaultName
}

// ApplyDefaults fills in zero-value fields with sensible defaults.
func ApplyDefaults(c *Config) {
	if c.General.CreateVisibility == "" {
		c.General.CreateVisibility = "private"
	}
	if len(c.General.PromoteToPublic) == 0 {
		c.General.PromoteToPublic = []string{".github", "releases", "homebrew-tap"}
	}
	if c.General.StatePath == "" {
		c.General.StatePath = ".state.json"
	}
	if c.Auth.ClientID == "" {
		c.Auth.ClientID = "Ov23li0LMhXrFhF4k8hu"
	}
	if c.Auth.Store == "" {
		c.Auth.Store = "gh-keyring"
	}
	if len(c.Auth.Scopes) == 0 {
		c.Auth.Scopes = []string{"repo", "workflow", "admin:org", "read:org"}
	}
	if c.Audit.RulesPath == "" {
		c.Audit.RulesPath = "configs/audit-default.toml"
	}
	if c.Profile.Mode == "" {
		c.Profile.Mode = "pinned"
	}
	if c.Profile.TopN == 0 {
		c.Profile.TopN = 10
	}
	if c.Tap.SecretName == "" {
		c.Tap.SecretName = "HOMEBREW_TAP_GITHUB_TOKEN"
	}
	if c.Log.Level == "" {
		c.Log.Level = "info"
	}
	if c.Log.Format == "" {
		c.Log.Format = "text"
	}
}

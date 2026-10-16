package config

import "fmt"

// Validate checks that the config is internally consistent.
func Validate(c *Config) error {
	if c.General.MainOrg == "" {
		return fmt.Errorf("general.main_org is required")
	}
	switch c.General.CreateVisibility {
	case "private", "public":
	default:
		return fmt.Errorf("general.create_visibility must be private or public, got %q",
			c.General.CreateVisibility)
	}
	switch c.Auth.Store {
	case "gh-keyring", "file":
	default:
		return fmt.Errorf("auth.store must be gh-keyring or file, got %q", c.Auth.Store)
	}
	switch c.Profile.Mode {
	case "pinned", "curated", "auto":
	default:
		return fmt.Errorf("profile.mode must be pinned, curated, or auto, got %q",
			c.Profile.Mode)
	}
	if c.Profile.TopN < 1 || c.Profile.TopN > 100 {
		return fmt.Errorf("profile.top_n must be between 1 and 100, got %d", c.Profile.TopN)
	}
	switch c.Log.Level {
	case "debug", "info", "warn", "error":
	default:
		return fmt.Errorf("log.level must be debug, info, warn, or error, got %q",
			c.Log.Level)
	}
	switch c.Log.Format {
	case "text", "json":
	default:
		return fmt.Errorf("log.format must be text or json, got %q", c.Log.Format)
	}
	return nil
}

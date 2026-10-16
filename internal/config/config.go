// Package config loads and validates org-bootstrap TOML configuration.
package config

// Config is the top-level configuration, decoded from TOML.
type Config struct {
	General General `toml:"general"`
	Auth    Auth    `toml:"auth"`
	Audit   Audit   `toml:"audit"`
	Profile Profile `toml:"profile"`
	Tap     Tap     `toml:"tap"`
	Orgs    Orgs    `toml:"orgs"`
	Log     Log     `toml:"log"`
}

// General controls global behavior.
type General struct {
	MainOrg          string   `toml:"main_org"`
	CreateVisibility string   `toml:"create_visibility"`
	PromoteToPublic  []string `toml:"promote_to_public"`
	StatePath        string   `toml:"state_path"`
}

// Auth holds OAuth device flow settings.
type Auth struct {
	ClientID string   `toml:"client_id"`
	Scopes   []string `toml:"scopes"`
	Store    string   `toml:"store"`
}

// Audit controls the secrets scanner.
type Audit struct {
	RulesPath  string `toml:"rules_path"`
	FailClosed bool   `toml:"fail_closed"`
}

// Profile controls org profile generation.
type Profile struct {
	Mode        string `toml:"mode"`
	TopN        int    `toml:"top_n"`
	ShowTeasers bool   `toml:"show_teasers"`
}

// Tap controls Homebrew tap interaction.
type Tap struct {
	SecretName string `toml:"secret_name"`
}

// Orgs holds manually-added orgs.
type Orgs struct {
	Extra []string `toml:"extra"`
}

// Log controls output verbosity and format.
type Log struct {
	Level  string `toml:"level"`
	Format string `toml:"format"`
}

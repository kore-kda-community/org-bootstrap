package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/kore-kda-community/org-bootstrap/internal/bootstrap"
	"github.com/kore-kda-community/org-bootstrap/internal/config"
	"github.com/kore-kda-community/org-bootstrap/internal/gh"
	"github.com/kore-kda-community/org-bootstrap/internal/migrate"
	"github.com/kore-kda-community/org-bootstrap/internal/oauth"
	"github.com/kore-kda-community/org-bootstrap/internal/state"
)

func main() {
	var cfgPath, statePath, migrateDir string
	flag.StringVar(&cfgPath, "config", "", "path to config.toml")
	flag.StringVar(&statePath, "state", "", "path to .state.json")
	flag.StringVar(&migrateDir, "migrate-from", "", "dir containing legacy dotfiles")
	flag.Parse()

	if err := run(cfgPath, statePath, migrateDir); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run(cfgPath, statePath, migrateDir string) error {
	cfg, err := config.Load(cfgPath)
	if err != nil {
		return err
	}

	st, err := state.Load(statePath)
	if err != nil {
		return err
	}

	// Optional one-time import of legacy dotfiles.
	if migrateDir != "" {
		legacy, err := migrate.Legacy(migrateDir)
		if err != nil {
			return fmt.Errorf("migrate: %w", err)
		}
		// Merge: legacy fills anything st hasn't got yet.
		if st.MainOrg == "" {
			st.MainOrg = legacy.MainOrg
		}
		if !st.PinsDecided() && legacy.PinsDecided() {
			st.IncludePins = legacy.IncludePins
		}
		for org, los := range legacy.Orgs {
			if _, ok := st.Orgs[org]; !ok {
				st.Orgs[org] = los
			}
		}
		fmt.Println("  ✓ imported legacy dotfiles")
	}

	ctx := context.Background()

	onPrompt := func(code, uri string) {
		fmt.Println()
		fmt.Println("  Device login")
		fmt.Printf("  Open: %s\n", uri)
		fmt.Printf("  Code: %s\n", code)
		fmt.Println()
	}
	token, err := oauth.EnsureToken(ctx, cfg.Auth, onPrompt)
	if err != nil {
		return fmt.Errorf("auth: %w", err)
	}

	// Export the token so downstream steps (bootstrap, release) can reuse
	// it as a fallback when no explicit tap token is provided.
	os.Setenv("GH_TOKEN", token)

	c, err := gh.New(ctx, token)
	if err != nil {
		return err
	}

	if err := bootstrap.Run(ctx, c, cfg, st); err != nil {
		return err
	}

	// Persist state after a successful run.
	stPath := statePath
	if stPath == "" {
		stPath = cfg.General.StatePath
	}
	return state.Save(stPath, st)
}

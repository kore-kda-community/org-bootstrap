// Command org-release is the interactive release manager.
//
// It is the Go port of release.sh: pick an org, browse repos with their
// release state, and drive releases through the alpha → beta → rc → final
// ladder — or bypass it entirely, lock main, archive old releases.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/kore-kda-community/org-bootstrap/internal/config"
	"github.com/kore-kda-community/org-bootstrap/internal/gh"
	"github.com/kore-kda-community/org-bootstrap/internal/oauth"
	"github.com/kore-kda-community/org-bootstrap/internal/state"
	"github.com/kore-kda-community/org-bootstrap/internal/tui"
)

func main() {
	var cfgPath, statePath, orgFlag string
	flag.StringVar(&cfgPath, "config", "", "path to config.toml")
	flag.StringVar(&statePath, "state", "", "path to .state.json")
	flag.StringVar(&orgFlag, "org", "", "skip the org picker and use this org")
	flag.Parse()

	if err := run(cfgPath, statePath, orgFlag); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run(cfgPath, statePath, orgFlag string) error {
	cfg, err := config.Load(cfgPath)
	if err != nil {
		return err
	}
	st, err := state.Load(statePath)
	if err != nil {
		return err
	}

	ctx := context.Background()
	token, err := oauth.EnsureToken(ctx, cfg.Auth, nil)
	if err != nil {
		return err
	}
	c, err := gh.New(ctx, token)
	if err != nil {
		return err
	}

	if orgFlag != "" {
		return repoList(ctx, c, st, cfg, orgFlag, statePath)
	}
	return orgPicker(ctx, c, st, cfg, statePath)
}

// orgPicker lists every admin org (plus saved extras) and lets the user pick
// one. Loops until the user quits.
func orgPicker(ctx context.Context, c *gh.Client, st *state.State, cfg *config.Config, statePath string) error {
	for {
		orgs, err := c.ListAdminOrgs(ctx)
		if err != nil {
			return err
		}
		names := make([]string, 0, len(orgs)+len(st.ExtraOrgs))
		seen := map[string]bool{}
		for _, o := range orgs {
			names = append(names, o.Login)
			seen[o.Login] = true
		}
		for _, e := range st.ExtraOrgs {
			if !seen[e] {
				names = append(names, e)
			}
		}
		if len(names) == 0 {
			return fmt.Errorf("no orgs found for this token")
		}

		picked, quit, err := tui.PickOrgs("── Select org ──", names)
		if err != nil {
			return err
		}
		if quit {
			return nil
		}
		if len(picked) == 0 {
			continue
		}
		// PickOrgs returns a list; take the first for single-org flow.
		if err := repoList(ctx, c, st, cfg, picked[0], statePath); err != nil {
			fmt.Fprintf(os.Stderr, "  ! %v\n", err)
			pause()
		}
	}
}

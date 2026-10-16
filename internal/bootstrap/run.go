// Package bootstrap orchestrates the interactive org setup flow.
package bootstrap

import (
	"context"
	"fmt"
	"os"

	"github.com/kore-kda-community/org-bootstrap/internal/config"
	"github.com/kore-kda-community/org-bootstrap/internal/gh"
	"github.com/kore-kda-community/org-bootstrap/internal/state"
	"github.com/kore-kda-community/org-bootstrap/internal/tui"
)

// Run walks the user through org selection, curation, and execution.
// Caller is responsible for auth (oauth.EnsureToken) before calling Run.
func Run(ctx context.Context, c *gh.Client, cfg *config.Config, st *state.State) error {
	// 1. List admin orgs.
	fmt.Println()
	fmt.Println("── Organizations ──")
	orgs, err := c.ListAdminOrgs(ctx)
	if err != nil {
		return fmt.Errorf("list orgs: %w", err)
	}
	if len(orgs) == 0 {
		return fmt.Errorf("no admin orgs found for this token")
	}

	names := make([]string, len(orgs))
	for i, o := range orgs {
		names[i] = o.Login
	}

	// 2. Pick orgs.
	fmt.Println()
	for i, n := range names {
		fmt.Printf("    [%2d] %s\n", i+1, n)
	}
	fmt.Println()
	choice, err := tui.PromptChoice("Process all, or pick numbers", "a", "a", "s", "q")
	if err != nil {
		return err
	}
	switch choice {
	case "q":
		return nil
	case "a":
		// keep all
	case "s":
		picks, err := tui.PickByNumber("", names)
		if err != nil {
			return err
		}
		names = picks
	}
	if len(names) == 0 {
		return fmt.Errorf("no orgs selected")
	}
	fmt.Printf("  ✓ will process %d org(s): %v\n", len(names), names)

	// 3. Main org (asked once).
	if st.MainOrg == "" {
		fmt.Println()
		fmt.Println("── Shared tooling org ──")
		main, err := tui.PromptString("Main org (hosts homebrew-tap-app)", cfg.General.MainOrg)
		if err != nil {
			return err
		}
		st.MainOrg = main
	}

	// 4. Include pins (asked once).
	if !st.PinsDecided() {
		fmt.Println()
		yes, err := tui.PromptYN("Include GitHub-pinned repos in profiles?", true)
		if err != nil {
			return err
		}
		if yes {
			st.IncludePins = "y"
		} else {
			st.IncludePins = "n"
		}
	}

	// 5. Per-org curation.
	for _, org := range names {
		if err := curate(ctx, c, st, org); err != nil {
			return err
		}
	}

	// 6. Tap token.
	fmt.Println()
	fmt.Println("── Homebrew tap token ──")
	token, err := tui.PromptSecret("Homebrew tap token")
	if err != nil {
		return err
	}
	if token == "" {
		return fmt.Errorf("tap token is required")
	}

	// 7. Confirm.
	fmt.Println()
	yes, err := tui.PromptYN(fmt.Sprintf("Proceed with %d org(s)?", len(names)), true)
	if err != nil {
		return err
	}
	if !yes {
		return nil
	}

	// 8. Execute per org.
	for _, org := range names {
		if err := executeOrg(ctx, c, cfg, st, org, token); err != nil {
			fmt.Fprintf(os.Stderr, "  ! org %s: %v\n", org, err)
		}
	}

	fmt.Println()
	fmt.Println("── All done ──")
	return nil
}

// curate prompts for public repo picks and teasers for one org, saving into
// state. Prompts are skipped if the org already has curation.
func curate(ctx context.Context, c *gh.Client, st *state.State, org string) error {
	os := st.Get(org)

	// Public picks
	fmt.Println()
	fmt.Println("── Public repos for", org, "──")
	edit, err := tui.PromptYN("Cherry-pick public repos for the profile?", false)
	if err != nil {
		return err
	}
	if edit {
		repos, err := c.ListPublicRepos(ctx, org)
		if err != nil {
			return err
		}
		names := make([]string, len(repos))
		for i, r := range repos {
			names[i] = r.GetName()
		}
		picked, err := tui.PickByNumber("", names)
		if err != nil {
			return err
		}
		os.Profile.CuratedPublic = picked
	}

	// Teasers
	fmt.Println()
	fmt.Println("── In Development teasers for", org, "──")
	privs, err := c.ListPrivateRepoNames(ctx, org)
	if err != nil {
		return err
	}
	if len(privs) > 0 {
		tease, err := tui.PromptYN("Add private repos to the In Development section?", false)
		if err != nil {
			return err
		}
		if tease {
			picked, err := tui.PickByNumber("", privs)
			if err != nil {
				return err
			}
			// Replace semantics
			os.Profile.Teasers = nil
			for _, name := range picked {
				slug, err := tui.PromptString(fmt.Sprintf("    %s slug (blank = repo name)", name), "")
				if err != nil {
					return err
				}
				os.Profile.Teasers = append(os.Profile.Teasers, state.Teaser{Repo: name, Slug: slug})
			}
		}
	}

	st.Put(org, os)
	return nil
}

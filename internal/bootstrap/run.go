// Package bootstrap orchestrates the interactive org setup flow.
package bootstrap

import (
	"context"
	"fmt"
	"os"
	"strings"

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
	picked, quit, err := tui.PickOrgs("── Organizations ──", names)
	if err != nil {
		return err
	}
	if quit {
		return nil
	}
	if len(picked) == 0 {
		return fmt.Errorf("no orgs selected")
	}
	names = picked
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

	// 6. Tap token. Silent lookup in priority order. Never prompts unless
	// there's genuinely nothing available — which shouldn't happen because
	// main.go exports GH_TOKEN after auth.
	token := lookupStoredTapToken(st)
	if token == "" {
		token = os.Getenv("HOMEBREW_TAP_GITHUB_TOKEN")
	}
	if token == "" {
		token = os.Getenv("GH_TOKEN")
	}
	if token == "" {
		token = os.Getenv("GITHUB_TOKEN")
	}
	if token == "" {
		return fmt.Errorf("no tap token available — set HOMEBREW_TAP_GITHUB_TOKEN or run `gh auth login`")
	}
	fmt.Println("  ✓ tap token ready")

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

// curate runs the interactive teaser/pick editor for one org. Mirrors
// bootstrap.sh's edit_teasers_for_org: shows existing picks with [+] marks,
// supports "-" to clear, preserves slugs on Enter, and applies replace
// semantics.
func curate(ctx context.Context, c *gh.Client, st *state.State, org string) error {
	os := st.Get(org)

	// ---- Public repo cherry-pick (optional) ----
	fmt.Println()
	fmt.Println("── Public repos for " + org + " ──")
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

	// ---- Teasers ----
	privs, err := c.ListPrivateRepoNames(ctx, org)
	if err != nil {
		return err
	}
	if len(privs) == 0 {
		st.Put(org, os)
		return nil
	}

	// Build the current set: repo name -> slug.
	current := map[string]string{}
	for _, t := range os.Profile.Teasers {
		current[t.Repo] = t.Slug
	}

	fmt.Println()
	fmt.Println("── In Development teasers for " + org + " ──")
	fmt.Println("  Private repos you can tease in the org profile.")
	fmt.Println("  Shown as name or custom slug only — no links, no descriptions.")
	fmt.Println()
	for i, name := range privs {
		mark := " "
		if _, ok := current[name]; ok {
			mark = "\033[32m+\033[0m"
		}
		fmt.Printf("    [%2d] %s %s\n", i+1, mark, name)
	}
	fmt.Println()
	fmt.Println("  Numbers to add (e.g. '1 3 5'), - to clear all, Enter to keep current.")

	line, err := tui.PromptString("  Selection", "")
	if err != nil {
		return err
	}
	line = strings.TrimSpace(line)

	// Empty → keep current set unchanged.
	if line == "" {
		st.Put(org, os)
		return nil
	}

	// "-" → clear all teasers for this org.
	if line == "-" {
		os.Profile.Teasers = nil
		st.Put(org, os)
		fmt.Printf("  ✓ cleared %s teasers\n", org)
		return nil
	}

	// Otherwise parse numbers and build the replacement set.
	nums, err := tui.ParseNumericPick(line)
	if err != nil {
		return err
	}
	var picked []string
	for _, n := range nums {
		if n >= 1 && n <= len(privs) {
			picked = append(picked, privs[n-1])
		}
	}
	if len(picked) == 0 {
		fmt.Println("  ! no valid picks — keeping current set")
		st.Put(org, os)
		return nil
	}

	// Prompt for slugs, preserving any existing slug on Enter.
	os.Profile.Teasers = nil
	for _, name := range picked {
		existing := current[name]
		hint := ""
		if existing != "" && existing != name {
			hint = fmt.Sprintf(" (current: %s)", existing)
		}
		slug, err := tui.PromptString(fmt.Sprintf("    %s slug%s", name, hint), existing)
		if err != nil {
			return err
		}
		os.Profile.Teasers = append(os.Profile.Teasers, state.Teaser{Repo: name, Slug: slug})
	}

	st.Put(org, os)
	fmt.Printf("  ✓ %s: %d teaser(s) selected (replaced)\n", org, len(picked))
	return nil
}

// lookupStoredTapToken returns any previously saved tap token from state.
// Processes multiple orgs in one run, so this returns the first one found.
func lookupStoredTapToken(st *state.State) string {
	for _, o := range st.Orgs {
		if o.TapToken != "" {
			return o.TapToken
		}
	}
	return ""
}

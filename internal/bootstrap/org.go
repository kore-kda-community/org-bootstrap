package bootstrap

import (
	"context"
	"fmt"
	"strings"

	"github.com/kore-kda-community/org-bootstrap/internal/config"
	"github.com/kore-kda-community/org-bootstrap/internal/gh"
	"github.com/kore-kda-community/org-bootstrap/internal/profile"
	"github.com/kore-kda-community/org-bootstrap/internal/repo"
	"github.com/kore-kda-community/org-bootstrap/internal/state"
	"github.com/kore-kda-community/org-bootstrap/internal/tap"
	"github.com/kore-kda-community/org-bootstrap/internal/visibility"
)

// executeOrg runs the four-step setup for one org.
func executeOrg(ctx context.Context, c *gh.Client, cfg *config.Config, st *state.State, org, tapToken string) error {
	fmt.Println()
	fmt.Println("── Org:", org, "──")
	os := st.Get(org)

	// [1/4] Org profile
	if err := doProfile(ctx, c, cfg, st, org, os); err != nil {
		return fmt.Errorf("[1/4] profile: %w", err)
	}

	// [2/4] Releases hub
	if err := doReleases(ctx, c, cfg, org); err != nil {
		return fmt.Errorf("[2/4] releases: %w", err)
	}

	// [3/4] Homebrew tap
	if err := doTap(ctx, c, cfg, org); err != nil {
		return fmt.Errorf("[3/4] tap: %w", err)
	}

	// [4/4] Source repo
	if err := doSource(ctx, c, cfg, st, org, tapToken); err != nil {
		return fmt.Errorf("[4/4] source: %w", err)
	}

	// Update managed list
	os.ManagedRepos = appendUnique(os.ManagedRepos, sourceRepoName(org))
	st.Put(org, os)

	fmt.Println("  ✓ org", org, "complete")
	return nil
}

func doProfile(ctx context.Context, c *gh.Client, cfg *config.Config, st *state.State, org string, os state.OrgState) error {
	fmt.Println("[1/4] org profile")
	created, err := repo.Ensure(ctx, c, org, repo.DotGithub, cfg.General.CreateVisibility, "Organization profile for "+org)
	if err != nil {
		return err
	}
	if created {
		fmt.Printf("  ✓ created %s/%s (%s)\n", org, repo.DotGithub, cfg.General.CreateVisibility)
	} else {
		fmt.Printf("  ✓ %s/%s already exists\n", org, repo.DotGithub)
	}

	content, err := profile.Render(ctx, c, profile.Inputs{
		Org:   org,
		State: os,
		Opts: profile.Options{
			MainOrg:     st.MainOrg,
			TopN:        cfg.Profile.TopN,
			IncludePins: st.PinsEnabled(),
			ShowTeasers: cfg.Profile.ShowTeasers,
		},
	})
	if err != nil {
		return err
	}
	changed, err := repo.PushFile(ctx, c, org, repo.DotGithub, repo.ProfileReadmePath,
		"Update org profile README", content)
	if err != nil {
		return err
	}
	if changed {
		fmt.Printf("  ✓ %s/%s:%s\n", org, repo.DotGithub, repo.ProfileReadmePath)
	} else {
		fmt.Printf("  ✓ %s/%s:%s (unchanged)\n", org, repo.DotGithub, repo.ProfileReadmePath)
	}

	// Promote to public via audit gate
	return promote(ctx, c, cfg, org, repo.DotGithub)
}

func doReleases(ctx context.Context, c *gh.Client, cfg *config.Config, org string) error {
	fmt.Println("[2/4] releases hub")
	created, err := repo.Ensure(ctx, c, org, repo.Releases, cfg.General.CreateVisibility, "Binary releases for "+org)
	if err != nil {
		return err
	}
	if created {
		fmt.Printf("  ✓ created %s/%s (%s)\n", org, repo.Releases, cfg.General.CreateVisibility)
	} else {
		fmt.Printf("  ✓ %s/%s already exists\n", org, repo.Releases)
	}

	readme := fmt.Sprintf("# Releases — %s\n\nEvery tagged build from every project in this org lands here.\n\n## Download\n\n    https://github.com/%s/releases\n", org, org)
	changed, err := repo.PushFile(ctx, c, org, repo.Releases, repo.ReleasesReadme,
		"Add releases README", []byte(readme))
	if err != nil {
		return err
	}
	if changed {
		fmt.Printf("  ✓ %s/%s:%s\n", org, repo.Releases, repo.ReleasesReadme)
	} else {
		fmt.Printf("  ✓ %s/%s:%s (unchanged)\n", org, repo.Releases, repo.ReleasesReadme)
	}

	return promote(ctx, c, cfg, org, repo.Releases)
}

func doTap(ctx context.Context, c *gh.Client, cfg *config.Config, org string) error {
	fmt.Println("[3/4] homebrew tap")
	created, err := repo.Ensure(ctx, c, org, repo.HomebrewTap, cfg.General.CreateVisibility, "Homebrew tap for "+org)
	if err != nil {
		return err
	}
	if created {
		fmt.Printf("  ✓ created %s/%s (%s)\n", org, repo.HomebrewTap, cfg.General.CreateVisibility)
	} else {
		fmt.Printf("  ✓ %s/%s already exists\n", org, repo.HomebrewTap)
	}

	readme := fmt.Sprintf("# Homebrew Tap — %s\n\nFormulae live under `Formula/`. They are updated automatically by GoReleaser.\n", org)
	changed, err := repo.PushFile(ctx, c, org, repo.HomebrewTap, repo.TapReadme,
		"Add tap README", []byte(readme))
	if err != nil {
		return err
	}
	if changed {
		fmt.Printf("  ✓ %s/%s:%s\n", org, repo.HomebrewTap, repo.TapReadme)
	} else {
		fmt.Printf("  ✓ %s/%s:%s (unchanged)\n", org, repo.HomebrewTap, repo.TapReadme)
	}

	formulae, err := tap.ListFormulae(ctx, c, org)
	if err == nil && len(formulae) > 0 {
		fmt.Printf("  ✓ %d formula(e): %s\n", len(formulae), strings.Join(formulae, ", "))
	}

	return promote(ctx, c, cfg, org, repo.HomebrewTap)
}

func doSource(ctx context.Context, c *gh.Client, cfg *config.Config, st *state.State, org, tapToken string) error {
	fmt.Println("[4/4] source repo")
	name := sourceRepoName(org)
	created, err := repo.Ensure(ctx, c, org, name, cfg.General.CreateVisibility, "Source repo for "+org)
	if err != nil {
		return err
	}
	if created {
		fmt.Printf("  ✓ created %s/%s (private)\n", org, name)
	} else {
		fmt.Printf("  ✓ %s/%s already exists\n", org, name)
	}

	// Write .goreleaser.yml and release workflow
	gr := renderGoReleaser(org, name)
	changed, err := repo.PushFile(ctx, c, org, name, repo.SourceGoReleaser,
		"Add GoReleaser config", []byte(gr))
	if err != nil {
		return err
	}
	if changed {
		fmt.Printf("  ✓ %s/%s:%s\n", org, name, repo.SourceGoReleaser)
	} else {
		fmt.Printf("  ✓ %s/%s:%s (unchanged)\n", org, name, repo.SourceGoReleaser)
	}

	wf := renderWorkflow()
	changed, err = repo.PushFile(ctx, c, org, name, repo.SourceWorkflow,
		"Add release workflow", []byte(wf))
	if err != nil {
		return err
	}
	if changed {
		fmt.Printf("  ✓ %s/%s:%s\n", org, name, repo.SourceWorkflow)
	} else {
		fmt.Printf("  ✓ %s/%s:%s (unchanged)\n", org, name, repo.SourceWorkflow)
	}

	// Set the tap token as an Actions secret
	secretName := cfg.Tap.SecretName
	if secretName == "" {
		secretName = "HOMEBREW_TAP_GITHUB_TOKEN"
	}
	if err := repo.SetSecret(ctx, c, org, name, secretName, tapToken); err != nil {
		return err
	}
	fmt.Printf("  ✓ secret %s on %s/%s\n", secretName, org, name)

	return nil
}

// promote runs the audit gate and flips the repo to public if clean.
func promote(ctx context.Context, c *gh.Client, cfg *config.Config, org, name string) error {
	res, err := visibility.Promote(ctx, c, cfg.Audit, org, name, "")
	if err != nil {
		return err
	}
	if res.Skipped {
		return nil
	}
	if res.Refused {
		fmt.Printf("  ✗ audit found %d issue(s) — repo stays private\n", len(res.Audit.Findings))
		for _, f := range res.Audit.Findings {
			fmt.Printf("      %s:%d %s\n", f.File, f.Line, f.RuleID)
		}
		return fmt.Errorf("audit blocked promotion")
	}
	if res.Promoted {
		fmt.Printf("  ✓ audit clean: %s/%s\n", org, name)
		fmt.Printf("  ✓ promoted %s/%s → public\n", org, name)
	}
	return nil
}

// sourceRepoName returns the auto-derived source repo name for an org.
func sourceRepoName(org string) string {
	return strings.ToLower(org)
}

// appendUnique adds s to list if not present.
func appendUnique(list []string, s string) []string {
	for _, v := range list {
		if v == s {
			return list
		}
	}
	return append(list, s)
}

package bootstrap

import (
	"context"
	"fmt"
	"strings"

	"github.com/kore-kda-community/org-bootstrap/internal/config"
	"github.com/kore-kda-community/org-bootstrap/internal/gh"
	"github.com/kore-kda-community/org-bootstrap/internal/repo"
	"github.com/kore-kda-community/org-bootstrap/internal/tui"
)

// tapChoice describes the outcome of the legacy-tap prompt.
type tapChoice int

const (
	tapUseCanonical tapChoice = iota // use/create homebrew-tap
	tapReuseLegacy                   // use existing homebrew-<project>
	tapSkip                          // don't create/manage a tap
)

// ensureTap handles the "[3/4] homebrew tap" step, including legacy
// detection. Returns the repo name to use ("" means skip), and whether it
// was freshly created.
func ensureTap(ctx context.Context, c *gh.Client, cfg *config.Config, org string) (repoName string, created bool, err error) {
	// 1. Canonical already exists? Just use it.
	if exists, _ := c.RepoExists(ctx, org, repo.HomebrewTap); exists {
		fmt.Printf("  ✓ %s/%s already exists — reusing it\n", org, repo.HomebrewTap)
		return repo.HomebrewTap, false, nil
	}

	// 2. Look for a legacy tap: any repo matching homebrew-* except homebrew-tap.
	legacy := findLegacyTap(ctx, c, org)
	if legacy == "" {
		// 3. Nothing exists. Create fresh.
		ok, err := repo.Ensure(ctx, c, org, repo.HomebrewTap,
			cfg.General.CreateVisibility, "Homebrew tap for "+org)
		if err != nil {
			return "", false, err
		}
		if ok {
			fmt.Printf("  ✓ created %s/%s (%s)\n", org, repo.HomebrewTap, cfg.General.CreateVisibility)
		}
		return repo.HomebrewTap, true, nil
	}

	// 4. Legacy found — ask what to do.
	fmt.Println()
	fmt.Printf("  %s! found legacy tap: %s/%s%s\n", "\033[33m", org, legacy, "\033[0m")
	fmt.Println("    The canonical layout uses a single 'homebrew-tap' per org.")
	fmt.Println("    Options:")
	fmt.Println("      [r] rename " + legacy + " → homebrew-tap (GitHub auto-redirects)")
	fmt.Println("      [u] use the existing " + legacy + " (no new repo)")
	fmt.Println("      [m] migrate: create homebrew-tap, copy formulae over, archive old")
	fmt.Println("      [n] ignore it and create homebrew-tap fresh")
	fmt.Println("      [s] skip homebrew-tap for this org entirely")

	choice, err := tui.PromptChoice("  Choice", "u", "r", "u", "m", "n", "s")
	if err != nil {
		return "", false, err
	}

	switch choice {
	case "r":
		if err := renameRepo(ctx, c, org, legacy, repo.HomebrewTap); err != nil {
			fmt.Printf("  ! rename failed: %v\n", err)
			fmt.Printf("  falling back to using %s/%s\n", org, legacy)
			return legacy, false, nil
		}
		fmt.Printf("  ✓ renamed — GitHub redirects %s/%s → %s/%s\n",
			org, legacy, org, repo.HomebrewTap)
		fmt.Printf("  archive the redirect when ready:\n")
		fmt.Printf("    gh repo archive %s/%s --yes\n", org, legacy)
		return repo.HomebrewTap, true, nil

	case "u":
		fmt.Printf("  ✓ using existing %s/%s\n", org, legacy)
		return legacy, false, nil

	case "m":
		fmt.Printf("  → creating homebrew-tap and copying contents of %s\n", legacy)
		if _, err := repo.Ensure(ctx, c, org, repo.HomebrewTap,
			cfg.General.CreateVisibility, "Homebrew tap for "+org); err != nil {
			return "", false, err
		}
		// Best-effort content migration via gh api.
		if err := migrateRepoContents(ctx, c, org, legacy, repo.HomebrewTap); err != nil {
			fmt.Printf("  ! migration produced no changes or failed: %v\n", err)
		} else {
			fmt.Printf("  ✓ migrated contents from %s\n", legacy)
		}
		fmt.Printf("  → archive %s when ready:\n", legacy)
		fmt.Printf("    gh repo archive %s/%s --yes\n", org, legacy)
		return repo.HomebrewTap, true, nil

	case "n":
		if _, err := repo.Ensure(ctx, c, org, repo.HomebrewTap,
			cfg.General.CreateVisibility, "Homebrew tap for "+org); err != nil {
			return "", false, err
		}
		return repo.HomebrewTap, true, nil

	case "s":
		fmt.Printf("  ! skipping homebrew-tap for %s\n", org)
		return "", false, nil
	}

	return repo.HomebrewTap, false, nil
}

// findLegacyTap returns the name of a homebrew-<project> repo, or "" if none.
func findLegacyTap(ctx context.Context, c *gh.Client, org string) string {
	repos, err := c.ListOrgRepos(ctx, org)
	if err != nil {
		return ""
	}
	for _, r := range repos {
		name := r.GetName()
		if name == repo.HomebrewTap {
			continue
		}
		if strings.HasPrefix(name, "homebrew-") {
			return name
		}
	}
	return ""
}

// renameRepo changes a repo's name via the API.
func renameRepo(ctx context.Context, c *gh.Client, org, from, to string) error {
	return c.RenameRepo(ctx, org, from, to)
}

// migrateRepoContents copies files from srcRepo to dstRepo via the Contents
// API. Best-effort: any error on a single file is logged and skipped.
func migrateRepoContents(ctx context.Context, c *gh.Client, org, srcRepo, dstRepo string) error {
	files, err := c.ListRepoFiles(ctx, org, srcRepo, "")
	if err != nil {
		return err
	}
	migrated := 0
	for _, f := range files {
		content, err := c.GetFileContent(ctx, org, srcRepo, f)
		if err != nil {
			continue
		}
		if _, err := c.PushFile(ctx, org, dstRepo, f, content,
			"Migrate from "+srcRepo); err != nil {
			continue
		}
		migrated++
	}
	if migrated == 0 {
		return fmt.Errorf("no files migrated")
	}
	return nil
}

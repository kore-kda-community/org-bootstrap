package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/kore-kda-community/org-bootstrap/internal/gh"
	"github.com/kore-kda-community/org-bootstrap/internal/release"
)

// inspectRepo prints branches, recent tags, secrets, and recent workflow runs.
func inspectRepo(ctx context.Context, c *gh.Client, org, name string) {
	hdr("Inspect: %s/%s", org, name)

	fmt.Printf("  %sBranches%s\n", cBold, cReset)
	branches, err := c.ListBranches(ctx, org, name)
	if err != nil {
		errf("list branches: %v", err)
	} else if len(branches) == 0 {
		fmt.Println("    (none)")
	} else {
		for _, b := range branches {
			parsed := release.ParseBranch(b)
			mark := ""
			if parsed.IsRelease() {
				mark = " " + cMag + "(release)" + cReset
			} else if parsed.IsDev() {
				mark = " " + cCyan + "(dev)" + cReset
			}
			fmt.Printf("    %-40s%s\n", b, mark)
		}
	}

	fmt.Println()
	fmt.Printf("  %sRecent tags%s\n", cBold, cReset)
	tags, err := c.ListTags(ctx, org, name)
	if err != nil {
		errf("list tags: %v", err)
	} else if len(tags) == 0 {
		fmt.Println("    (none)")
	} else {
		limit := 10
		if len(tags) < limit {
			limit = len(tags)
		}
		for _, t := range tags[:limit] {
			fmt.Printf("    %s\n", t)
		}
	}

	fmt.Println()
	fmt.Printf("  %sSecrets%s\n", cBold, cReset)
	secrets, err := c.ListActionsSecrets(ctx, org, name)
	if err != nil {
		fmt.Println("    (unavailable — token needs admin:repo_hook?)")
	} else if len(secrets) == 0 {
		fmt.Println("    (none)")
	} else {
		for _, s := range secrets {
			fmt.Printf("    %s\n", s)
		}
	}

	fmt.Println()
	fmt.Printf("  %sRecent runs%s\n", cBold, cReset)
	runs, err := c.ListWorkflowRuns(ctx, org, name, 5)
	if err != nil {
		fmt.Println("    (unavailable)")
	} else if len(runs) == 0 {
		fmt.Println("    (none)")
	} else {
		for _, r := range runs {
			fmt.Printf("    %s\n", r)
		}
	}
}

// readinessCheck runs the same list of checks release.sh does.
func readinessCheck(ctx context.Context, c *gh.Client, org, name string) {
	hdr("Readiness: %s/%s", org, name)

	pass, fail := 0, 0
	chk := func(label string, ok bool) {
		if ok {
			okPrint(label)
			pass++
		} else {
			errf("%s", label)
			fail++
		}
	}

	// Branch
	branch := promptLine("Branch to check", "main")

	// Files on the branch
	gor, _ := c.FileExists(ctx, org, name, ".goreleaser.yml")
	if !gor {
		gor, _ = c.FileExists(ctx, org, name, ".goreleaser.yaml")
	}
	chk(".goreleaser.yml on repo", gor)

	wf, _ := c.FileExists(ctx, org, name, ".github/workflows/release.yml")
	chk(".github/workflows/release.yml", wf)

	gomod, _ := c.FileExists(ctx, org, name, "go.mod")
	chk("go.mod present", gomod)

	// Secret
	secrets, _ := c.ListActionsSecrets(ctx, org, name)
	hasSecret := false
	for _, s := range secrets {
		if s == "HOMEBREW_TAP_GITHUB_TOKEN" {
			hasSecret = true
			break
		}
	}
	chk("secret HOMEBREW_TAP_GITHUB_TOKEN", hasSecret)

	// Target repos
	for _, target := range []string{"releases", "homebrew-tap"} {
		info, err := c.GetRepo(ctx, org, target)
		if err != nil {
			errf("target %s/%s missing", org, target)
			fail++
			continue
		}
		if info.GetVisibility() == "public" {
			okPrint("target " + org + "/" + target + " (public)")
			pass++
		} else {
			warn("target %s/%s is %s", org, target, info.GetVisibility())
			fail++
		}
	}

	// Branch shape
	parsed := release.ParseBranch(branch)
	if parsed.IsRelease() {
		okPrint(fmt.Sprintf("branch %q is a release branch (version %s)", branch, parsed.Version))
		pass++
	} else {
		warn("branch %q is not a release branch", branch)
	}

	fmt.Println()
	if fail == 0 {
		fmt.Printf("  %s%sReady to release%s  (%d checks passed)\n", cBold, cGreen, cReset, pass)
	} else {
		fmt.Printf("  %s%sNot ready%s  (%d ok, %d failed)\n", cBold, cYellow, cReset, pass, fail)
	}
}

// okPrint is like ok() but with a plain string argument.
func okPrint(label string) {
	fmt.Printf("  %s✓%s %s\n", cGreen, cReset, label)
}

// ensureStrings is a guard so unused-import errors don't kill the build if
// we later drop the only strings use.
var _ = strings.TrimSpace

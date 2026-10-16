package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/kore-kda-community/org-bootstrap/internal/gh"
	"github.com/kore-kda-community/org-bootstrap/internal/release"
	"github.com/kore-kda-community/org-bootstrap/internal/state"
)

// triggerRelease pushes a stage tag on the repo's current release branch.
func triggerRelease(ctx context.Context, c *gh.Client, org, name string, stage release.Stage) error {
	sts, err := release.Scan(ctx, c, org, name)
	if err != nil {
		return err
	}
	if sts.CurrentBranch == nil {
		return fmt.Errorf("no release branch — create one first")
	}
	if !sts.CurrentStage.Allows(stage) {
		return fmt.Errorf("cannot trigger %s from stage %s", stage, sts.CurrentStage)
	}

	version := sts.CurrentVer
	tag := release.TagForVersion(version, stage)
	branch := sts.CurrentBranch.Name

	fmt.Printf("  version:  %s\n", version)
	fmt.Printf("  stage:    %s\n", stage)
	fmt.Printf("  tag:      %s\n", tag)
	fmt.Printf("  branch:   %s\n", branch)
	fmt.Println()

	exists, _ := c.TagExists(ctx, org, name, tag)
	if exists {
		warn("tag %s already exists", tag)
		if !promptYN(fmt.Sprintf("Move it to the tip of %s?", branch), true) {
			return nil
		}
		if err := c.DeleteTag(ctx, org, name, tag); err != nil {
			return fmt.Errorf("delete tag: %w", err)
		}
	}

	if !promptYN(fmt.Sprintf("Push %s on %s?", tag, branch), true) {
		return nil
	}

	sha, err := c.GetRefSHA(ctx, org, name, "heads/"+branch)
	if err != nil {
		return fmt.Errorf("get branch SHA: %w", err)
	}

	if err := c.CreateTag(ctx, org, name, tag, sha); err != nil {
		return fmt.Errorf("create tag: %w", err)
	}

	ok("pushed %s on %s", tag, branch)
	fmt.Printf("  workflow will trigger from tag %s\n", tag)
	fmt.Printf("  watch: https://github.com/%s/%s/actions\n", org, name)
	return nil
}

// bypassRelease pushes a manual version tag, skipping stage checks.
func bypassRelease(ctx context.Context, c *gh.Client, org, name string, sts *release.RepoStatus) {
	hdr("Bypass release: %s/%s", org, name)
	fmt.Println("  manual version tag — stage checks skipped")
	fmt.Println()

	branch := "main"
	if sts.CurrentBranch != nil {
		branch = sts.CurrentBranch.Name
	}

	tag := promptLine("  Version tag (e.g. v5.4.1-alpha)", "")
	if tag == "" {
		return
	}
	branch = promptLine("  Branch to tag", branch)

	exists, _ := c.TagExists(ctx, org, name, tag)
	if exists {
		errf("tag %s already exists", tag)
		if !promptYN("Delete and recreate?", false) {
			return
		}
		if err := c.DeleteTag(ctx, org, name, tag); err != nil {
			errf("delete: %v", err)
			return
		}
	}

	if !promptYN(fmt.Sprintf("Push %s on %s?", tag, branch), true) {
		return
	}

	sha, err := c.GetRefSHA(ctx, org, name, "heads/"+branch)
	if err != nil {
		errf("get SHA: %v", err)
		return
	}

	if err := c.CreateTag(ctx, org, name, tag, sha); err != nil {
		errf("create tag: %v", err)
		return
	}
	ok("pushed %s on %s", tag, branch)
}

// keep strings import alive if unused elsewhere
var _ = strings.TrimSpace
var _ *state.State

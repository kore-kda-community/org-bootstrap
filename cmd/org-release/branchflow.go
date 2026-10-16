package main

import (
	"context"
	"fmt"

	"github.com/kore-kda-community/org-bootstrap/internal/gh"
	"github.com/kore-kda-community/org-bootstrap/internal/release"
	"github.com/kore-kda-community/org-bootstrap/internal/state"
)

// createReleaseBranchFlow walks the user through creating a release branch
// (and optionally a matching dev branch) for the repo.
func createReleaseBranchFlow(ctx context.Context, c *gh.Client, st *state.State, org, name string) error {
	hdr("Create release branch: %s/%s", org, name)
	fmt.Printf("  %srelease branch format: release_<prefix>_<x.y.z>%s\n", cDim, cReset)
	fmt.Println()

	sts, err := release.Scan(ctx, c, org, name)
	if err != nil {
		return err
	}

	// Work out prefix and suggested version.
	var prefix, suggested, source string

	if sts.CurrentBranch != nil {
		prefix = sts.CurrentBranch.Prefix
		fmt.Printf("  previous release branch: %s  (version %s, stage %s)\n",
			sts.CurrentBranch.Name, sts.CurrentVer, sts.CurrentStage)

		next, _ := release.NextVersion(sts.CurrentVer, release.BumpPatch)
		suggested = next
		source = sts.CurrentBranch.Name
		fmt.Printf("  suggested next version:  %s\n", suggested)
	} else {
		prefix = name
		suggested = "0.1.0"
		if sts.DefaultBranch != "" {
			source = sts.DefaultBranch
		} else {
			source = "main"
		}
	}
	fmt.Println()

	// Let the user adjust everything.
	prefix = promptLine("  Project prefix", prefix)
	if prefix == "" {
		return fmt.Errorf("prefix is required")
	}
	version := promptLine("  Version (x.y.z)", suggested)
	if version == "" {
		return fmt.Errorf("version is required")
	}
	// Strip a leading v for cleanliness.
	if len(version) > 0 && version[0] == 'v' {
		version = version[1:]
	}

	// Source branch — offer existing branches.
	branches, _ := c.ListBranches(ctx, org, name)
	if len(branches) > 0 {
		fmt.Println()
		fmt.Printf("  %sBranches on %s/%s%s\n", cBold, org, name, cReset)
		for i, b := range branches {
			fmt.Printf("    [%2d] %s\n", i+1, b)
		}
		fmt.Println()
	}
	source = promptLine("  Source branch", source)
	if source == "" {
		return fmt.Errorf("source branch is required")
	}

	newBranch := release.MakeReleaseName(prefix, version)
	fmt.Println()
	fmt.Printf("  new release branch: %s%s%s\n", cBold, newBranch, cReset)
	fmt.Printf("  source:             %s\n", source)

	if !promptYN(fmt.Sprintf("Create %s from %s?", newBranch, source), true) {
		return nil
	}

	sha, err := c.GetRefSHA(ctx, org, name, "heads/"+source)
	if err != nil {
		return fmt.Errorf("get source SHA: %w", err)
	}

	if err := c.CreateBranch(ctx, org, name, newBranch, sha); err != nil {
		return fmt.Errorf("create branch: %w", err)
	}
	ok("created %s from %s", newBranch, source)

	// Optionally create the matching dev branch.
	devName := release.MakeDevName(prefix, version)
	if promptYN(fmt.Sprintf("Also create dev branch %s?", devName), true) {
		if err := c.CreateBranch(ctx, org, name, devName, sha); err != nil {
			errf("create dev branch: %v", err)
		} else {
			ok("created %s", devName)
		}
	}

	fmt.Println()
	fmt.Printf("  %sdev branches merge into %s; tag %s to release%s\n",
		cDim, newBranch, newBranch, cReset)
	return nil
}

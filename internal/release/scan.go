package release

import (
	"context"
	"fmt"
	"sort"

	gh "github.com/kore-kda-community/org-bootstrap/internal/gh"
)

// BranchStatus captures the release state of one branch.
type BranchStatus struct {
	Branch  Branch
	Stage   Stage  // highest stage tag on this branch
	Tag     string // exact tag achieving Stage ("" if none)
	Version string // version extracted from branch name
}

// RepoStatus is the full release picture for one repo.
type RepoStatus struct {
	Org           string
	Name          string
	DefaultBranch string
	Branches      []Branch
	Tags          []string

	// Current release: highest-versioned release branch, if any.
	CurrentBranch *Branch
	CurrentStage  Stage
	CurrentTag    string // e.g. "v5.4.1-rc"
	CurrentVer    string // e.g. "5.4.1"

	// Suggested next versions per bump, computed against CurrentVer.
	NextPatch string
	NextMinor string
	NextMajor string

	HasGoReleaser  bool
	HasWorkflow    bool
	HasVersionFile bool
}

// Scan inspects one repo and returns its full release picture.
func Scan(ctx context.Context, c *gh.Client, org, name string) (*RepoStatus, error) {
	st := &RepoStatus{Org: org, Name: name}

	info, err := c.GetRepo(ctx, org, name)
	if err != nil {
		return nil, fmt.Errorf("get %s/%s: %w", org, name, err)
	}
	st.DefaultBranch = info.GetDefaultBranch()

	rawTags, err := c.ListTags(ctx, org, name)
	if err != nil {
		return nil, fmt.Errorf("list tags: %w", err)
	}
	st.Tags = rawTags

	rawBranches, err := c.ListBranches(ctx, org, name)
	if err != nil {
		return nil, fmt.Errorf("list branches: %w", err)
	}

	// Classify all branches.
	for _, n := range rawBranches {
		st.Branches = append(st.Branches, ParseBranch(n))
	}

	// Pick the current release branch: highest version among release branches.
	relBranches := FilterReleaseBranches(rawBranches)
	if len(relBranches) > 0 {
		cur := relBranches[0]
		st.CurrentBranch = &cur
		st.CurrentVer = cur.Version

		stage, tag := stageFromTagsOnBranch(ctx, c, org, name, cur)
		st.CurrentStage = stage
		st.CurrentTag = tag
	} else {
		st.CurrentStage = StageNone
	}

	// Suggested next versions off the current version.
	base := st.CurrentVer
	if base == "" {
		base = "0.0.0"
	}
	st.NextPatch, _ = NextVersion(base, BumpPatch)
	st.NextMinor, _ = NextVersion(base, BumpMinor)
	st.NextMajor, _ = NextVersion(base, BumpMajor)

	// Pipeline files.
	if ok, _ := c.FileExists(ctx, org, name, ".goreleaser.yml"); ok {
		st.HasGoReleaser = true
	} else if ok, _ := c.FileExists(ctx, org, name, ".goreleaser.yaml"); ok {
		st.HasGoReleaser = true
	}
	if ok, _ := c.FileExists(ctx, org, name, ".github/workflows/release.yml"); ok {
		st.HasWorkflow = true
	}
	if ok, _ := c.FileExists(ctx, org, name, "VERSION"); ok {
		st.HasVersionFile = true
	}

	return st, nil
}

// stageFromTagsOnBranch determines the highest stage among the four possible
// tags for a release branch (v<ver>, v<ver>-rc, v<ver>-beta, v<ver>-alpha).
// Returns the stage and the exact tag achieving it.
//
// Uses the compare API: a tag is "on" the branch when the branch is at or
// ahead of the tag commit.
func stageFromTagsOnBranch(ctx context.Context, c *gh.Client, org, name string, b Branch) (Stage, string) {
	if b.Version == "" {
		return StageNone, ""
	}

	candidates := []struct {
		tag   string
		stage Stage
	}{
		{"v" + b.Version, StageFinal},
		{"v" + b.Version + "-rc", StageRC},
		{"v" + b.Version + "-beta", StageBeta},
		{"v" + b.Version + "-alpha", StageAlpha},
	}

	for _, cand := range candidates {
		exists, err := c.TagExists(ctx, org, name, cand.tag)
		if err != nil || !exists {
			continue
		}
		status, err := c.CompareStatus(ctx, org, name, cand.tag, b.Name)
		if err != nil {
			continue
		}
		// "identical" — tag is at the branch tip
		// "ahead"     — branch is ahead of the tag
		// "behind"    — tag is ahead of the branch (tag not on this branch)
		// "diverged"  — no linear relation
		if status == "identical" || status == "ahead" {
			return cand.stage, cand.tag
		}
	}
	return StageNone, ""
}

// AllBranches returns st.Branches sorted: release branches first (newest
// version), then dev branches, then everything else, each group alphabetical.
func (st *RepoStatus) AllBranches() []Branch {
	out := append([]Branch(nil), st.Branches...)
	sort.SliceStable(out, func(i, j int) bool {
		order := func(k BranchKind) int {
			switch k {
			case BranchRelease:
				return 0
			case BranchDev:
				return 1
			}
			return 2
		}
		oi, oj := order(out[i].Kind), order(out[j].Kind)
		if oi != oj {
			return oi < oj
		}
		if out[i].Kind == BranchRelease || out[i].Kind == BranchDev {
			return versionGreater(out[i].Version, out[j].Version)
		}
		return out[i].Name < out[j].Name
	})
	return out
}

// ExistingTagsFromStatus builds the ExistingTags helper from a RepoStatus.
func (st *RepoStatus) ExistingTagsFromStatus() ExistingTags {
	names := make([]string, 0, len(st.Branches))
	for _, b := range st.Branches {
		names = append(names, b.Name)
	}
	return ExistingTags{Tags: st.Tags, Branches: names}
}

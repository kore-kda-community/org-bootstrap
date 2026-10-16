package release

import (
	"fmt"
	"sort"
	"strings"

	"github.com/Masterminds/semver/v3"
)

// Bump is the type of version step to compute.
type Bump string

const (
	BumpPatch  Bump = "patch"
	BumpMinor  Bump = "minor"
	BumpMajor  Bump = "major"
	BumpAlpha  Bump = "alpha"
	BumpBeta   Bump = "beta"
	BumpRC     Bump = "rc"
	BumpFinal  Bump = "final"
	BumpStable Bump = "stable"
)

// BumpFromStage maps a stage name to the Bump that advances to it.
func BumpFromStage(s Stage) Bump {
	switch s {
	case StageAlpha:
		return BumpAlpha
	case StageBeta:
		return BumpBeta
	case StageRC:
		return BumpRC
	case StageFinal:
		return BumpFinal
	}
	return BumpPatch
}

// ExistingTags is the current tag set for a repo, provided by the caller so
// this package stays network-free.
type ExistingTags struct {
	Tags     []string // raw tag names, e.g. "v0.1.0-alpha.1"
	Branches []string // branch names, e.g. "release_pact_5.4.1"
}

// HasTag reports whether any tag exactly matches name.
func (e ExistingTags) HasTag(name string) bool {
	for _, t := range e.Tags {
		if t == name {
			return true
		}
	}
	return false
}

// HasAnyTagForBase reports whether any tag starts with "v<base>".
// base is like "5.4.1", not "v5.4.1".
func (e ExistingTags) HasAnyTagForBase(base string) bool {
	prefix := "v" + strings.TrimPrefix(base, "v")
	for _, t := range e.Tags {
		if t == prefix || strings.HasPrefix(t, prefix+"-") {
			return true
		}
	}
	return false
}

// HasBranch reports whether any branch exactly matches name.
func (e ExistingTags) HasBranch(name string) bool {
	for _, b := range e.Branches {
		if b == name {
			return true
		}
	}
	return false
}

// LatestKnown returns the highest semver-parsed tag, preferring stable over
// pre-release when versions are otherwise equal. Returns "" if none parse.
func (e ExistingTags) LatestKnown() string {
	type parsed struct {
		raw string
		v   *semver.Version
	}
	var all []parsed
	for _, t := range e.Tags {
		v, err := semver.NewVersion(strings.TrimPrefix(t, "v"))
		if err != nil {
			continue
		}
		all = append(all, parsed{raw: t, v: v})
	}
	if len(all) == 0 {
		return ""
	}
	sort.Slice(all, func(i, j int) bool {
		return all[i].v.LessThan(all[j].v)
	})
	return all[len(all)-1].raw
}

// LatestStable returns the highest non-prerelease tag, or "" if none.
func (e ExistingTags) LatestStable() string {
	var best *semver.Version
	var bestRaw string
	for _, t := range e.Tags {
		v, err := semver.NewVersion(strings.TrimPrefix(t, "v"))
		if err != nil {
			continue
		}
		if v.Prerelease() != "" {
			continue
		}
		if best == nil || v.GreaterThan(best) {
			best = v
			bestRaw = t
		}
	}
	return bestRaw
}

// StageOf returns the stage encoded in the given tag.
func StageOf(tag string) Stage {
	v, err := semver.NewVersion(strings.TrimPrefix(tag, "v"))
	if err != nil {
		return StageNone
	}
	pre := v.Prerelease()
	if pre == "" {
		return StageFinal
	}
	// Strip trailing ".N" so "alpha.1" → "alpha"
	if i := strings.LastIndex(pre, "."); i >= 0 {
		pre = pre[:i]
	}
	return ParseStage(pre)
}

// NextVersion computes the next version for (last, bump). Returns a string
// without the leading "v".
//
// - last="" → "0.1.0" (alpha.1 / beta.1 / rc.1 for pre-release bumps)
// - last="1.2.3", patch → "1.2.4"
// - last="1.2.3-alpha.2", beta → "1.2.3-beta.1"
// - last="1.2.3-rc.5", final → "1.2.3"
func NextVersion(last string, bump Bump) (string, error) {
	last = strings.TrimPrefix(last, "v")

	if last == "" {
		switch bump {
		case BumpAlpha:
			return "0.1.0-alpha.1", nil
		case BumpBeta:
			return "0.1.0-beta.1", nil
		case BumpRC:
			return "0.1.0-rc.1", nil
		default:
			return "0.1.0", nil
		}
	}

	v, err := semver.NewVersion(last)
	if err != nil {
		return "", fmt.Errorf("parse %q: %w", last, err)
	}

	curStage := StageOf("v" + last)
	curPre := v.Prerelease()
	curPreBase := curPre
	curPreNum := 0
	if i := strings.LastIndex(curPre, "."); i >= 0 {
		curPreBase = curPre[:i]
		fmt.Sscanf(curPre[i+1:], "%d", &curPreNum)
	}

	base := func() string {
		return fmt.Sprintf("%d.%d.%d", v.Major(), v.Minor(), v.Patch())
	}

	switch bump {
	case BumpPatch:
		if curPre != "" {
			// bumping from a pre-release: keep the base, clear pre
			return base(), nil
		}
		return fmt.Sprintf("%d.%d.%d", v.Major(), v.Minor(), v.Patch()+1), nil

	case BumpMinor:
		return fmt.Sprintf("%d.%d.0", v.Major(), v.Minor()+1), nil

	case BumpMajor:
		return fmt.Sprintf("%d.0.0", v.Major()+1), nil

	case BumpFinal, BumpStable:
		if curPre != "" {
			return base(), nil
		}
		return base(), nil

	case BumpAlpha:
		if curPreBase == "alpha" {
			return fmt.Sprintf("%s-alpha.%d", base(), curPreNum+1), nil
		}
		if curStage == StageNone {
			return fmt.Sprintf("%d.%d.%d-alpha.1", v.Major(), v.Minor(), v.Patch()+1), nil
		}
		return fmt.Sprintf("%s-alpha.1", base()), nil

	case BumpBeta:
		if curPreBase == "beta" {
			return fmt.Sprintf("%s-beta.%d", base(), curPreNum+1), nil
		}
		if curPreBase == "alpha" {
			return fmt.Sprintf("%s-beta.1", base()), nil
		}
		if curStage == StageNone {
			return fmt.Sprintf("%d.%d.%d-beta.1", v.Major(), v.Minor(), v.Patch()+1), nil
		}
		return fmt.Sprintf("%s-beta.1", base()), nil

	case BumpRC:
		if curPreBase == "rc" {
			return fmt.Sprintf("%s-rc.%d", base(), curPreNum+1), nil
		}
		if curPreBase == "beta" || curPreBase == "alpha" {
			return fmt.Sprintf("%s-rc.1", base()), nil
		}
		if curStage == StageNone {
			return fmt.Sprintf("%d.%d.%d-rc.1", v.Major(), v.Minor(), v.Patch()+1), nil
		}
		return fmt.Sprintf("%s-rc.1", base()), nil
	}

	return "", fmt.Errorf("unknown bump: %s", bump)
}

// NextFreeVersion walks forward from candidate until a version is found that
// has no matching tag and no matching release branch. bump controls how the
// walk advances. prefix is used to check branch names. Returns the version
// without a leading "v".
func NextFreeVersion(candidate string, bump Bump, prefix string, existing ExistingTags) (string, error) {
	cur := strings.TrimPrefix(candidate, "v")
	for i := 0; i < 100; i++ {
		if !existing.HasAnyTagForBase(cur) &&
			!existing.HasBranch(MakeReleaseName(prefix, cur)) {
			return cur, nil
		}
		next, err := NextVersion(cur, bump)
		if err != nil {
			return "", err
		}
		cur = next
	}
	return "", fmt.Errorf("exhausted 100 attempts walking forward from %s", candidate)
}

// AllowedBumps returns the bumps a user can pick given the current stage.
func AllowedBumps(s Stage) []Bump {
	switch s {
	case StageNone:
		return []Bump{BumpAlpha}
	case StageAlpha:
		return []Bump{BumpAlpha, BumpBeta}
	case StageBeta:
		return []Bump{BumpBeta, BumpRC}
	case StageRC:
		return []Bump{BumpRC, BumpFinal}
	case StageFinal:
		return nil
	}
	return []Bump{BumpAlpha}
}

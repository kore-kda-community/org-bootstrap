// Package release scans repos for tag state and computes the next version.
package release

import (
	"fmt"
	"sort"
	"strings"

	"github.com/Masterminds/semver/v3"
)

// Tag is a parsed git tag.
type Tag struct {
	Raw     string
	Version *semver.Version
	Pre     string
}

// ParseTags filters to semver-shaped tags and returns them sorted ascending.
// Non-conforming tags (e.g. "nightly", "vX") are skipped.
func ParseTags(raw []string) []Tag {
	out := make([]Tag, 0, len(raw))
	for _, r := range raw {
		trimmed := strings.TrimPrefix(r, "v")
		v, err := semver.NewVersion(trimmed)
		if err != nil {
			continue
		}
		out = append(out, Tag{Raw: r, Version: v, Pre: v.Prerelease()})
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Version.LessThan(out[j].Version)
	})
	return out
}

// Latest returns the highest tag, or (nil, false) if none.
func Latest(tags []Tag) (*Tag, bool) {
	if len(tags) == 0 {
		return nil, false
	}
	return &tags[len(tags)-1], true
}

// NextPatch returns the smallest version greater than the current latest, or
// v0.1.0 if there is no latest. Pre-release suffixes are stripped in the
// comparison; the caller decides whether to append -alpha, -beta, or -rc.
func NextPatch(latest *Tag) (string, error) {
	if latest == nil {
		return "v0.1.0", nil
	}
	next := latest.Version.IncPatch()
	next, err := next.SetPrerelease("")
	if err != nil {
		return "", fmt.Errorf("clear prerelease: %w", err)
	}
	return "v" + next.String(), nil
}

// NextMinor returns the next minor bump (X.Y+1.0).
func NextMinor(latest *Tag) (string, error) {
	if latest == nil {
		return "v0.1.0", nil
	}
	next := latest.Version.IncMinor()
	next, err := next.SetPrerelease("")
	if err != nil {
		return "", fmt.Errorf("clear prerelease: %w", err)
	}
	return "v" + next.String(), nil
}

// NextMajor returns the next major bump (X+1.0.0).
func NextMajor(latest *Tag) (string, error) {
	if latest == nil {
		return "v1.0.0", nil
	}
	next := latest.Version.IncMajor()
	next, err := next.SetPrerelease("")
	if err != nil {
		return "", fmt.Errorf("clear prerelease: %w", err)
	}
	return "v" + next.String(), nil
}

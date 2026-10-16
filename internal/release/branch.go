package release

import (
	"regexp"
	"sort"
	"strings"

	"github.com/Masterminds/semver/v3"
)

// BranchKind classifies branch names in the release model.
type BranchKind int

const (
	BranchOther BranchKind = iota
	BranchRelease
	BranchDev
)

var (
	// release_<prefix>_<x.y.z>  |  <prefix>_<x.y.z>   (legacy form allowed)
	releaseBranchRe = regexp.MustCompile(`^(?:release_)?([A-Za-z0-9._-]+)_([0-9]+\.[0-9]+\.[0-9]+)$`)
	// dev_<prefix>_<x.y.z>
	devBranchRe = regexp.MustCompile(`^dev_([A-Za-z0-9._-]+)_([0-9]+\.[0-9]+\.[0-9]+)$`)
)

// Branch is a parsed branch name.
type Branch struct {
	Name    string
	Kind    BranchKind
	Prefix  string // project prefix, e.g. "pact"
	Version string // semver, e.g. "5.4.1"
}

// ParseBranch classifies a branch name. Anything that doesn't match a release
// or dev pattern returns BranchOther.
func ParseBranch(name string) Branch {
	b := Branch{Name: name, Kind: BranchOther}

	if m := devBranchRe.FindStringSubmatch(name); m != nil {
		b.Kind = BranchDev
		b.Prefix = m[1]
		b.Version = m[2]
		return b
	}

	// Guard: dev_ prefix always wins; release regex would otherwise accept it
	if strings.HasPrefix(name, "dev_") {
		return b
	}

	if m := releaseBranchRe.FindStringSubmatch(name); m != nil {
		b.Kind = BranchRelease
		b.Prefix = m[1]
		b.Version = m[2]
	}
	return b
}

// IsRelease reports whether b is a release branch.
func (b Branch) IsRelease() bool { return b.Kind == BranchRelease }

// IsDev reports whether b is a dev branch.
func (b Branch) IsDev() bool { return b.Kind == BranchDev }

// MakeReleaseName builds "release_<prefix>_<version>".
func MakeReleaseName(prefix, version string) string {
	return "release_" + prefix + "_" + strings.TrimPrefix(version, "v")
}

// MakeDevName builds "dev_<prefix>_<version>".
func MakeDevName(prefix, version string) string {
	return "dev_" + prefix + "_" + strings.TrimPrefix(version, "v")
}

// FilterReleaseBranches returns parsed release branches sorted by version
// descending (newest first). Non-release branches are dropped.
func FilterReleaseBranches(names []string) []Branch {
	var out []Branch
	for _, n := range names {
		b := ParseBranch(n)
		if b.IsRelease() {
			out = append(out, b)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		return versionGreater(out[i].Version, out[j].Version)
	})
	return out
}

// versionGreater reports whether a > b, falling back to string compare when
// either side isn't valid semver.
func versionGreater(a, b string) bool {
	va, ea := semver.NewVersion(a)
	vb, eb := semver.NewVersion(b)
	if ea != nil || eb != nil {
		return a > b
	}
	return va.GreaterThan(vb)
}

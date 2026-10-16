// Package repo orchestrates the GitHub repos the bootstrapper manages.
package repo

// Well-known repo names.
const (
	DotGithub   = ".github"
	Releases    = "releases"
	HomebrewTap = "homebrew-tap"
	ToolRepo    = "homebrew-tap-app"
)

// Profile README path inside .github.
const ProfileReadmePath = "profile/README.md"

// Standard paths for generated files.
const (
	ReleasesReadme   = "README.md"
	TapReadme        = "README.md"
	SourceGoReleaser = ".goreleaser.yml"
	SourceWorkflow   = ".github/workflows/release.yml"
)

// InfraRepos are the repos that must be promoted to public after creation.
var InfraRepos = []string{DotGithub, Releases, HomebrewTap}

package release

import (
	"context"
	"fmt"

	gh "github.com/kore-kda-community/org-bootstrap/internal/gh"
)

// RepoStatus describes what the release manager found for one repo.
type RepoStatus struct {
	Org          string
	Name         string
	DefaultBranch string
	Tags         []Tag
	Latest       *Tag
	NextPatch    string
	NextMinor    string
	NextMajor    string
	HasGoReleaser bool
	HasWorkflow   bool
}

// Scan inspects one repo and returns its release-relevant status.
func Scan(ctx context.Context, c *gh.Client, org, name string) (*RepoStatus, error) {
	status := &RepoStatus{Org: org, Name: name}

	// Default branch + tag list
	repoInfo, err := c.GetRepo(ctx, org, name)
	if err != nil {
		return nil, fmt.Errorf("get %s/%s: %w", org, name, err)
	}
	status.DefaultBranch = repoInfo.GetDefaultBranch()

	rawTags, err := c.ListTags(ctx, org, name)
	if err != nil {
		return nil, fmt.Errorf("list tags %s/%s: %w", org, name, err)
	}
	status.Tags = ParseTags(rawTags)
	if latest, ok := Latest(status.Tags); ok {
		status.Latest = latest
	}

	// Compute next versions
	if v, err := NextPatch(status.Latest); err == nil {
		status.NextPatch = v
	}
	if v, err := NextMinor(status.Latest); err == nil {
		status.NextMinor = v
	}
	if v, err := NextMajor(status.Latest); err == nil {
		status.NextMajor = v
	}

	// Check for pipeline files
	if ok, _ := c.FileExists(ctx, org, name, ".goreleaser.yml"); ok {
		status.HasGoReleaser = true
	}
	if ok, _ := c.FileExists(ctx, org, name, ".github/workflows/release.yml"); ok {
		status.HasWorkflow = true
	}

	return status, nil
}

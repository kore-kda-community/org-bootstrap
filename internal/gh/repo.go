package gh

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/go-github/v60/github"
)

// RepoExists reports whether org/repo is accessible to the current token.
func (c *Client) RepoExists(ctx context.Context, org, repo string) (bool, error) {
	_, resp, err := c.raw.Repositories.Get(ctx, org, repo)
	if err != nil {
		if resp != nil && resp.StatusCode == 404 {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// EnsureRepo creates org/repo at the given visibility if it doesn't exist.
// Returns created=true only when this call actually created it.
func (c *Client) EnsureRepo(ctx context.Context, org, repo, visibility, description string) (bool, error) {
	exists, err := c.RepoExists(ctx, org, repo)
	if err != nil {
		return false, err
	}
	if exists {
		return false, nil
	}

	private := visibility != "public"
	r := &github.Repository{
		Name:        github.String(repo),
		Description: github.String(description),
		Private:     github.Bool(private),
		AutoInit:    github.Bool(false),
	}
	if _, _, err := c.raw.Repositories.Create(ctx, org, r); err != nil {
		return false, fmt.Errorf("create %s/%s: %w", org, repo, err)
	}
	return true, nil
}

// GetVisibility returns "public" or "private".
func (c *Client) GetVisibility(ctx context.Context, org, repo string) (string, error) {
	r, _, err := c.raw.Repositories.Get(ctx, org, repo)
	if err != nil {
		return "", err
	}
	if r.GetPrivate() {
		return "private", nil
	}
	return "public", nil
}

// SetVisibility flips visibility. No-op if already at target.
func (c *Client) SetVisibility(ctx context.Context, org, repo, visibility string) error {
	cur, err := c.GetVisibility(ctx, org, repo)
	if err != nil {
		return err
	}
	if cur == visibility {
		return nil
	}
	_, _, err = c.raw.Repositories.Edit(ctx, org, repo, &github.Repository{
		Visibility: github.String(visibility),
	})
	if err != nil {
		return fmt.Errorf("set visibility %s/%s -> %s: %w", org, repo, visibility, err)
	}
	return nil
}

// ListPublicRepos returns non-fork, non-archived public repos, sorted by pushed_at desc.
func (c *Client) ListPublicRepos(ctx context.Context, org string) ([]*github.Repository, error) {
	var out []*github.Repository
	opts := &github.RepositoryListByOrgOptions{
		Type:        "public",
		Sort:        "pushed",
		Direction:   "desc",
		ListOptions: github.ListOptions{PerPage: 100},
	}
	for {
		repos, resp, err := c.raw.Repositories.ListByOrg(ctx, org, opts)
		if err != nil {
			return nil, err
		}
		for _, r := range repos {
			if r.GetFork() || r.GetArchived() {
				continue
			}
			out = append(out, r)
		}
		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}
	return out, nil
}

// ListPrivateRepoNames returns non-fork, non-archived private repo names.
func (c *Client) ListPrivateRepoNames(ctx context.Context, org string) ([]string, error) {
	var out []string
	opts := &github.RepositoryListByOrgOptions{
		Type:        "private",
		ListOptions: github.ListOptions{PerPage: 100},
	}
	for {
		repos, resp, err := c.raw.Repositories.ListByOrg(ctx, org, opts)
		if err != nil {
			return nil, err
		}
		for _, r := range repos {
			if r.GetFork() || r.GetArchived() {
				continue
			}
			out = append(out, r.GetName())
		}
		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}
	return out, nil
}

// DeleteRepo removes org/repo. Cleanup only.
func (c *Client) DeleteRepo(ctx context.Context, org, repo string) error {
	_, err := c.raw.Repositories.Delete(ctx, org, repo)
	return err
}

// GetRepo returns the underlying go-github Repository.
func (c *Client) GetRepo(ctx context.Context, org, name string) (*github.Repository, error) {
	r, _, err := c.raw.Repositories.Get(ctx, org, name)
	return r, err
}

// ListTags returns tag names for org/name, in whatever order GitHub provides.
func (c *Client) ListTags(ctx context.Context, org, name string) ([]string, error) {
	var out []string
	opts := &github.ListOptions{PerPage: 100}
	for {
		tags, resp, err := c.raw.Repositories.ListTags(ctx, org, name, opts)
		if err != nil {
			return nil, err
		}
		for _, t := range tags {
			out = append(out, t.GetName())
		}
		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}
	return out, nil
}

// FileExists reports whether path exists in org/name at the default branch.
func (c *Client) FileExists(ctx context.Context, org, name, path string) (bool, error) {
	_, _, resp, err := c.raw.Repositories.GetContents(ctx, org, name, path, nil)
	if err != nil {
		if resp != nil && resp.StatusCode == 404 {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// ListBranches returns branch names for org/name.
func (c *Client) ListBranches(ctx context.Context, org, name string) ([]string, error) {
	var out []string
	opts := &github.BranchListOptions{ListOptions: github.ListOptions{PerPage: 100}}
	for {
		branches, resp, err := c.raw.Repositories.ListBranches(ctx, org, name, opts)
		if err != nil {
			return nil, err
		}
		for _, b := range branches {
			out = append(out, b.GetName())
		}
		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}
	return out, nil
}

// BranchExists reports whether a branch exists on org/name.
func (c *Client) BranchExists(ctx context.Context, org, name, branch string) (bool, error) {
	_, resp, err := c.raw.Repositories.GetBranch(ctx, org, name, branch, 0)
	if err != nil {
		if resp != nil && resp.StatusCode == 404 {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// TagExists reports whether a git tag exists on org/name.
func (c *Client) TagExists(ctx context.Context, org, name, tag string) (bool, error) {
	_, _, err := c.raw.Git.GetRef(ctx, org, name, "tags/"+tag)
	if err != nil {
		if strings.Contains(err.Error(), "404") {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// CompareStatus returns the GitHub compare status between base and head:
// "identical" | "ahead" | "behind" | "diverged".
func (c *Client) CompareStatus(ctx context.Context, org, name, base, head string) (string, error) {
	cmp, _, err := c.raw.Repositories.CompareCommits(ctx, org, name, base, head, nil)
	if err != nil {
		return "", err
	}
	return cmp.GetStatus(), nil
}

// ListOrgRepos returns every repo in an org, paginated. Forks and archived
// repos are skipped.
func (c *Client) ListOrgRepos(ctx context.Context, org string) ([]*github.Repository, error) {
	var out []*github.Repository
	opts := &github.RepositoryListByOrgOptions{
		ListOptions: github.ListOptions{PerPage: 100},
		Type:        "all",
	}
	for {
		repos, resp, err := c.raw.Repositories.ListByOrg(ctx, org, opts)
		if err != nil {
			return nil, err
		}
		for _, r := range repos {
			if r.GetFork() || r.GetArchived() {
				continue
			}
			out = append(out, r)
		}
		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}
	return out, nil
}

// GetLatestRelease returns the newest published release, or nil.
func (c *Client) GetLatestRelease(ctx context.Context, org, repo string) (*github.RepositoryRelease, error) {
	r, resp, err := c.raw.Repositories.GetLatestRelease(ctx, org, repo)
	if err == nil {
		return r, nil
	}
	if resp == nil || resp.StatusCode != 404 {
		return nil, err
	}
	// Fallback: list releases, take the first (newest). Catches repos where
	// GitHub hasn't flagged any release as "Latest" (e.g. only prereleases).
	rels, _, lerr := c.raw.Repositories.ListReleases(ctx, org, repo,
		&github.ListOptions{PerPage: 1})
	if lerr != nil || len(rels) == 0 {
		return nil, nil
	}
	return rels[0], nil
}

// RenameRepo changes a repo's name. GitHub auto-redirects the old name.
func (c *Client) RenameRepo(ctx context.Context, org, from, to string) error {
	_, _, err := c.raw.Repositories.Edit(ctx, org, from, &github.Repository{
		Name: github.String(to),
	})
	return err
}

// ListRepoFiles returns paths of every blob reachable from HEAD in org/repo.
// Recursive. Skips nothing — caller filters.
func (c *Client) ListRepoFiles(ctx context.Context, org, repo, prefix string) ([]string, error) {
	tree, _, err := c.raw.Git.GetTree(ctx, org, repo, "HEAD", true)
	if err != nil {
		return nil, err
	}
	var out []string
	for _, e := range tree.Entries {
		if e.GetType() != "blob" {
			continue
		}
		path := e.GetPath()
		if prefix != "" && !strings.HasPrefix(path, prefix) {
			continue
		}
		out = append(out, path)
	}
	return out, nil
}

// GetFileContent downloads a file from the default branch.
func (c *Client) GetFileContent(ctx context.Context, org, repo, path string) ([]byte, error) {
	f, _, _, err := c.raw.Repositories.GetContents(ctx, org, repo, path, nil)
	if err != nil {
		return nil, err
	}
	content, err := f.GetContent()
	if err != nil {
		return nil, err
	}
	return []byte(content), nil
}

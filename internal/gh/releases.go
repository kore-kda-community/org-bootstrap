package gh

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/google/go-github/v60/github"
)

// ListReleases returns every release for org/repo, paginated.
func (c *Client) ListReleases(ctx context.Context, org, repo string) ([]*github.RepositoryRelease, error) {
	var all []*github.RepositoryRelease
	opts := &github.ListOptions{PerPage: 100}
	for {
		rels, resp, err := c.raw.Repositories.ListReleases(ctx, org, repo, opts)
		if err != nil {
			return nil, err
		}
		all = append(all, rels...)
		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}
	return all, nil
}

// GetReleaseByTag fetches one release.
func (c *Client) GetReleaseByTag(ctx context.Context, org, repo, tag string) (*github.RepositoryRelease, error) {
	r, _, err := c.raw.Repositories.GetReleaseByTag(ctx, org, repo, tag)
	return r, err
}

// CreateRelease creates a release. If tag doesn't already exist on the repo,
// GitHub auto-creates it at the default branch HEAD.
func (c *Client) CreateRelease(ctx context.Context, org, repo, tag, name, body string) (*github.RepositoryRelease, error) {
	req := &github.RepositoryRelease{
		TagName: github.String(tag),
		Name:    github.String(name),
		Body:    github.String(body),
	}
	r, _, err := c.raw.Repositories.CreateRelease(ctx, org, repo, req)
	return r, err
}

// DeleteRelease deletes a release by ID. Does not touch the git tag.
func (c *Client) DeleteRelease(ctx context.Context, org, repo string, id int64) error {
	_, err := c.raw.Repositories.DeleteRelease(ctx, org, repo, id)
	return err
}

// DownloadReleaseAsset writes a release asset to dest. Returns the byte count.
func (c *Client) DownloadReleaseAsset(ctx context.Context, org, repo string, assetID int64, dest string) (int64, error) {
	rc, redirectURL, err := c.raw.Repositories.DownloadReleaseAsset(ctx, org, repo, assetID, nil)
	if err != nil {
		return 0, err
	}
	if rc == nil && redirectURL != "" {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, redirectURL, nil)
		if err != nil {
			return 0, err
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return 0, err
		}
		defer resp.Body.Close()
		rc = resp.Body
	}
	if rc == nil {
		return 0, fmt.Errorf("no asset stream for asset %d", assetID)
	}
	defer rc.Close()

	f, err := os.Create(dest)
	if err != nil {
		return 0, err
	}
	defer f.Close()
	return io.Copy(f, rc)
}

// UploadReleaseAsset uploads local file to an existing release. name is the
// asset name shown on GitHub (may differ from the local filename).
func (c *Client) UploadReleaseAsset(ctx context.Context, org, repo string, releaseID int64, localPath, name string) error {
	f, err := os.Open(localPath)
	if err != nil {
		return err
	}
	defer f.Close()
	opts := &github.UploadOptions{Name: name}
	_, _, err = c.raw.Repositories.UploadReleaseAsset(ctx, org, repo, releaseID, opts, f)
	return err
}

// CreateRepo creates an auto-initialized repo. Only used for the archive
// repo here; bootstrap has its own Ensure.
func (c *Client) CreateRepo(ctx context.Context, org, name, description string, private bool) error {
	_, _, err := c.raw.Repositories.Create(ctx, org, &github.Repository{
		Name:        github.String(name),
		Description: github.String(description),
		Private:     github.Bool(private),
		AutoInit:    github.Bool(true),
	})
	return err
}

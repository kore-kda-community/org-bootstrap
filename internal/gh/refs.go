package gh

import (
	"context"
	"fmt"
	"time"

	"github.com/google/go-github/v60/github"
)

// GetRefSHA returns the commit SHA a ref points at. ref is like "heads/main".
func (c *Client) GetRefSHA(ctx context.Context, org, repo, ref string) (string, error) {
	r, _, err := c.raw.Git.GetRef(ctx, org, repo, ref)
	if err != nil {
		return "", err
	}
	return r.GetObject().GetSHA(), nil
}

// CreateTag creates a tag ref pointing at the given SHA.
// Returns an error if the tag already exists.
func (c *Client) CreateTag(ctx context.Context, org, repo, tag, sha string) error {
	ref := "refs/tags/" + tag
	_, _, err := c.raw.Git.CreateRef(ctx, org, repo, &github.Reference{
		Ref:    github.String(ref),
		Object: &github.GitObject{SHA: github.String(sha)},
	})
	return err
}

// ForceUpdateTag moves an existing tag to a new SHA.
func (c *Client) ForceUpdateTag(ctx context.Context, org, repo, tag, sha string) error {
	ref := "refs/tags/" + tag
	_, _, err := c.raw.Git.UpdateRef(ctx, org, repo, &github.Reference{
		Ref:    github.String(ref),
		Object: &github.GitObject{SHA: github.String(sha)},
		// Force is implied by non-fast-forward updates through this API
	}, true)
	return err
}

// DeleteTag removes a tag ref.
func (c *Client) DeleteTag(ctx context.Context, org, repo, tag string) error {
	_, err := c.raw.Git.DeleteRef(ctx, org, repo, "tags/"+tag)
	return err
}

// CreateBranch creates a branch ref pointing at sha.
func (c *Client) CreateBranch(ctx context.Context, org, repo, name, sha string) error {
	ref := "refs/heads/" + name
	_, _, err := c.raw.Git.CreateRef(ctx, org, repo, &github.Reference{
		Ref:    github.String(ref),
		Object: &github.GitObject{SHA: github.String(sha)},
	})
	return err
}

// ListActionsSecrets returns the names of Actions secrets on org/repo.
func (c *Client) ListActionsSecrets(ctx context.Context, org, repo string) ([]string, error) {
	var out []string
	opts := &github.ListOptions{PerPage: 100}
	for {
		secrets, resp, err := c.raw.Actions.ListRepoSecrets(ctx, org, repo, opts)
		if err != nil {
			return nil, err
		}
		for _, s := range secrets.Secrets {
			out = append(out, s.Name)
		}
		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}
	return out, nil
}

// ListWorkflowRuns returns the most recent run summary lines, newest first.
func (c *Client) ListWorkflowRuns(ctx context.Context, org, repo string, limit int) ([]string, error) {
	opts := &github.ListWorkflowRunsOptions{
		ListOptions: github.ListOptions{PerPage: limit},
	}
	runs, _, err := c.raw.Actions.ListRepositoryWorkflowRuns(ctx, org, repo, opts)
	if err != nil {
		return nil, err
	}
	var out []string
	for _, r := range runs.WorkflowRuns {
		status := r.GetStatus()
		conclusion := r.GetConclusion()
		if conclusion == "" {
			conclusion = "—"
		}
		out = append(out, fmt.Sprintf("%-12s %-10s %s",
			truncStr(r.GetName(), 12),
			status+"/"+conclusion,
			r.GetCreatedAt().Format("2006-01-02 15:04"),
		))
	}
	return out, nil
}

func truncStr(s string, n int) string {
	if len(s) <= n {
		return s
	}
	if n <= 1 {
		return s[:n]
	}
	return s[:n-1] + "…"
}

// GetCommitDate returns the committer date of the commit pointed at by ref
// (tag, branch, or SHA).
func (c *Client) GetCommitDate(ctx context.Context, org, repo, ref string) (time.Time, error) {
	cm, _, err := c.raw.Repositories.GetCommit(ctx, org, repo, ref, nil)
	if err != nil {
		return time.Time{}, err
	}
	if cm.GetCommit().GetCommitter().Date != nil {
		return cm.GetCommit().GetCommitter().Date.Time, nil
	}
	return time.Time{}, nil
}

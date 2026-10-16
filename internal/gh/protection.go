package gh

import (
	"context"
	"fmt"
	"net/http"
)

// ProtectionState captures what we know about a branch's protection config.
type ProtectionState struct {
	Protected   bool
	Locked      bool // lock_branch == true
	RequiredPRs bool
	ForcePush   bool // allow_force_pushes
	Deletions   bool // allow_deletions
}

// GetBranchProtection fetches the current protection config for a branch.
// Returns Protected=false if the branch is not protected or returns 404.
func (c *Client) GetBranchProtection(ctx context.Context, org, repo, branch string) (*ProtectionState, error) {
	req, err := c.raw.NewRequest(http.MethodGet,
		fmt.Sprintf("repos/%s/%s/branches/%s/protection", org, repo, branch), nil)
	if err != nil {
		return nil, err
	}

	var raw struct {
		LockBranch       *bool `json:"lock_branch"`
		AllowForcePushes *struct {
			Enabled bool `json:"enabled"`
		} `json:"allow_force_pushes"`
		AllowDeletions *struct {
			Enabled bool `json:"enabled"`
		} `json:"allow_deletions"`
		RequiredPRs *struct{} `json:"required_pull_request_reviews"`
	}

	resp, err := c.raw.Do(ctx, req, &raw)
	if err != nil {
		if resp != nil && resp.StatusCode == 404 {
			return &ProtectionState{Protected: false}, nil
		}
		return nil, err
	}

	st := &ProtectionState{Protected: true}
	if raw.LockBranch != nil {
		st.Locked = *raw.LockBranch
	}
	if raw.AllowForcePushes != nil {
		st.ForcePush = raw.AllowForcePushes.Enabled
	}
	if raw.AllowDeletions != nil {
		st.Deletions = raw.AllowDeletions.Enabled
	}
	st.RequiredPRs = raw.RequiredPRs != nil
	return st, nil
}

// UpdateBranchProtection writes a protection config to a branch.
//
// When lockBranch is true this attempts the strongest config (lock_branch +
// no force pushes + no deletions + linear history + PR required). If GitHub
// rejects lock_branch (some plans disallow it), it retries without lock_branch.
//
// Returns (locked, err). locked=true means lock_branch was applied.
func (c *Client) UpdateBranchProtection(ctx context.Context, org, repo, branch string, lockBranch bool) (bool, error) {
	body := func(lock bool) map[string]any {
		return map[string]any{
			"required_status_checks": nil,
			"enforce_admins":         true,
			"required_pull_request_reviews": map[string]any{
				"dismiss_stale_reviews":           true,
				"require_code_owner_reviews":      false,
				"required_approving_review_count": 0,
			},
			"restrictions":            nil,
			"allow_force_pushes":      false,
			"allow_deletions":         false,
			"required_linear_history": true,
			"lock_branch":             lock,
		}
	}

	req, err := c.raw.NewRequest(http.MethodPut,
		fmt.Sprintf("repos/%s/%s/branches/%s/protection", org, repo, branch), body(lockBranch))
	if err != nil {
		return false, err
	}

	resp, err := c.raw.Do(ctx, req, nil)
	if err == nil {
		return lockBranch, nil
	}

	// lock_branch unsupported on this plan? Retry without.
	if lockBranch && resp != nil && (resp.StatusCode == 422 || resp.StatusCode == 403) {
		req2, err2 := c.raw.NewRequest(http.MethodPut,
			fmt.Sprintf("repos/%s/%s/branches/%s/protection", org, repo, branch), body(false))
		if err2 != nil {
			return false, err2
		}
		if _, err3 := c.raw.Do(ctx, req2, nil); err3 == nil {
			return false, nil
		}
	}
	return false, err
}

// RemoveBranchProtection deletes the protection config from a branch.
func (c *Client) RemoveBranchProtection(ctx context.Context, org, repo, branch string) error {
	req, err := c.raw.NewRequest(http.MethodDelete,
		fmt.Sprintf("repos/%s/%s/branches/%s/protection", org, repo, branch), nil)
	if err != nil {
		return err
	}
	_, err = c.raw.Do(ctx, req, nil)
	return err
}

// CreateMainRuleset creates a repository ruleset that blocks deletions and
// non-fast-forward pushes on refs/heads/main.
func (c *Client) CreateMainRuleset(ctx context.Context, org, repo string) error {
	payload := map[string]any{
		"name":        "protect-main",
		"target":      "branch",
		"enforcement": "active",
		"conditions": map[string]any{
			"ref_name": map[string]any{
				"include": []string{"refs/heads/main"},
				"exclude": []string{},
			},
		},
		"rules": []map[string]any{
			{"type": "deletion"},
			{"type": "non_fast_forward"},
		},
	}
	req, err := c.raw.NewRequest(http.MethodPost,
		fmt.Sprintf("repos/%s/%s/rulesets", org, repo), payload)
	if err != nil {
		return err
	}
	_, err = c.raw.Do(ctx, req, nil)
	return err
}

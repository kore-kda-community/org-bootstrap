// Package protection provides high-level branch protection operations with
// fallbacks that accommodate different GitHub plans.
package protection

import (
	"context"
	"fmt"

	"github.com/kore-kda-community/org-bootstrap/internal/gh"
)

// LockResult describes what happened during a LockMain call.
type LockResult struct {
	Locked    bool   // true if lock_branch was applied
	Protected bool   // true if any protection was applied
	Method    string // "already" | "protection+lock" | "protection" | "ruleset" | "none"
	Warning   string // populated when Method=="none"
}

// LockMain applies the strongest available protection to the main branch:
//
//  1. If already locked → return immediately, method "already".
//  2. Try branch protection with lock_branch=true.
//  3. Fall back to branch protection without lock_branch.
//  4. Fall back to a repository ruleset (deletion + non-fast-forward).
//  5. All fail → return Warning, no error.
//
// A plan that disallows protection and rulesets is not an error — the caller
// treats this as informational and proceeds. The scripts never write to main
// anyway, so protection is defence-in-depth.
func LockMain(ctx context.Context, c *gh.Client, org, repo string) (*LockResult, error) {
	cur, err := c.GetBranchProtection(ctx, org, repo, "main")
	if err != nil {
		return nil, fmt.Errorf("get protection: %w", err)
	}
	if cur.Protected && cur.Locked {
		return &LockResult{Locked: true, Protected: true, Method: "already"}, nil
	}

	locked, err := c.UpdateBranchProtection(ctx, org, repo, "main", true)
	if err == nil {
		if locked {
			return &LockResult{Locked: true, Protected: true, Method: "protection+lock"}, nil
		}
		return &LockResult{Locked: false, Protected: true, Method: "protection"}, nil
	}

	// Protection itself failed — try a ruleset as a last resort.
	if rsErr := c.CreateMainRuleset(ctx, org, repo); rsErr == nil {
		return &LockResult{Locked: false, Protected: true, Method: "ruleset"}, nil
	}

	return &LockResult{
		Locked:    false,
		Protected: false,
		Method:    "none",
		Warning:   "this GitHub plan does not allow branch protection or rulesets on private repos — informational only",
	}, nil
}

// UnlockMain removes main branch protection if present. Returns nil if main
// was not protected (nothing to do).
func UnlockMain(ctx context.Context, c *gh.Client, org, repo string) error {
	cur, err := c.GetBranchProtection(ctx, org, repo, "main")
	if err != nil {
		return err
	}
	if !cur.Protected {
		return nil
	}
	return c.RemoveBranchProtection(ctx, org, repo, "main")
}

// MainIsLocked is a convenience wrapper for the interactive menu.
func MainIsLocked(ctx context.Context, c *gh.Client, org, repo string) bool {
	st, err := c.GetBranchProtection(ctx, org, repo, "main")
	if err != nil {
		return false
	}
	return st.Protected
}

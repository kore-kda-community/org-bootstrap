// Package visibility gates repo promotion on a secrets audit pass.
package visibility

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/kore-kda-community/org-bootstrap/internal/audit"
	"github.com/kore-kda-community/org-bootstrap/internal/config"
	"github.com/kore-kda-community/org-bootstrap/internal/gh"
	"github.com/kore-kda-community/org-bootstrap/internal/repo"
)

// Result reports what Promote did, for logging.
type Result struct {
	Org      string
	Repo     string
	Skipped  bool
	Audit    *audit.Report
	Promoted bool
	Refused  bool
}

// Promote audits workDir and, if clean, flips org/name to public via the gh
// client. If the repo is already public, it returns Result{Skipped: true}
// without running the audit.
func Promote(ctx context.Context, c *gh.Client, cfg config.Audit, org, name, workDir string) (*Result, error) {
	res := &Result{Org: org, Repo: name}

	cur, err := c.GetVisibility(ctx, org, name)
	if err != nil {
		return res, fmt.Errorf("get visibility %s/%s: %w", org, name, err)
	}
	if cur == "public" {
		res.Skipped = true
		return res, nil
	}

	rulesPath := cfg.RulesPath
	if rulesPath == "" {
		rulesPath = "configs/audit-default.toml"
	}
	rules, excludes, err := audit.LoadRules(rulesPath)
	if err != nil {
		return res, fmt.Errorf("load rules: %w", err)
	}

	scanRoot := workDir
	if scanRoot == "" {
		tmp, terr := os.MkdirTemp("", "org-bootstrap-audit-")
		if terr != nil {
			return res, fmt.Errorf("mktemp: %w", terr)
		}
		defer os.RemoveAll(tmp)
		scanRoot = filepath.Join(tmp, name)
		if cerr := cloneInto(ctx, org, name, scanRoot); cerr != nil {
			return res, fmt.Errorf("clone for audit: %w", cerr)
		}
	}

	started := time.Now()
	outcome, err := audit.Scan(scanRoot, rules, excludes)
	if err != nil {
		return res, fmt.Errorf("scan: %w", err)
	}
	finished := time.Now()

	rep := audit.Build(outcome, org, name, "", started, finished)
	res.Audit = rep

	if !rep.Clean {
		res.Refused = true
		return res, nil
	}

	if err := repo.Promote(ctx, c, org, name); err != nil {
		return res, fmt.Errorf("promote %s/%s: %w", org, name, err)
	}
	res.Promoted = true
	return res, nil
}

// cloneInto shells out to git to clone org/name into dest, depth 1.
func cloneInto(ctx context.Context, org, name, dest string) error {
	token := os.Getenv("GH_TOKEN")
	url := fmt.Sprintf("https://x-access-token:%s@github.com/%s/%s.git", token, org, name)
	if err := os.MkdirAll(dest, 0o755); err != nil {
		return err
	}
	cmd := exec.CommandContext(ctx, "git", "clone", "--depth", "1", url, dest)
	return cmd.Run()
}

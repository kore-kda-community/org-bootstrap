package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/kore-kda-community/org-bootstrap/internal/audit"
)

func main() {
	var (
		dir    string
		rules  string
		org    string
		repo   string
		sha    string
		format string
	)
	flag.StringVar(&dir, "dir", ".", "directory to scan")
	flag.StringVar(&rules, "rules", "configs/audit-default.toml", "audit rules TOML")
	flag.StringVar(&org, "org", "", "org name for the report")
	flag.StringVar(&repo, "repo", "", "repo name for the report")
	flag.StringVar(&sha, "sha", "", "commit SHA (optional)")
	flag.StringVar(&format, "format", "text", "output format: text or json")
	flag.Parse()

	r, ex, err := audit.LoadRules(rules)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}

	started := time.Now()
	outcome, err := audit.Scan(dir, r, ex)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
	finished := time.Now()

	if org == "" {
		org = filepath.Base(mustAbs(dir))
	}
	if repo == "" {
		repo = "."
	}
	rep := audit.Build(outcome, org, repo, sha, started, finished)

	switch format {
	case "json":
		if err := rep.WriteJSON(os.Stdout); err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			os.Exit(1)
		}
	default:
		if err := rep.WriteText(os.Stdout); err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			os.Exit(1)
		}
	}

	if !rep.Clean {
		os.Exit(2)
	}
}

func mustAbs(p string) string {
	a, err := filepath.Abs(p)
	if err != nil {
		return p
	}
	return a
}

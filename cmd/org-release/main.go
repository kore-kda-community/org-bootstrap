package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/kore-kda-community/org-bootstrap/internal/gh"
	"github.com/kore-kda-community/org-bootstrap/internal/oauth"
	"github.com/kore-kda-community/org-bootstrap/internal/release"
	"github.com/kore-kda-community/org-bootstrap/internal/config"
)

func main() {
	var (
		cfgPath string
		org     string
		repo    string
		bump    string
	)
	flag.StringVar(&cfgPath, "config", "", "path to config.toml")
	flag.StringVar(&org, "org", "", "org name")
	flag.StringVar(&repo, "repo", "", "repo name")
	flag.StringVar(&bump, "bump", "patch", "patch | minor | major")
	flag.Parse()

	if err := run(cfgPath, org, repo, bump); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run(cfgPath, org, repo, bump string) error {
	if org == "" || repo == "" {
		return fmt.Errorf("--org and --repo are required")
	}
	cfg, err := config.Load(cfgPath)
	if err != nil {
		return err
	}
	ctx := context.Background()
	token, err := oauth.EnsureToken(ctx, cfg.Auth, nil)
	if err != nil {
		return err
	}
	c, err := gh.New(ctx, token)
	if err != nil {
		return err
	}

	st, err := release.Scan(ctx, c, org, repo)
	if err != nil {
		return err
	}

	fmt.Printf("repo: %s/%s\n", st.Org, st.Name)
	fmt.Printf("default branch: %s\n", st.DefaultBranch)
	fmt.Printf("tags: %d\n", len(st.Tags))
	if st.Latest != nil {
		fmt.Printf("latest: %s\n", st.Latest.Raw)
	} else {
		fmt.Println("latest: (none)")
	}
	fmt.Printf("next patch: %s\n", st.NextPatch)
	fmt.Printf("next minor: %s\n", st.NextMinor)
	fmt.Printf("next major: %s\n", st.NextMajor)
	fmt.Printf("goreleaser: %v\n", st.HasGoReleaser)
	fmt.Printf("workflow: %v\n", st.HasWorkflow)

	switch bump {
	case "patch":
		fmt.Println("→ suggested:", st.NextPatch)
	case "minor":
		fmt.Println("→ suggested:", st.NextMinor)
	case "major":
		fmt.Println("→ suggested:", st.NextMajor)
	}
	return nil
}

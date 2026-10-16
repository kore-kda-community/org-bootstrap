// Command org-adopt adds release management to an existing source repo.
//
// It is the Go port of adopt-repo.sh: prompts for build metadata, writes
// .goreleaser.yml and .github/workflows/release.yml, sets the tap token as a
// repo secret, and records the repo in state.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/kore-kda-community/org-bootstrap/internal/config"
	"github.com/kore-kda-community/org-bootstrap/internal/gh"
	"github.com/kore-kda-community/org-bootstrap/internal/oauth"
	"github.com/kore-kda-community/org-bootstrap/internal/repo"
	"github.com/kore-kda-community/org-bootstrap/internal/state"
	"github.com/kore-kda-community/org-bootstrap/internal/tui"
)

func main() {
	var (
		cfgPath, statePath   string
		org, name            string
		binary, goMain, desc string
		homepage, license    string
		bundleVersion        string
	)
	flag.StringVar(&cfgPath, "config", "", "path to config.toml")
	flag.StringVar(&statePath, "state", "", "path to .state.json")
	flag.StringVar(&org, "org", "", "org name (required)")
	flag.StringVar(&name, "repo", "", "repo name (required)")
	flag.StringVar(&binary, "binary", "", "binary name (default: repo name)")
	flag.StringVar(&goMain, "main", "", "go main path (default: ./cmd/<binary>)")
	flag.StringVar(&desc, "desc", "", "description")
	flag.StringVar(&homepage, "homepage", "", "homepage URL")
	flag.StringVar(&license, "license", "MIT", "license identifier")
	flag.StringVar(&bundleVersion, "bundle-version", "v1.0.0", "kadena-bundle tag")
	flag.Parse()

	if err := run(cfgPath, statePath, org, name, binary, goMain, desc, homepage, license, bundleVersion); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run(cfgPath, statePath, org, name, binary, goMain, desc, homepage, license, bundleVersion string) error {
	if org == "" || name == "" {
		return fmt.Errorf("--org and --repo are required")
	}

	cfg, err := config.Load(cfgPath)
	if err != nil {
		return err
	}
	st, err := state.Load(statePath)
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

	// Confirm the repo exists.
	exists, err := c.RepoExists(ctx, org, name)
	if err != nil {
		return err
	}
	if !exists {
		return fmt.Errorf("%s/%s does not exist", org, name)
	}

	// Fill in defaults from flags or repo name.
	if binary == "" {
		binary = name
	}
	if goMain == "" {
		goMain = "./cmd/" + binary
	}
	if desc == "" {
		desc = binary
	}
	if homepage == "" {
		homepage = fmt.Sprintf("https://github.com/%s/%s", org, name)
	}

	// Interactive confirmation of all values.
	fmt.Println()
	fmt.Println("── Adopt:", org+"/"+name, "──")
	fmt.Println()

	if v, err := tui.PromptString("Binary name", binary); err != nil {
		return err
	} else {
		binary = v
	}
	if v, err := tui.PromptString("Go main path", goMain); err != nil {
		return err
	} else {
		goMain = v
	}
	if v, err := tui.PromptString("Description", desc); err != nil {
		return err
	} else {
		desc = v
	}
	if v, err := tui.PromptString("Homepage", homepage); err != nil {
		return err
	} else {
		homepage = v
	}
	if v, err := tui.PromptString("License", license); err != nil {
		return err
	} else {
		license = v
	}

	// Render and push both files.
	goreleaser := renderGoReleaser(org, name, binary, goMain, desc, homepage, license, bundleVersion)
	changed, err := repo.PushFile(ctx, c, org, name, repo.SourceGoReleaser,
		"Adopt: add GoReleaser config", []byte(goreleaser))
	if err != nil {
		return err
	}
	printResult(org, name, repo.SourceGoReleaser, changed)

	workflow := renderWorkflow(org, bundleVersion)
	changed, err = repo.PushFile(ctx, c, org, name, repo.SourceWorkflow,
		"Adopt: add release workflow", []byte(workflow))
	if err != nil {
		return err
	}
	printResult(org, name, repo.SourceWorkflow, changed)

	// Get the tap token — from state, or prompt.
	tapToken := lookupTapToken(st, org)
	if tapToken == "" {
		fmt.Println()
		fmt.Println("── Homebrew tap token ──")
		fmt.Println("  Fine-grained PAT with contents:write on homebrew-tap.")
		v, err := tui.PromptSecret("Tap token")
		if err != nil {
			return err
		}
		if v == "" {
			return fmt.Errorf("tap token is required")
		}
		tapToken = v
		recordTapToken(st, org, tapToken)
	}

	secretName := cfg.Tap.SecretName
	if secretName == "" {
		secretName = "HOMEBREW_TAP_GITHUB_TOKEN"
	}
	if err := repo.SetSecret(ctx, c, org, name, secretName, tapToken); err != nil {
		return fmt.Errorf("set secret %s: %w", secretName, err)
	}
	fmt.Printf("  ✓ secret %s on %s/%s\n", secretName, org, name)

	// Record in state.
	os := st.Get(org)
	os.ManagedRepos = appendUnique(os.ManagedRepos, name)
	st.Put(org, os)
	if err := state.Save(statePath, st); err != nil {
		return fmt.Errorf("save state: %w", err)
	}
	fmt.Printf("  ✓ %s/%s added to managed list\n", org, name)

	fmt.Println()
	fmt.Println("── Done ──")
	fmt.Println("  Next: tag a release to trigger the workflow")
	fmt.Println("    git tag v0.1.0 && git push origin v0.1.0")
	return nil
}

func printResult(org, name, path string, changed bool) {
	if changed {
		fmt.Printf("  ✓ %s/%s:%s\n", org, name, path)
	} else {
		fmt.Printf("  ✓ %s/%s:%s (unchanged)\n", org, name, path)
	}
}

func appendUnique(list []string, s string) []string {
	for _, v := range list {
		if v == s {
			return list
		}
	}
	return append(list, s)
}

// lookupTapToken looks for an org-specific tap token in state. The state
// schema stores these in OrgState.TapToken.
func lookupTapToken(st *state.State, org string) string {
	os := st.Get(org)
	return os.TapToken
}

// recordTapToken stores a tap token for an org in state.
func recordTapToken(st *state.State, org, token string) {
	os := st.Get(org)
	os.TapToken = token
	st.Put(org, os)
}

// renderGoReleaser builds the .goreleaser.yml for a repo that packages the
// kadena-bundle. Matches the config adopt-repo.sh writes today.
func renderGoReleaser(org, repoName, binary, goMain, desc, homepage, license, bundleVersion string) string {
	_ = bundleVersion // reserved for future cross-references; the workflow fetches the bundle
	return fmt.Sprintf(`version: 2

project_name: %s

before:
  hooks:
    - go mod tidy

builds:
  - id: %s
    main: %s
    binary: %s
    env:
      - CGO_ENABLED=0
    goos: [darwin, linux]
    goarch: [amd64, arm64]

archives:
  - id: %s
    formats: [tar.gz]
    name_template: "{{ .ProjectName }}_{{ .Version }}_{{ .Os }}_{{ .Arch }}"
    files:
      - src: kadena-bundle
        dst: "@kadena"

checksum:
  name_template: "checksums.txt"

release:
  github:
    owner: %s
    name: releases
    token: "{{ .Env.RELEASE_REPO_TOKEN }}"
  prerelease: auto
  replace_existing_draft: true
  use_existing_draft: true

brews:
  - name: %s
    repository:
      owner: %s
      name: homebrew-tap
      branch: main
      token: "{{ .Env.HOMEBREW_TAP_TOKEN }}"
    directory: Formula
    homepage: "%s"
    description: "%s"
    license: "%s"
    install: |
      bin.install "%s"
    test: |
      system "#{bin}/%s", "--version"
`,
		binary, binary, goMain, binary, binary,
		org, binary, org, homepage, desc, license, binary, binary)
}

// renderWorkflow builds the release workflow with the kadena-bundle fetch
// step. Matches what adopt-repo.sh writes today.
func renderWorkflow(org, bundleVersion string) string {
	return fmt.Sprintf(`name: Release

on:
  push:
    tags:
      - 'v*'

permissions:
  contents: write

jobs:
  goreleaser:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
        with:
          fetch-depth: 0

      - uses: actions/setup-go@v5
        with:
          go-version: 'stable'

      - name: Fetch @kadena bundle
        env:
          BUNDLE_ORG: ${{ github.repository_owner }}
          BUNDLE_VERSION: %s
        run: |
          set -euo pipefail
          ARCHIVE="kadena-bundle.tar.gz"
          URL="https://github.com/${BUNDLE_ORG}/kadena-bundle/archive/refs/tags/${BUNDLE_VERSION}.tar.gz"
          curl -sSL "$URL" -o "$ARCHIVE"
          mkdir -p _bundle_tmp
          tar -xzf "$ARCHIVE" -C _bundle_tmp --strip-components=1
          rm -rf kadena-bundle
          mv _bundle_tmp kadena-bundle
          rm -f "$ARCHIVE"
          echo "Bundled: $(cat kadena-bundle/VERSION)"
          ls -la kadena-bundle/

      - name: Run GoReleaser
        uses: goreleaser/goreleaser-action@v6
        with:
          distribution: goreleaser
          version: latest
          args: release --clean
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
          RELEASE_REPO_TOKEN: ${{ secrets.HOMEBREW_TAP_GITHUB_TOKEN }}
          HOMEBREW_TAP_GITHUB_TOKEN: ${{ secrets.HOMEBREW_TAP_GITHUB_TOKEN }}
          HOMEBREW_TAP_TOKEN: ${{ secrets.HOMEBREW_TAP_GITHUB_TOKEN }}
`, bundleVersion)
}

// _ ensures strings import isn't dropped if we later add string ops.
var _ = strings.TrimSpace

package bootstrap

import (
	"fmt"
)

// renderGoReleaser renders the .goreleaser.yml content for a source repo.
func renderGoReleaser(org, project string) string {
	return fmt.Sprintf(`version: 2

project_name: %s

before:
  hooks:
    - go mod tidy

builds:
  - id: %s
    main: ./cmd/%s
    binary: %s
    env:
      - CGO_ENABLED=0
    goos: [linux, darwin]
    goarch: [amd64, arm64]
    ldflags:
      - -s -w
      - -X main.version={{.Version}}

archives:
  - id: %s
    formats: [tar.gz]
    name_template: "{{ .ProjectName }}_{{ .Version }}_{{ .Os }}_{{ .Arch }}"
    files: [README.md, LICENSE]

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
  - repository:
      owner: %s
      name: homebrew-tap
      token: "{{ .Env.HOMEBREW_TAP_TOKEN }}"
    directory: Formula
    homepage: "https://github.com/%s/%s"
    description: "Releases for %s"
    license: "MIT"
    install: |
      bin.install "%s"
    test: |
      system "#{bin}/%s", "--version"
`,
		project, project, project, project, project,
		org, org, org, project, org, project, project)
}

// renderWorkflow returns the release workflow YAML.
func renderWorkflow() string {
	return `name: release

on:
  push:
    tags: ["v*"]

permissions:
  contents: read

jobs:
  goreleaser:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
        with:
          fetch-depth: 0

      - uses: actions/setup-go@v5
        with:
          go-version: "1.22"

      - uses: goreleaser/goreleaser-action@v6
        with:
          version: "~> v2"
          args: release --clean
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
          RELEASE_REPO_TOKEN: ${{ secrets.HOMEBREW_TAP_GITHUB_TOKEN }}
          HOMEBREW_TAP_TOKEN: ${{ secrets.HOMEBREW_TAP_GITHUB_TOKEN }}
`
}

package release

import "fmt"

// RenderGoReleaser produces the .goreleaser.yml for a source repo that ships
// a kadena-bundle. Used by adopt and by the release manager's adopt action.
func RenderGoReleaser(org, binary, goMain, desc, homepage, license, bundleVersion string) string {
	_ = bundleVersion
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

// RenderAdoptWorkflow produces the release workflow with the kadena-bundle
// fetch step.
func RenderAdoptWorkflow(bundleVersion string) string {
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

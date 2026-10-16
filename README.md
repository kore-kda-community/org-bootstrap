# org-bootstrap

One toolchain that takes a bare GitHub org and gives it the structure and release pipeline we use across our community.

Three binaries, all Go:

| Binary | Job |
|---|---|
| **`org-bootstrap`** | Interactive org scaffolder: `.github` profile, `releases` hub, `homebrew-tap`, source repos |
| **`org-audit`** | Secrets scanner run before any repo is promoted to public |
| **`org-release`** | Release manager: inspects tags, suggests next semver |

## What it does

For every selected org:

- **`.github`** — org profile README, regenerated from live org state on every run
- **`releases`** — central hub where tagged builds from every project land
- **`homebrew-tap`** — formulae, auto-bumped by GoReleaser on every tag
- **`<org-slug>`** — private source repo with `.goreleaser.yml` and release workflow

Repos are created **private first**, audited for secrets, then promoted to public only if the audit is clean. Source repos stay private.

## Install

    brew install kore-kda-community/tap/org-bootstrap

Or build from source:

    git clone https://github.com/kore-kda-community/homebrew-tap-app.git
    cd homebrew-tap-app
    go install ./cmd/...

## Quick start

    # 1. Config
    cp configs/example.toml config.toml
    $EDITOR config.toml

    # 2. Run the bootstrapper
    org-bootstrap

The first run walks you through authentication (OAuth device flow via our registered app), org selection, and per-org curation. Everything is saved to `.state.json` so re-runs skip every prompt you've already answered.

## Requirements

- **Go 1.22+** to build
- **`gh`** — used for auth fallback and by some internal calls
- **`git`** — cloning for the audit step
- **One fine-grained PAT** scoped to `<org>/homebrew-tap` with `Contents: Read and write`, used by GoReleaser at release time

## Layout

    cmd/org-bootstrap        Interactive CLI
    cmd/org-audit            Secrets scanner
    cmd/org-release          Release manager
    internal/config          TOML config loader + validator
    internal/state           JSON state persistence
    internal/gh              GitHub REST wrapper
    internal/oauth           Device flow + keyring persistence
    internal/profile         Org profile renderer
    internal/repo            Repo create / push / promote
    internal/audit           Secret scanner engine
    internal/visibility      Audit-then-promote gate
    internal/release         Tag scanning, semver
    internal/tap             Tap token + formula discovery
    internal/template        Go text/template wrapper
    internal/tui             Prompt helpers
    internal/bootstrap       Orchestrator
    internal/migrate         Import legacy dotfiles
    templates/               Go text/template files
    configs/                 Example TOML + JSON schemas

## Documentation

- **[docs/USAGE.md](docs/USAGE.md)** — full walkthrough of every prompt, config knob, and behaviour
- **[docs/ARCHITECTURE.md](docs/ARCHITECTURE.md)** — how the pieces fit together
- **[configs/example.toml](configs/example.toml)** — every config field with comments

## Migrating from the bash version

If you have the old `.main-org`, `.profile-private-repos` etc. dotfiles:

    org-bootstrap -migrate-from /path/to/old/dir

That imports the legacy state and merges it into `.state.json`. One-time only.

## License

MIT. See [LICENSE](LICENSE).

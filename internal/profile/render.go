package profile

import (
    "context"
    "fmt"
    "strings"

    gh "github.com/kore-kda-community/org-bootstrap/internal/gh"
    "github.com/kore-kda-community/org-bootstrap/internal/state"
    tmpl "github.com/kore-kda-community/org-bootstrap/internal/template"
)

const (
    defaultTagline     = "Kadena AI ecosystem tools and releases."
    defaultWhatWeBuild = "AI-focused development (community-based effort) for the Kadena Chainweb blockchain."
)

// Inputs holds everything Render needs.
type Inputs struct {
    Org   string
    State state.OrgState
    Opts  Options
}

// Render produces the profile README for an org.
func Render(ctx context.Context, c *gh.Client, in Inputs) ([]byte, error) {
    opts := in.Opts
    if opts.Tagline == "" {
        opts.Tagline = defaultTagline
    }
    if opts.WhatWeBuild == "" {
        opts.WhatWeBuild = defaultWhatWeBuild
    }
    if opts.TopN == 0 {
        opts.TopN = 10
    }

    repos := buildRepoList(ctx, c, in.Org, in.State.Profile.CuratedPublic,
        opts.IncludePins, opts.TopN)
    teasers := filterTeasers(ctx, c, in.Org, in.State.Profile.Teasers)

    data := TemplateData{
        Org:           in.Org,
        Slug:          strings.ToLower(in.Org),
        Tagline:       opts.Tagline,
        WhatWeBuild:   opts.WhatWeBuild,
        RepoList:      renderRepoList(in.Org, repos),
        TeaserSection: renderTeaserSection(opts.ShowTeasers, teasers),
        DevSection:    renderDevSection(ctx, c, opts.MainOrg),
    }

    return tmpl.Render("profile.md.tmpl", data)
}

// renderRepoList formats the "Repositories of Interest" bullet list.
func renderRepoList(org string, repos []Repo) string {
    if len(repos) == 0 {
        return "_No public repositories yet._\n"
    }
    var b strings.Builder
    for _, r := range repos {
        b.WriteString(fmt.Sprintf("- [`%s`](https://github.com/%s/%s)", r.Name, org, r.Name))
        if r.Description != "" {
            b.WriteString(" — ")
            b.WriteString(r.Description)
        }
        b.WriteString("\n")
    }
    return b.String()
}

// renderTeaserSection returns the "In Development" block, or "" if disabled
// or empty.
func renderTeaserSection(show bool, teasers []Teaser) string {
    if !show || len(teasers) == 0 {
        return ""
    }
    var b strings.Builder
    b.WriteString("\n## In Development\n\n")
    b.WriteString("Sneak preview — what we're working on next:\n\n")
    for _, t := range teasers {
        b.WriteString("- `")
        b.WriteString(t.Name)
        b.WriteString("`")
        if t.DisplaySlug != "" && t.DisplaySlug != t.Name {
            b.WriteString(" — ")
            b.WriteString(t.DisplaySlug)
        }
        b.WriteString("\n")
    }
    return b.String()
}

// renderDevSection returns the "For Developers" block if the tool repo exists
// in mainOrg, else "".
func renderDevSection(ctx context.Context, c *gh.Client, mainOrg string) string {
    if mainOrg == "" {
        return ""
    }
    exists, err := c.RepoExists(ctx, mainOrg, "homebrew-tap-app")
    if err != nil || !exists {
        return ""
    }
    return fmt.Sprintf(`
## For Developers: Bootstrap Your Own Org

Running your own org and want the same structure? The tool we use to set this up lives in our shared tooling org:

**[homebrew-tap-app](https://github.com/%s/homebrew-tap-app)** — a single script that scaffolds:

- **.github** — org profile README + shared community health files
- **releases** — central hub where every tagged build lands
- **homebrew-tap** — package manager formulae, auto-bumped by CI
- **Source repos** — one per project, with branch layout for project work
- **Release workflow** — GoReleaser config + GitHub Actions; tag -> build -> publish -> formula bump, fully automated
- **Release schedule** — driven by tags, so git tag vX.Y.Z && git push --tags is the whole workflow

Clone it, point it at your org, run ./run.sh. Safe to re-run.
`, mainOrg)
}

// filterTeasers keeps only teasers whose repo is actually private in this org.
func filterTeasers(ctx context.Context, c *gh.Client, org string, in []state.Teaser) []Teaser {
    if len(in) == 0 {
        return nil
    }
    privates, err := c.ListPrivateRepoNames(ctx, org)
    if err != nil {
        return nil
    }
    isPrivate := make(map[string]bool, len(privates))
    for _, n := range privates {
        isPrivate[n] = true
    }
    out := []Teaser{}
    for _, t := range in {
        if !isPrivate[t.Repo] {
            continue
        }
        out = append(out, Teaser{Name: t.Repo, DisplaySlug: t.Slug})
    }
    return out
}

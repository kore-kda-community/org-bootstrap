// Package profile renders an org's profile README from live GitHub state
// and curation stored in state.State.
package profile

// TemplateData is the payload passed to templates/profile.md.tmpl.
type TemplateData struct {
    Org           string
    Tagline       string
    WhatWeBuild   string
    RepoList      string
    TeaserSection string
    DevSection    string
    Slug          string // lowercase org, used for brew tap path
}

// Repo is a single entry in the "Repositories of Interest" list.
type Repo struct {
    Name        string
    Description string
    Pinned      bool
}

// Teaser is a single entry in the "In Development" list.
type Teaser struct {
    Name        string // repo name
    DisplaySlug string // repo name or custom slug
}

// Options controls how the profile is assembled.
type Options struct {
    MainOrg      string // for the "For Developers" section
    TopN         int    // max repos in the list
    IncludePins  bool   // whether to include GitHub-pinned repos
    ShowTeasers  bool
    Tagline      string // e.g. "Kadena AI ecosystem tools and releases."
    WhatWeBuild  string // e.g. "AI-focused development ..."
}

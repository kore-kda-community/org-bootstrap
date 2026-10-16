// Package state persists cross-run state to a JSON file.
package state

import "time"

// SchemaVersion is the current state file format version.
const SchemaVersion = 1

// State is the persisted JSON document.
type State struct {
	Version     int                 `json:"version"`
	UpdatedAt   time.Time           `json:"updated_at"`
	MainOrg     string              `json:"main_org,omitempty"`
	IncludePins string              `json:"include_pins,omitempty"`
	ExtraOrgs   []string            `json:"extra_orgs,omitempty"`
	Orgs        map[string]OrgState `json:"orgs"`
}

// AddExtraOrg appends org to ExtraOrgs if not already present.
func (s *State) AddExtraOrg(org string) {
	for _, e := range s.ExtraOrgs {
		if e == org {
			return
		}
	}
	s.ExtraOrgs = append(s.ExtraOrgs, org)
}

// PinsEnabled reports whether the user asked to include GitHub-pinned repos.
func (s *State) PinsEnabled() bool { return s.IncludePins == "y" }

// PinsDecided reports whether the user has been prompted about pins yet.
func (s *State) PinsDecided() bool { return s.IncludePins == "y" || s.IncludePins == "n" }

// OrgState is per-org persistent state.
type OrgState struct {
	Profile      ProfileState `json:"profile"`
	ManagedRepos []string     `json:"managed_repos"`
	LastRun      time.Time    `json:"last_run"`

	// TapToken is the PAT used by GoReleaser to push formulae to the
	// org's homebrew-tap. Stored per-org so one run can cover many orgs with
	// different tokens.
	TapToken string `json:"tap_token,omitempty"`
}

// ProfileState holds profile curation choices for an org.
type ProfileState struct {
	PinnedMode    string   `json:"pinned_mode"` // "include" | "ignore"
	CuratedPublic []string `json:"curated_public"`
	Teasers       []Teaser `json:"teasers"`
}

// Teaser is an "In Development" entry: a private repo name and its display slug.
type Teaser struct {
	Repo string `json:"repo"`
	Slug string `json:"slug"`
}

// Get returns the OrgState for org, creating one with defaults if missing.
func (s *State) Get(org string) OrgState {
	if s.Orgs == nil {
		s.Orgs = map[string]OrgState{}
	}
	if _, ok := s.Orgs[org]; !ok {
		s.Orgs[org] = OrgState{
			Profile: ProfileState{PinnedMode: "include"},
		}
	}
	return s.Orgs[org]
}

// Put stores the OrgState for org.
func (s *State) Put(org string, st OrgState) {
	if s.Orgs == nil {
		s.Orgs = map[string]OrgState{}
	}
	s.Orgs[org] = st
}

// SetTeaser adds or updates a teaser for org. Pass slug="" to use the repo name.
func (s *State) SetTeaser(org, repo, slug string) {
	st := s.Get(org)
	for i := range st.Profile.Teasers {
		if st.Profile.Teasers[i].Repo == repo {
			st.Profile.Teasers[i].Slug = slug
			s.Put(org, st)
			return
		}
	}
	st.Profile.Teasers = append(st.Profile.Teasers, Teaser{Repo: repo, Slug: slug})
	s.Put(org, st)
}

// RemoveTeaser deletes a teaser for org, if present.
func (s *State) RemoveTeaser(org, repo string) {
	st := s.Get(org)
	out := st.Profile.Teasers[:0]
	for _, t := range st.Profile.Teasers {
		if t.Repo != repo {
			out = append(out, t)
		}
	}
	st.Profile.Teasers = out
	s.Put(org, st)
}

// Package migrate imports the old dotfile-based state into .state.json.
package migrate

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"

	"github.com/kore-kda-community/org-bootstrap/internal/state"
)

// Legacy reads the pre-Go dotfiles (if present) and returns a State
// populated from them. Existing state is left untouched if the legacy
// files aren't found.
func Legacy(dir string) (*state.State, error) {
	st := &state.State{Version: state.SchemaVersion, Orgs: map[string]state.OrgState{}}

	if v := readLine(filepath.Join(dir, ".main-org")); v != "" {
		st.MainOrg = v
	}
	if v := readLine(filepath.Join(dir, ".profile-include-pins")); v != "" {
		st.IncludePins = v
	}

	// .profile-private-repos: "repo-name" or "repo-name | slug"
	teasers := readTeasers(filepath.Join(dir, ".profile-private-repos"))

	// .profile-public-repos: one repo per line
	curated := readLines(filepath.Join(dir, ".profile-public-repos"))

	// .managed-repos: "org/repo" per line
	managed := readLines(filepath.Join(dir, ".managed-repos"))

	// Seed each referenced org with what we found.
	seedOrg := func(org string) {
		os := st.Get(org)
		for _, c := range curated {
			os.Profile.CuratedPublic = append(os.Profile.CuratedPublic, c)
		}
		for _, t := range teasers {
			os.Profile.Teasers = append(os.Profile.Teasers, t)
		}
		st.Put(org, os)
	}

	// Orgs from managed repos
	orgSet := map[string]bool{}
	for _, line := range managed {
		if i := strings.IndexByte(line, '/'); i > 0 {
			orgSet[line[:i]] = true
		}
	}
	for org := range orgSet {
		seedOrg(org)
	}
	// Also any org named in .extra-orgs
	for _, org := range readLines(filepath.Join(dir, ".extra-orgs")) {
		orgSet[org] = true
		seedOrg(org)
	}

	return st, nil
}

func readLine(path string) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	if sc.Scan() {
		return strings.TrimSpace(sc.Text())
	}
	return ""
}

func readLines(path string) []string {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()
	var out []string
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		out = append(out, line)
	}
	return out
}

func readTeasers(path string) []state.Teaser {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()
	var out []state.Teaser
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "|", 2)
		name := strings.TrimSpace(parts[0])
		slug := ""
		if len(parts) == 2 {
			slug = strings.TrimSpace(parts[1])
		}
		out = append(out, state.Teaser{Repo: name, Slug: slug})
	}
	return out
}

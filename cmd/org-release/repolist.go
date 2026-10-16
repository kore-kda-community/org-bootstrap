package main

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/google/go-github/v60/github"
	"github.com/kore-kda-community/org-bootstrap/internal/config"
	"github.com/kore-kda-community/org-bootstrap/internal/gh"
	"github.com/kore-kda-community/org-bootstrap/internal/release"
	"github.com/kore-kda-community/org-bootstrap/internal/state"
)

// repoRow is one line of the repo table.
type repoRow struct {
	Name       string
	Private    bool
	Default    string
	Managed    bool
	LastTag    string
	LastAge    string
	LastWhen   string // YYYY-MM-DD
	RelCount   int
	BranchKind release.BranchKind
	BranchName string
}

// isInfraRepo reports whether name is one of the scaffolded repos that
// shouldn't appear in the project list.
func isInfraRepo(name string) bool {
	switch name {
	case "homebrew-tap", "releases", ".github", "releases-archive":
		return true
	}
	return strings.HasPrefix(name, "homebrew-")
}

// repoList renders the repo table for one org and handles selection.
// Loops until the user picks [b]ack or [q]uit.
func repoList(ctx context.Context, c *gh.Client, st *state.State, cfg *config.Config, org, statePath string) error {
	managedOnly := false

	for {
		fmt.Print("\033[H\033[2J") // clear screen

		title := fmt.Sprintf("Repos: %s", org)
		if managedOnly {
			title += "  (managed only)"
		}
		hdr("%s", title)

		rows, err := buildRepoRows(ctx, c, st, org, managedOnly)
		if err != nil {
			return err
		}
		if len(rows) == 0 {
			warn("no repos match filter")
			fmt.Println("    press [f] to show all")
		}

		renderRepoTable(rows)

		fmt.Println()
		fmt.Println("    [f] filter managed/all   [r] refresh   [b] back   [q] quit")
		pick := promptLine("  Repo (number), or action", "")

		switch strings.ToLower(pick) {
		case "f":
			managedOnly = !managedOnly
			continue
		case "r":
			continue
		case "":
			return nil
		case "b":
			return nil
		case "q":
			os.Exit(0)
		}

		n := parsePick(pick)
		if n < 1 || n > len(rows) {
			continue
		}
		row := rows[n-1]
		if !row.Managed {
			fmt.Println()
			warn("%s/%s is NOT managed", org, row.Name)
			if promptYN("Adopt it into release management now?", true) {
				adoptRepo(ctx, c, st, cfg, org, row.Name, statePath)
				pause()
				continue
			}
		}
		if err := repoActions(ctx, c, st, cfg, org, row); err != nil {
			fmt.Fprintf(os.Stderr, "  ! %v\n", err)
			pause()
		}
	}
}

// buildRepoRows fetches every non-infra repo for the org and computes the
// release info displayed in the table.
func buildRepoRows(ctx context.Context, c *gh.Client, st *state.State, org string, managedOnly bool) ([]repoRow, error) {
	repos, err := c.ListOrgRepos(ctx, org)
	if err != nil {
		return nil, err
	}

	os := st.Get(org)
	managedSet := map[string]bool{}
	for _, r := range os.ManagedRepos {
		managedSet[r] = true
	}

	var rows []repoRow
	for _, r := range repos {
		name := r.GetName()
		if isInfraRepo(name) {
			continue
		}
		managed := managedSet[name]
		if managedOnly && !managed {
			continue
		}

		row := repoRow{
			Name:    name,
			Private: r.GetPrivate(),
			Default: r.GetDefaultBranch(),
			Managed: managed,
		}

		// Release info. Prefer an actual release on the source repo, but fall
		// back to the newest tag — which is the case for every project that
		// publishes to a central <org>/releases hub instead of itself.
		rel, err := c.GetLatestRelease(ctx, org, name)
		if err == nil && rel != nil {
			row.LastTag = rel.GetTagName()
			if rel.PublishedAt != nil {
				row.LastWhen = rel.PublishedAt.Format("2006-01-02")
				row.LastAge = humanAge(rel.PublishedAt.Time)
			}
		} else {
			tags, _ := c.ListTags(ctx, org, name)
			if parsed := release.ParseTags(tags); len(parsed) > 0 {
				latest := parsed[len(parsed)-1]
				row.LastTag = latest.Raw
				if t, err := c.GetCommitDate(ctx, org, name, latest.Raw); err == nil && !t.IsZero() {
					row.LastWhen = t.Format("2006-01-02")
					row.LastAge = humanAge(t)
				}
			}
		}

		// Branch info: the release branch if any.
		branches, err := c.ListBranches(ctx, org, name)
		if err == nil {
			rb := release.FilterReleaseBranches(branches)
			if len(rb) > 0 {
				row.BranchName = rb[0].Name
				row.BranchKind = release.BranchRelease
			}
		}

		rows = append(rows, row)
	}
	return rows, nil
}

// renderRepoTable prints the table header and rows.
func renderRepoTable(rows []repoRow) {
	fmt.Printf("         %s%-6s %-1s %-28s %-12s %-8s %-16s %s%s\n",
		cDim, "VIS", "M", "REPO", "LAST REL", "AGE", "VERS", "BRANCH", cReset)
	fmt.Println()

	for i, r := range rows {
		vcolor := cGreen
		vlabel := "PUB"
		if r.Private {
			vcolor = cRed
			vlabel = "PRIV"
		}

		mmark := " "
		if r.Managed {
			mmark = cBold + "*" + cReset
		}

		relCell := cDim + "no releases" + cReset
		ageCell := "—"
		verCell := cDim + "—" + cReset
		if r.LastTag != "" {
			relCell = r.LastWhen
			ageCell = r.LastAge
			verCell = cCyan + truncate(r.LastTag, 14) + cReset
		}

		branchCell := cDim + "—" + cReset
		if r.BranchName != "" {
			branchCell = cMag + truncate(r.BranchName, 24) + cReset
		}

		fmt.Printf("    [%2d] %s%-4s%s  %s  %-28s %-12s %-8s %-16s %s\n",
			i+1, vcolor, vlabel, cReset, mmark,
			truncate(r.Name, 28),
			relCell,
			ageCell,
			verCell,
			branchCell,
		)
	}

	fmt.Println()
	fmt.Printf("    %sPUB%s=public  %sPRIV%s=private  %s*%s=managed\n",
		cGreen, cReset, cRed, cReset, cBold, cReset)
}

// parsePick converts "3" or "3x" into 3. Returns 0 for empty input.
func parsePick(s string) int {
	s = strings.TrimSpace(s)
	n := 0
	for _, r := range s {
		if r < '0' || r > '9' {
			break
		}
		n = n*10 + int(r-'0')
	}
	return n
}

// adoptRepo marks an existing repo as managed. If it already has a
// .goreleaser.yml, the config is left alone — only the secret and the
// managed-list entry are written.
func adoptRepo(ctx context.Context, c *gh.Client, st *state.State, cfg *config.Config, org, name, statePath string) {
	fmt.Println()
	fmt.Printf("  %sAdopting %s/%s%s\n", cBold, org, name, cReset)

	// Detect existing config before prompting for anything.
	hasGor, _ := c.FileExists(ctx, org, name, ".goreleaser.yml")
	if !hasGor {
		hasGor, _ = c.FileExists(ctx, org, name, ".goreleaser.yaml")
	}
	hasWorkflow, _ := c.FileExists(ctx, org, name, ".github/workflows/release.yml")
	alreadySetUp := hasGor && hasWorkflow

	if alreadySetUp {
		fmt.Printf("  %s· repo already has .goreleaser.yml + release.yml — skipping config writes%s\n", cDim, cReset)
	} else {
		fmt.Printf("  %s· no .goreleaser.yml found — will write default configs%s\n", cDim, cReset)
	}
	fmt.Println()

	binary := promptLine("Binary name", name)
	goMain := promptLine("Go main path", "./cmd/"+binary)
	desc := promptLine("Description", binary)
	homepage := promptLine("Homepage", "https://github.com/"+org+"/"+name)
	license := promptLine("License", "MIT")

	os := st.Get(org)
	token := os.TapToken
	if token == "" {
		fmt.Println()
		fmt.Println("  tap token required to set the repo secret")
		token = promptSecret("Tap token")
		if token == "" {
			errf("no token — aborting adopt")
			return
		}
		os.TapToken = token
		st.Put(org, os)
	}

	secretName := cfg.Tap.SecretName
	if secretName == "" {
		secretName = "HOMEBREW_TAP_GITHUB_TOKEN"
	}

	if !alreadySetUp {
		goreleaser := release.RenderGoReleaser(org, binary, goMain, desc, homepage, license, "v1.0.0")
		workflow := release.RenderAdoptWorkflow("v1.0.0")

		if _, err := c.PushFile(ctx, org, name, ".goreleaser.yml", []byte(goreleaser), "Adopt: add GoReleaser config"); err != nil {
			errf("push .goreleaser.yml: %v", err)
			return
		}
		ok("%s/%s:.goreleaser.yml", org, name)

		if _, err := c.PushFile(ctx, org, name, ".github/workflows/release.yml", []byte(workflow), "Adopt: add release workflow"); err != nil {
			errf("push workflow: %v", err)
			return
		}
		ok("%s/%s:.github/workflows/release.yml", org, name)
	} else {
		ok("existing configs left untouched")
	}

	if err := c.SetSecret(ctx, org, name, secretName, token); err != nil {
		errf("set secret: %v", err)
		return
	}
	ok("secret %s on %s/%s", secretName, org, name)

	os.ManagedRepos = appendUnique(os.ManagedRepos, name)
	st.Put(org, os)
	if statePath != "" {
		if err := state.Save(statePath, st); err != nil {
			errf("save state: %v", err)
		}
	}
	ok("%s/%s added to managed list", org, name)
}

func appendUnique(list []string, s string) []string {
	for _, v := range list {
		if v == s {
			return list
		}
	}
	return append(list, s)
}

// keep unused imports from biting during the incremental build.
var (
	_ = regexp.MustCompile
	_ *github.Repository
)

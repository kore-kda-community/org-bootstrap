package profile

import (
    "context"
    "sort"

    "github.com/google/go-github/v60/github"
    "github.com/kore-kda-community/org-bootstrap/internal/gh"
)

// metaRepos are repo names that never appear in the "Repositories of Interest"
// list even if public.
var metaRepos = map[string]bool{
    ".github":          true,
    "releases":         true,
    "homebrew-tap":     true,
    "homebrew-tap-app": true,
}

// buildRepoList assembles the ordered repo list. Pins first (if enabled),
// then curated picks (if any), then top-N public by stars until the cap is hit.
func buildRepoList(
    ctx context.Context,
    c *gh.Client,
    org string,
    curated []string,
    includePins bool,
    topN int,
) []Repo {
    seen := map[string]bool{}
    out := []Repo{}

    // 1. Pins first, if enabled
    if includePins {
        for _, p := range fetchPinned(ctx, c, org) {
            if metaRepos[p.Name] || seen[p.Name] {
                continue
            }
            seen[p.Name] = true
            out = append(out, p)
        }
    }

    // 2. Curated list, in caller-supplied order, filtered to public repos in
    // this org that weren't already pinned.
    if len(curated) > 0 {
        publicBySlug := fetchPublicIndex(ctx, c, org)
        for _, name := range curated {
            if metaRepos[name] || seen[name] {
                continue
            }
            r, ok := publicBySlug[name]
            if !ok {
                continue
            }
            seen[name] = true
            out = append(out, Repo{
                Name:        r.GetName(),
                Description: r.GetDescription(),
            })
        }
    }

    // 3. Fill remaining slots with top public by stars (excluding anything
    // already included), unless curation was provided and short — then we
    // still respect TopN.
    if topN > 0 && len(out) < topN {
        all := fetchPublicSorted(ctx, c, org)
        for _, r := range all {
            if len(out) >= topN {
                break
            }
            name := r.GetName()
            if metaRepos[name] || seen[name] {
                continue
            }
            seen[name] = true
            out = append(out, Repo{
                Name:        name,
                Description: r.GetDescription(),
            })
        }
    }

    return out
}

// fetchPublicIndex returns public repos keyed by name.
func fetchPublicIndex(ctx context.Context, c *gh.Client, org string) map[string]*github.Repository {
    repos, err := c.ListPublicRepos(ctx, org)
    if err != nil {
        return map[string]*github.Repository{}
    }
    out := make(map[string]*github.Repository, len(repos))
    for _, r := range repos {
        out[r.GetName()] = r
    }
    return out
}

// fetchPublicSorted returns public repos ordered by stars desc, then pushed desc.
func fetchPublicSorted(ctx context.Context, c *gh.Client, org string) []*github.Repository {
    repos, err := c.ListPublicRepos(ctx, org)
    if err != nil {
        return nil
    }
    sort.SliceStable(repos, func(i, j int) bool {
        if repos[i].GetStargazersCount() != repos[j].GetStargazersCount() {
            return repos[i].GetStargazersCount() > repos[j].GetStargazersCount()
        }
        return repos[i].GetPushedAt().After(repos[j].GetPushedAt().Time)
    })
    return repos
}

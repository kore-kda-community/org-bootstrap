package profile

import (
    "context"

    "github.com/kore-kda-community/org-bootstrap/internal/gh"
)

// fetchPinned returns the org's GitHub-pinned repos in display order.
// Returns an empty slice on error, since pinning is optional.
func fetchPinned(ctx context.Context, c *gh.Client, org string) []Repo {
    pins, err := c.ListPinnedRepos(ctx, org)
    if err != nil {
        return nil
    }
    out := make([]Repo, 0, len(pins))
    for _, p := range pins {
        out = append(out, Repo{
            Name:        p.Name,
            Description: p.Description,
            Pinned:      true,
        })
    }
    return out
}

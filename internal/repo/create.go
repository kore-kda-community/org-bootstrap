package repo

import (
	"context"
	"fmt"

	"github.com/kore-kda-community/org-bootstrap/internal/gh"
)

// Ensure creates the repo if missing, using the requested visibility.
// Returns (created, error). Logging is the caller function.
func Ensure(ctx context.Context, c *gh.Client, org, name, visibility, description string) (bool, error) {
	created, err := c.EnsureRepo(ctx, org, name, visibility, description)
	if err != nil {
		return false, fmt.Errorf("ensure %s/%s: %w", org, name, err)
	}
	return created, nil
}

// Exists reports whether the repo is present.
func Exists(ctx context.Context, c *gh.Client, org, name string) (bool, error) {
	return c.RepoExists(ctx, org, name)
}

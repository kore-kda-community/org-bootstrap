package repo

import (
	"context"
	"fmt"

	"github.com/kore-kda-community/org-bootstrap/internal/gh"
)

// PushFile writes content to org/name:path with the given commit message.
// Returns changed=true if the file was created or modified, false if it was
// already byte-identical.
func PushFile(ctx context.Context, c *gh.Client, org, name, path, message string, content []byte) (bool, error) {
	changed, err := c.PushFile(ctx, org, name, path, content, message)
	if err != nil {
		return false, fmt.Errorf("push %s/%s:%s: %w", org, name, path, err)
	}
	return changed, nil
}

// SetSecret upserts an Actions secret on org/name.
func SetSecret(ctx context.Context, c *gh.Client, org, name, secret, value string) error {
	if err := c.SetSecret(ctx, org, name, secret, value); err != nil {
		return fmt.Errorf("set secret %s on %s/%s: %w", secret, org, name, err)
	}
	return nil
}

// Promote flips visibility if the repo is not already at the target.
// No-op when already public.
func Promote(ctx context.Context, c *gh.Client, org, name string) error {
	return c.SetVisibility(ctx, org, name, "public")
}

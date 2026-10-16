package tap

import (
	"context"
	"strings"

	"github.com/kore-kda-community/org-bootstrap/internal/gh"
)

// ListFormulae returns the .rb filenames under Formula/ in org/homebrew-tap.
// Returns an empty slice (no error) if the directory or repo doesn't exist.
func ListFormulae(ctx context.Context, c *gh.Client, org string) ([]string, error) {
	entries, ok, err := c.ListDir(ctx, org, "homebrew-tap", "Formula")
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, nil
	}
	out := make([]string, 0, len(entries))
	for _, e := range entries {
		if strings.HasSuffix(e, ".rb") {
			out = append(out, e)
		}
	}
	return out, nil
}

package gh

import (
	"context"
	"fmt"
	"net/http"

	"github.com/google/go-github/v60/github"
)

// PushFile writes content to org/repo:path. Returns changed=true if created or
// updated, false if the file already contained identical bytes.
func (c *Client) PushFile(ctx context.Context, org, repo, path string, content []byte, message string) (bool, error) {
	var sha string
	file, _, resp, err := c.raw.Repositories.GetContents(ctx, org, repo, path, nil)
	switch {
	case err == nil && file != nil:
		sha = file.GetSHA()
		existing, derr := file.GetContent()
		if derr == nil && existing == string(content) {
			return false, nil
		}
	case err != nil && resp != nil && resp.StatusCode == http.StatusNotFound:
		// will create
	case err != nil:
		return false, fmt.Errorf("get contents %s/%s:%s: %w", org, repo, path, err)
	}

	opts := &github.RepositoryContentFileOptions{
		Message: github.String(message),
		Content: content,
	}
	if sha != "" {
		opts.SHA = github.String(sha)
	}
	if _, _, err := c.raw.Repositories.CreateFile(ctx, org, repo, path, opts); err != nil {
		return false, fmt.Errorf("push %s/%s:%s: %w", org, repo, path, err)
	}
	return true, nil
}

// ReadFile fetches and decodes a file. Returns (nil, false, nil) if missing.
func (c *Client) ReadFile(ctx context.Context, org, repo, path string) ([]byte, bool, error) {
	file, _, resp, err := c.raw.Repositories.GetContents(ctx, org, repo, path, nil)
	if err != nil {
		if resp != nil && resp.StatusCode == http.StatusNotFound {
			return nil, false, nil
		}
		return nil, false, err
	}
	if file == nil {
		return nil, false, nil
	}
	data, err := file.GetContent()
	if err != nil {
		return nil, false, err
	}
	return []byte(data), true, nil
}

// SetSecret upserts an Actions secret on org/repo. Value is plaintext;
// the public key fetch and libsodium sealing are handled internally.
func (c *Client) SetSecret(ctx context.Context, org, repo, name, value string) error {
	pk, _, err := c.raw.Actions.GetRepoPublicKey(ctx, org, repo)
	if err != nil {
		return fmt.Errorf("get public key %s/%s: %w", org, repo, err)
	}
	sealed, err := sealSecret(pk.GetKey(), value)
	if err != nil {
		return fmt.Errorf("seal secret %s: %w", name, err)
	}
	_, err = c.raw.Actions.CreateOrUpdateRepoSecret(ctx, org, repo, &github.EncryptedSecret{
		Name:           name,
		KeyID:          pk.GetKeyID(),
		EncryptedValue: sealed,
	})
	if err != nil {
		return fmt.Errorf("set secret %s on %s/%s: %w", name, org, repo, err)
	}
	return nil
}

// ListDir returns the names of entries in org/repo:path. Returns
// (nil, false, nil) if the directory doesn't exist.
func (c *Client) ListDir(ctx context.Context, org, repo, path string) ([]string, bool, error) {
	_, entries, resp, err := c.raw.Repositories.GetContents(ctx, org, repo, path, nil)
	if err != nil {
		if resp != nil && resp.StatusCode == http.StatusNotFound {
			return nil, false, nil
		}
		return nil, false, err
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		names = append(names, e.GetName())
	}
	return names, true, nil
}

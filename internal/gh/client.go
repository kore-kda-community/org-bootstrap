// Package gh wraps the GitHub REST API for org-bootstrap's needs.
package gh

import (
    "context"
    "fmt"
    "net/http"

    "github.com/google/go-github/v60/github"
    "golang.org/x/oauth2"
)

// Client is the concrete GitHub client. Tests may supply a custom *http.Client
// via NewWithHTTP to intercept calls.
type Client struct {
    raw *github.Client
}

// New builds a Client authenticated with a static bearer token.
func New(ctx context.Context, token string) (*Client, error) {
    if token == "" {
        return nil, fmt.Errorf("gh: empty token")
    }
    ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token})
    hc := oauth2.NewClient(ctx, ts)
    return NewWithHTTP(hc), nil
}

// NewWithHTTP is used by tests to inject a transport (e.g. httptest server).
func NewWithHTTP(hc *http.Client) *Client {
    return &Client{raw: github.NewClient(hc)}
}

// Raw exposes the underlying go-github client for callers that need
// endpoints this wrapper doesn't cover.
func (c *Client) Raw() *github.Client {
    return c.raw
}

// Org is a trimmed representation used by the picker and profile renderer.
type Org struct {
    Login       string
    Description string
}

// PinnedRepo is a repo the org has pinned on GitHub.
type PinnedRepo struct {
    Name        string
    Description string
}

package gh

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"

	"github.com/google/go-github/v60/github"
)

// ListAdminOrgs returns orgs where the authenticated user has the admin role.
// Falls back to /user/orgs if memberships returns nothing (some tokens lack
// the admin:org scope on /memberships but still list orgs).
func (c *Client) ListAdminOrgs(ctx context.Context) ([]Org, error) {
	var out []Org
	opts := &github.ListOrgMembershipsOptions{
		ListOptions: github.ListOptions{PerPage: 100},
	}
	for {
		ms, resp, err := c.raw.Organizations.ListOrgMemberships(ctx, opts)
		if err != nil {
			break
		}
		for _, m := range ms {
			if m.GetRole() != "admin" || m.Organization == nil {
				continue
			}
			out = append(out, Org{
				Login:       m.Organization.GetLogin(),
				Description: m.Organization.GetDescription(),
			})
		}
		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}
	if len(out) > 0 {
		return out, nil
	}
	return c.listUserOrgs(ctx)
}

func (c *Client) listUserOrgs(ctx context.Context) ([]Org, error) {
	var out []Org
	opts := &github.ListOptions{PerPage: 100}
	for {
		orgs, resp, err := c.raw.Organizations.List(ctx, "", opts)
		if err != nil {
			return nil, err
		}
		for _, o := range orgs {
			out = append(out, Org{
				Login:       o.GetLogin(),
				Description: o.GetDescription(),
			})
		}
		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}
	return out, nil
}

// ListPinnedRepos queries the org profile's pinned items via GraphQL.
// Returns an empty slice (no error) if the org has no pins.
func (c *Client) ListPinnedRepos(ctx context.Context, org string) ([]PinnedRepo, error) {
	const q = `query($org: String!) {
      organization(login: $org) {
        pinnedItems(first: 6, types: REPOSITORY) {
          nodes { ... on Repository { name description } }
        }
      }
    }`

	body, err := json.Marshal(map[string]any{
		"query":     q,
		"variables": map[string]string{"org": org},
	})
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		"https://api.github.com/graphql", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	var resp struct {
		Data struct {
			Organization struct {
				PinnedItems struct {
					Nodes []struct {
						Name        string `json:"name"`
						Description string `json:"description"`
					} `json:"nodes"`
				} `json:"pinnedItems"`
			} `json:"organization"`
		} `json:"data"`
	}
	if _, err := c.raw.Do(ctx, req, &resp); err != nil {
		return nil, err
	}

	nodes := resp.Data.Organization.PinnedItems.Nodes
	out := make([]PinnedRepo, 0, len(nodes))
	for _, n := range nodes {
		out = append(out, PinnedRepo{Name: n.Name, Description: n.Description})
	}
	return out, nil
}

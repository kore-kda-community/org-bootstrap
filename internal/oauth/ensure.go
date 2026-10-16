package oauth

import (
    "context"
    "fmt"
    "os"

    "github.com/kore-kda-community/org-bootstrap/internal/config"
)

// EnsureToken returns a GitHub access token, in this order:
//
//  1. $GH_TOKEN or $GITHUB_TOKEN if already set in the environment.
//  2. The token stored in gh's credential store (via `gh auth token`).
//  3. A fresh OAuth device flow using cfg.ClientID and cfg.Scopes.
//
// After a successful device flow the token is persisted into gh's keyring so
// subsequent runs skip the flow. onPrompt is optional; pass nil for silent
// operation.
func EnsureToken(ctx context.Context, cfg config.Auth, onPrompt Prompt) (string, error) {
    if t := envToken(); t != "" {
        return t, nil
    }

    if t, err := ghTokenFromKeyring(); err == nil && t != "" {
        return t, nil
    }

    if cfg.ClientID == "" {
        return "", fmt.Errorf("oauth: no client_id configured and no stored token")
    }

    token, err := DeviceFlow(ctx, cfg.ClientID, cfg.Scopes, onPrompt)
    if err != nil {
        return "", err
    }

    // Best-effort persist. Non-fatal on failure.
    _ = persistToKeyring(token)
    return token, nil
}

func envToken() string {
    if v := os.Getenv("GH_TOKEN"); v != "" {
        return v
    }
    return os.Getenv("GITHUB_TOKEN")
}

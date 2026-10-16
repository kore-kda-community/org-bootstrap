// Package oauth implements GitHub's OAuth device flow plus keyring persistence.
package oauth

import (
    "context"
    "encoding/json"
    "fmt"
    "io"
    "net/http"
    "net/url"
    "strings"
    "time"
)

const (
    deviceCodeURL  = "https://github.com/login/device/code"
    accessTokenURL = "https://github.com/login/oauth/access_token"
    defaultInterval = 5 * time.Second
)

// DeviceCode is the initial response from /login/device/code.
type DeviceCode struct {
    DeviceCode      string `json:"device_code"`
    UserCode        string `json:"user_code"`
    VerificationURI string `json:"verification_uri"`
    ExpiresIn       int    `json:"expires_in"`
    Interval        int    `json:"interval"`
}

// Prompt is called by DeviceFlow when the user needs to authorize. It receives
// the user_code and verification_uri; callers print them and optionally open
// the browser.
type Prompt func(userCode, verificationURI string)

// DeviceFlow runs the GitHub OAuth device flow. It blocks until the user
// authorizes, the code expires, or the context is cancelled.
func DeviceFlow(ctx context.Context, clientID string, scopes []string, onPrompt Prompt) (string, error) {
    dc, err := requestDeviceCode(ctx, clientID, scopes)
    if err != nil {
        return "", err
    }
    if onPrompt != nil {
        onPrompt(dc.UserCode, dc.VerificationURI)
    }

    interval := time.Duration(dc.Interval) * time.Second
    if interval <= 0 {
        interval = defaultInterval
    }
    expires := time.Duration(dc.ExpiresIn) * time.Second
    if expires <= 0 {
        expires = 15 * time.Minute
    }

    deadline := time.Now().Add(expires)
    for time.Now().Before(deadline) {
        select {
        case <-ctx.Done():
            return "", ctx.Err()
        case <-time.After(interval):
        }

        token, err := pollAccessToken(ctx, clientID, dc.DeviceCode)
        if err == nil && token != "" {
            return token, nil
        }
        if err == errSlowDown {
            interval += 5 * time.Second
            continue
        }
        if err == errPending {
            continue
        }
        if err != nil {
            return "", err
        }
    }
    return "", fmt.Errorf("device code expired")
}

func requestDeviceCode(ctx context.Context, clientID string, scopes []string) (*DeviceCode, error) {
    form := url.Values{}
    form.Set("client_id", clientID)
    form.Set("scope", strings.Join(scopes, " "))

    req, err := http.NewRequestWithContext(ctx, http.MethodPost, deviceCodeURL,
        strings.NewReader(form.Encode()))
    if err != nil {
        return nil, err
    }
    req.Header.Set("Accept", "application/json")
    req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

    resp, err := http.DefaultClient.Do(req)
    if err != nil {
        return nil, fmt.Errorf("request device code: %w", err)
    }
    defer resp.Body.Close()

    body, _ := io.ReadAll(resp.Body)
    if resp.StatusCode != http.StatusOK {
        return nil, fmt.Errorf("device code HTTP %d: %s", resp.StatusCode, string(body))
    }

    var dc DeviceCode
    if err := json.Unmarshal(body, &dc); err != nil {
        return nil, fmt.Errorf("parse device code: %w (body=%s)", err, string(body))
    }
    if dc.DeviceCode == "" || dc.UserCode == "" {
        return nil, fmt.Errorf("device code response missing fields: %s", string(body))
    }
    return &dc, nil
}

var (
    errPending  = fmt.Errorf("authorization_pending")
    errSlowDown = fmt.Errorf("slow_down")
)

func pollAccessToken(ctx context.Context, clientID, deviceCode string) (string, error) {
    form := url.Values{}
    form.Set("client_id", clientID)
    form.Set("device_code", deviceCode)
    form.Set("grant_type", "urn:ietf:params:oauth:grant-type:device_code")

    req, err := http.NewRequestWithContext(ctx, http.MethodPost, accessTokenURL,
        strings.NewReader(form.Encode()))
    if err != nil {
        return "", err
    }
    req.Header.Set("Accept", "application/json")
    req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

    resp, err := http.DefaultClient.Do(req)
    if err != nil {
        return "", fmt.Errorf("poll access token: %w", err)
    }
    defer resp.Body.Close()

    body, _ := io.ReadAll(resp.Body)

    var parsed struct {
        AccessToken string `json:"access_token"`
        Error       string `json:"error"`
        Description string `json:"error_description"`
    }
    if err := json.Unmarshal(body, &parsed); err != nil {
        return "", fmt.Errorf("parse token response: %w (body=%s)", err, string(body))
    }

    switch parsed.Error {
    case "":
        if parsed.AccessToken == "" {
            return "", fmt.Errorf("empty access token: %s", string(body))
        }
        return parsed.AccessToken, nil
    case "authorization_pending":
        return "", errPending
    case "slow_down":
        return "", errSlowDown
    case "expired_token":
        return "", fmt.Errorf("device code expired")
    case "access_denied":
        return "", fmt.Errorf("user denied authorization")
    default:
        return "", fmt.Errorf("oauth error %q: %s", parsed.Error, parsed.Description)
    }
}

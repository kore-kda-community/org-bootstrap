package oauth

import (
    "bytes"
    "fmt"
    "os"
    "os/exec"
    "strings"
)

// ghTokenFromKeyring shells out to `gh auth token` and returns the stored
// token. Returns ("", nil) if gh isn't authenticated or the binary is missing.
func ghTokenFromKeyring() (string, error) {
    cmd := exec.Command("gh", "auth", "token", "--hostname", "github.com")
    var stdout, stderr bytes.Buffer
    cmd.Stdout = &stdout
    cmd.Stderr = &stderr
    if err := cmd.Run(); err != nil {
        // Not authenticated, gh missing, etc. — treat as "no token".
        return "", nil
    }
    return strings.TrimSpace(stdout.String()), nil
}

// persistToKeyring writes token into gh's credential store via
// `gh auth login --with-token`. GH_TOKEN and GITHUB_TOKEN are stripped from
// the child environment because gh refuses to persist when they are set.
func persistToKeyring(token string) error {
    cmd := exec.Command("gh", "auth", "login",
        "--hostname", "github.com",
        "--with-token")
    cmd.Stdin = strings.NewReader(token + "\n")
    cmd.Env = filterEnv(os.Environ(), "GH_TOKEN", "GITHUB_TOKEN")

    var stderr bytes.Buffer
    cmd.Stderr = &stderr
    if err := cmd.Run(); err != nil {
        return fmt.Errorf("gh auth login --with-token: %w: %s",
            err, strings.TrimSpace(stderr.String()))
    }
    return nil
}

// filterEnv returns env without entries whose keys match any of the given
// names (case-sensitive). Used to strip GH_TOKEN before shelling out.
func filterEnv(env []string, names ...string) []string {
    drop := make(map[string]struct{}, len(names))
    for _, n := range names {
        drop[n] = struct{}{}
    }
    out := env[:0:0]
    for _, kv := range env {
        eq := strings.IndexByte(kv, '=')
        if eq < 0 {
            out = append(out, kv)
            continue
        }
        if _, hit := drop[kv[:eq]]; hit {
            continue
        }
        out = append(out, kv)
    }
    return out
}

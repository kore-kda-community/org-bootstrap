// Package tap handles Homebrew tap token resolution and formula discovery.
package tap

import (
	"fmt"
	"os"
	"strings"
)

// TokenFromEnv reads the tap token from an environment variable. Returns an
// error if the variable is empty.
func TokenFromEnv(envName string) (string, error) {
	if envName == "" {
		envName = "HOMEBREW_TAP_GITHUB_TOKEN"
	}
	v := strings.TrimSpace(os.Getenv(envName))
	if v == "" {
		return "", fmt.Errorf("tap token not set in $%s", envName)
	}
	return v, nil
}

// Mask returns a redacted form of the token for logging (first 4 + ellipsis).
func Mask(token string) string {
	if len(token) <= 8 {
		return "***"
	}
	return token[:4] + "…"
}

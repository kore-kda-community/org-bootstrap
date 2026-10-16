package gh

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"

	"golang.org/x/crypto/nacl/box"
)

// sealSecret encrypts plaintext for GitHub's Actions secret API using a
// libsodium sealed box. Result is base64-encoded for the JSON payload.
func sealSecret(publicKeyB64, plaintext string) (string, error) {
	var pk [32]byte
	raw, err := base64.StdEncoding.DecodeString(publicKeyB64)
	if err != nil {
		return "", fmt.Errorf("decode public key: %w", err)
	}
	if len(raw) != 32 {
		return "", fmt.Errorf("public key must be 32 bytes, got %d", len(raw))
	}
	copy(pk[:], raw)

	sealed, err := box.SealAnonymous(nil, []byte(plaintext), &pk, rand.Reader)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(sealed), nil
}

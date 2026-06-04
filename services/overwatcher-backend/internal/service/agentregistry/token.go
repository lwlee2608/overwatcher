package agentregistry

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
)

// TokenPrefix marks an agent credential so it's greppable in logs and secret
// scanners. It is part of the value the agent sends and is covered by the hash.
const TokenPrefix = "owa_"

// generateToken returns a fresh opaque token (≥256 bits of entropy, URL-safe,
// prefixed) and its sha256 digest. Only the digest is stored; the raw token is
// surfaced to the caller exactly once.
func generateToken() (raw, hash string, err error) {
	buf := make([]byte, 32)
	if _, err = rand.Read(buf); err != nil {
		return "", "", err
	}
	raw = TokenPrefix + base64.RawURLEncoding.EncodeToString(buf)
	return raw, hashToken(raw), nil
}

// hashToken computes the lowercase-hex sha256 digest of the whole token string
// (prefix included). Lookup hashes the presented token and matches this value.
func hashToken(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

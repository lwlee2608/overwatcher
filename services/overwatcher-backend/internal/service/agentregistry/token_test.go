package agentregistry

import "testing"

func TestValidToken(t *testing.T) {
	raw, _, err := generateToken()
	if err != nil {
		t.Fatalf("generateToken: %v", err)
	}
	if !validToken(raw) {
		t.Errorf("freshly minted token %q rejected", raw)
	}

	bad := []string{
		"",                     // empty
		"x",                    // attacker-chosen weak value
		"password123",          // no prefix
		"owa_",                 // prefix only, no body
		"owa_short",            // prefix but too few bytes
		raw[len(TokenPrefix):], // body without prefix
	}
	for _, tok := range bad {
		if validToken(tok) {
			t.Errorf("expected %q to be rejected", tok)
		}
	}
}

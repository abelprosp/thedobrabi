package sso

import (
	"strings"
	"testing"
)

func TestParseSAMLResponseRejectsWithoutCrypto(t *testing.T) {
	// Even a response that contains a literal "Signature" must not be accepted.
	fake := "PHNhbWxwOlJlc3BvbnNlPjxkczpTaWduYXR1cmU+ZmFrZTwvZHM6U2lnbmF0dXJlPjwvc2FtbHA6UmVzcG9uc2U+"
	_, _, _, err := ParseSAMLResponse(fake)
	if err == nil {
		t.Fatal("expected error for unverified SAML response")
	}
	if !strings.Contains(err.Error(), "validação criptográfica") {
		t.Fatalf("unexpected error: %v", err)
	}
}

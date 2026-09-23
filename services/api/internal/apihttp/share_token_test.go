package apihttp

import (
	"testing"

	"github.com/thedobra/thedobra/services/api/internal/cryptoenc"
)

func TestShareTokenHashRoundTrip(t *testing.T) {
	plain, err := cryptoenc.RandomToken(18)
	if err != nil {
		t.Fatal(err)
	}
	hashed := cryptoenc.HashToken(plain)
	if len(hashed) != 64 {
		t.Fatalf("SHA-256 hex length want 64, got %d", len(hashed))
	}
	if hashed == plain {
		t.Fatal("hash must differ from plain")
	}
	if cryptoenc.HashToken(plain) != hashed {
		t.Fatal("HashToken must be deterministic")
	}
	// Dual-read semantics: lookup matches either stored hash or legacy plain.
	storedHash := hashed
	storedPlain := plain
	incoming := plain
	matchHash := cryptoenc.HashToken(incoming) == storedHash || incoming == storedHash
	matchPlain := cryptoenc.HashToken(incoming) == storedPlain || incoming == storedPlain
	if !matchHash {
		t.Fatal("hashed row must match HashToken(incoming)")
	}
	if !matchPlain {
		t.Fatal("legacy plaintext row must match incoming")
	}
}

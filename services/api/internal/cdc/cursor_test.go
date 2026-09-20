package cdc

import "testing"

func TestEncodeDecodeCursor(t *testing.T) {
	enc := encodeCursor("2026-01-02T03:04:05Z", "42")
	v, pk := decodeCursor(enc)
	if v != "2026-01-02T03:04:05Z" || pk != "42" {
		t.Fatalf("got %q %q", v, pk)
	}
	v, pk = decodeCursor("plain")
	if v != "plain" || pk != "" {
		t.Fatalf("plain got %q %q", v, pk)
	}
	if encodeCursor("x", "") != "x" {
		t.Fatal("empty pk should not add sep")
	}
}

package apihttp

import (
	"net/http"
	"testing"
)

func TestRequestIPUsesXFFBehindLoopback(t *testing.T) {
	r := &http.Request{
		RemoteAddr: "127.0.0.1:54321",
		Header:     http.Header{"X-Forwarded-For": []string{"203.0.113.9, 10.0.0.1"}},
	}
	if got := requestIP(r); got != "203.0.113.9" {
		t.Fatalf("got %q, want client IP from XFF", got)
	}
}

func TestRequestIPIgnoresXFFWhenNotLoopback(t *testing.T) {
	r := &http.Request{
		RemoteAddr: "198.51.100.2:443",
		Header:     http.Header{"X-Forwarded-For": []string{"203.0.113.9"}},
	}
	if got := requestIP(r); got != "198.51.100.2" {
		t.Fatalf("got %q, want RemoteAddr host", got)
	}
}

package ingest

import (
	"net/http"
	"os"
	"testing"
	"time"
)

func TestMain(m *testing.M) {
	// Injectable plain client so httptest loopback works without weakening production SSRF.
	SetConnectorHTTP(&http.Client{Timeout: 20 * time.Second})
	code := m.Run()
	SetConnectorHTTP(nil)
	os.Exit(code)
}

package ingest

import (
	"net"
	"testing"
)

func withPrivateHostsDenied(t *testing.T) {
	t.Helper()
	t.Setenv("CONNECTOR_ALLOW_PRIVATE", "")
	connectorMu.Lock()
	prev := allowPrivateConnectorHosts
	allowPrivateConnectorHosts = false
	connectorMu.Unlock()
	t.Cleanup(func() {
		connectorMu.Lock()
		allowPrivateConnectorHosts = prev
		connectorMu.Unlock()
	})
}

func TestAssertPublicHost_blocksPrivateAndLoopback(t *testing.T) {
	withPrivateHostsDenied(t)
	for _, host := range []string{"127.0.0.1", "::1", "10.0.0.1", "192.168.1.1", "169.254.169.254", "localhost"} {
		if err := AssertPublicHost(host); err == nil {
			t.Fatalf("expected block for %q", host)
		}
	}
}

func TestAssertPublicHost_allowPrivateEnv(t *testing.T) {
	t.Setenv("CONNECTOR_ALLOW_PRIVATE", "1")
	if err := AssertPublicHost("127.0.0.1"); err != nil {
		t.Fatalf("expected allow with CONNECTOR_ALLOW_PRIVATE=1: %v", err)
	}
}

func TestBlockedConnectorIP(t *testing.T) {
	if !BlockedConnectorIP(net.ParseIP("127.0.0.1")) {
		t.Fatal("loopback should be blocked")
	}
	if !BlockedConnectorIP(net.ParseIP("10.1.2.3")) {
		t.Fatal("private should be blocked")
	}
	if BlockedConnectorIP(net.ParseIP("8.8.8.8")) {
		t.Fatal("public should not be blocked")
	}
}

func TestAssertConnectorConfig_brokerAndURL(t *testing.T) {
	withPrivateHostsDenied(t)
	if err := AssertConnectorConfig(SQLConfig{Broker: "10.0.0.5:9092"}); err == nil {
		t.Fatal("expected kafka private broker blocked")
	}
	if err := AssertConnectorConfig(SQLConfig{URL: "postgres://u:p@192.168.0.9:5432/db"}); err == nil {
		t.Fatal("expected URL private host blocked")
	}
}

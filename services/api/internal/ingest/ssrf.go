package ingest

import (
	"context"
	"fmt"
	"net"
	"net/netip"
	"net/url"
	"os"
	"strings"
	"time"
)

// AssertPublicHost resolves host and rejects private, loopback, link-local,
// multicast, unspecified, and cloud-metadata addresses (SSRF guard for TCP
// connectors). Set CONNECTOR_ALLOW_PRIVATE=1 to allow RFC1918/loopback hosts
// for local development (e.g. Docker Postgres on 172.x or localhost).
// Public cloud hostnames (Snowflake, etc.) continue to work when they resolve
// to public addresses.
func AssertPublicHost(host string) error {
	if privateHostsAllowed() {
		return nil
	}
	host = strings.TrimSpace(host)
	if host == "" {
		return fmt.Errorf("host obrigatório")
	}
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}
	host = strings.Trim(host, "[]")
	if host == "" {
		return fmt.Errorf("host obrigatório")
	}

	if ip, err := netip.ParseAddr(host); err == nil {
		if BlockedConnectorIP(net.IP(ip.AsSlice())) {
			return fmt.Errorf("host interno ou reservado não permitido")
		}
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	ips, err := net.DefaultResolver.LookupIP(ctx, "ip", host)
	if err != nil {
		return fmt.Errorf("falha ao resolver host")
	}
	if len(ips) == 0 {
		return fmt.Errorf("falha ao resolver host")
	}
	for _, ip := range ips {
		if BlockedConnectorIP(ip) {
			return fmt.Errorf("host interno ou reservado não permitido")
		}
	}
	return nil
}

// BlockedConnectorIP reports whether ip is unsuitable as a connector target
// (loopback, private, link-local, multicast, unspecified, or link-local metadata).
func BlockedConnectorIP(ip net.IP) bool {
	addr, err := netip.ParseAddr(ip.String())
	if err != nil {
		return true
	}
	return addr.IsLoopback() || addr.IsPrivate() || addr.IsLinkLocalUnicast() ||
		addr.IsLinkLocalMulticast() || addr.IsUnspecified() || addr.IsMulticast() ||
		addr.String() == "169.254.169.254"
}

func privateHostsAllowed() bool {
	if strings.TrimSpace(os.Getenv("CONNECTOR_ALLOW_PRIVATE")) == "1" {
		return true
	}
	connectorMu.RLock()
	defer connectorMu.RUnlock()
	return allowPrivateConnectorHosts
}

// AssertConnectorConfig rejects private/reserved endpoints present in a
// connector SQLConfig (Host, URL, Broker, Snowflake Account).
func AssertConnectorConfig(cfg SQLConfig) error {
	return assertConfigHosts(cfg)
}

// assertConfigHosts checks Host, URL hostname, Broker, and Snowflake Account
// before opening TCP connections from connector configs.
func assertConfigHosts(cfg SQLConfig) error {
	seen := map[string]struct{}{}
	add := func(h string) error {
		h = strings.TrimSpace(h)
		if h == "" {
			return nil
		}
		if _, ok := seen[h]; ok {
			return nil
		}
		seen[h] = struct{}{}
		return AssertPublicHost(h)
	}

	if err := add(cfg.Host); err != nil {
		return err
	}
	if raw := strings.TrimSpace(cfg.URL); raw != "" {
		if u, err := url.Parse(raw); err == nil {
			if err := add(u.Hostname()); err != nil {
				return err
			}
		}
	}
	if err := add(brokerHostname(cfg.Broker)); err != nil {
		return err
	}
	if acc := strings.TrimSpace(cfg.Account); acc != "" && strings.TrimSpace(cfg.Host) == "" {
		acc = strings.TrimSuffix(acc, ".snowflakecomputing.com")
		if err := add(acc + ".snowflakecomputing.com"); err != nil {
			return err
		}
	}
	return nil
}

func brokerHostname(broker string) string {
	b := strings.TrimSpace(broker)
	if b == "" {
		return ""
	}
	if strings.Contains(b, "://") {
		if u, err := url.Parse(b); err == nil {
			return u.Hostname()
		}
	}
	if h, _, err := net.SplitHostPort(b); err == nil {
		return h
	}
	return b
}

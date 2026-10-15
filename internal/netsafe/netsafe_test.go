package netsafe

import (
	"context"
	"net/netip"
	"testing"
)

func TestAllowed(t *testing.T) {
	for _, s := range []string{"127.0.0.1", "10.1.2.3", "192.168.1.1", "172.16.0.1", "169.254.169.254", "100.64.0.1",
		"0.0.0.0", "::1", "fc00::1", "fe80::1", "::ffff:127.0.0.1", "64:ff9b::a00:1"} {
		if allowed(netip.MustParseAddr(s)) {
			t.Errorf("%s should be blocked", s)
		}
	}
	for _, s := range []string{"1.1.1.1", "8.8.8.8", "2606:4700::1111"} {
		if !allowed(netip.MustParseAddr(s)) {
			t.Errorf("%s should be allowed", s)
		}
	}
}

func TestFetchRefusesBadURLs(t *testing.T) {
	for _, u := range []string{"file:///etc/passwd", "gopher://x", "http://127.0.0.1/", "http://localhost:22/", "ftp://example.com/a.png"} {
		if _, _, err := Fetch(context.Background(), u, 1<<20); err == nil {
			t.Errorf("%s: expected an error", u)
		}
	}
}

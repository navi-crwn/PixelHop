// Package netsafe fetches visitor-supplied URLs without letting them reach
// the server's own network (SSRF): loopback, private ranges, link-local cloud
// metadata endpoints and the like are refused at connect time, after DNS, so
// DNS rebinding and redirects can't sneak past the check.
package netsafe

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"syscall"
	"time"
)

var ErrBlocked = errors.New("that address is not allowed")

func allowed(a netip.Addr) bool {
	a = a.Unmap()
	if !a.IsGlobalUnicast() || a.IsPrivate() || a.IsLoopback() || a.IsLinkLocalUnicast() {
		return false
	}
	for _, p := range blocked {
		if p.Contains(a) {
			return false
		}
	}
	return true
}

var blocked = []netip.Prefix{
	netip.MustParsePrefix("0.0.0.0/8"),
	netip.MustParsePrefix("100.64.0.0/10"), // carrier-grade NAT
	netip.MustParsePrefix("192.0.0.0/24"),
	netip.MustParsePrefix("198.18.0.0/15"),
	netip.MustParsePrefix("240.0.0.0/4"),
	netip.MustParsePrefix("64:ff9b::/96"), // NAT64 can map to private IPv4
	netip.MustParsePrefix("2002::/16"),    // 6to4, same
}

// Public reports whether a is an address outside every private range.
func Public(a netip.Addr) bool { return allowed(a) }

func control(_, address string, _ syscall.RawConn) error {
	ap, err := netip.ParseAddrPort(address)
	if err != nil || !allowed(ap.Addr()) {
		return ErrBlocked
	}
	return nil
}

var client = &http.Client{
	Timeout: 30 * time.Second,
	Transport: &http.Transport{
		Proxy: nil, // never route through an environment proxy that could reach inside
		DialContext: (&net.Dialer{
			Timeout: 10 * time.Second,
			Control: control,
		}).DialContext,
		TLSHandshakeTimeout:   10 * time.Second,
		ResponseHeaderTimeout: 15 * time.Second,
		MaxIdleConns:          10,
	},
	CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if len(via) >= 3 {
			return errors.New("too many redirects")
		}
		if req.URL.Scheme != "http" && req.URL.Scheme != "https" {
			return ErrBlocked
		}
		return nil
	},
}

// Fetch downloads at most limit bytes from a public http(s) URL.
func Fetch(ctx context.Context, raw string, limit int64) ([]byte, string, error) {
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return nil, "", errors.New("enter a full http:// or https:// link")
	}
	if p := u.Port(); p != "" && p != "80" && p != "443" && p != "8080" && p != "8443" {
		return nil, "", ErrBlocked
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, "", err
	}
	req.Header.Set("User-Agent", "PixelHop/3 (+https://github.com/navi-crwn/pixelhop)")
	req.Header.Set("Accept", "image/*")
	resp, err := client.Do(req)
	if err != nil {
		if errors.Is(err, ErrBlocked) {
			return nil, "", ErrBlocked
		}
		return nil, "", fmt.Errorf("couldn't download that link")
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("that link answered %s", resp.Status)
	}
	if resp.ContentLength > limit {
		return nil, "", fmt.Errorf("that image is larger than %d MB", limit>>20)
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, limit+1))
	if err != nil {
		return nil, "", fmt.Errorf("download interrupted")
	}
	if int64(len(b)) > limit {
		return nil, "", fmt.Errorf("that image is larger than %d MB", limit>>20)
	}
	name := u.Path
	return b, name, nil
}

// Post sends body to a public https URL (webhooks) through the same guard as
// Fetch, and returns the status code.
func Post(ctx context.Context, raw, contentType string, body []byte, headers map[string]string) (int, error) {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.Host == "" {
		return 0, errors.New("webhook URLs must be https")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u.String(), bytes.NewReader(body))
	if err != nil {
		return 0, err
	}
	req.Header.Set("Content-Type", contentType)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := client.Do(req)
	if err != nil {
		if errors.Is(err, ErrBlocked) {
			return 0, ErrBlocked
		}
		return 0, errors.New("couldn't reach it")
	}
	io.Copy(io.Discard, io.LimitReader(resp.Body, 64<<10))
	resp.Body.Close()
	return resp.StatusCode, nil
}

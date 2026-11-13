package app

import (
	"net/http"
	"strings"
	"testing"
)

// The studio's QR tool moved to LinkHop's QR page (product decision): the tool button is gone from
// the studio and its server endpoints are gone. The link to LinkHop and the old /qr address only
// point there when this installation is connected to the HOP family (HEL.ink); a stand-alone copy
// keeps its own pages and nothing is remembered permanently.
func TestStudioQRToolRemoved(t *testing.T) {
	h := newTestApp(t) // stand-alone
	for _, path := range []string{"/tools/qr.png?text=hi", "/tools/qr.svg?text=hi"} {
		if rec := fetch(h, path); rec.Code != http.StatusNotFound {
			t.Errorf("%s: %d, want 404", path, rec.Code)
		}
	}
	body := fetch(h, "/tools").Body.String()
	if strings.Contains(body, `data-tool="qr"`) {
		t.Error("the studio still lists a QR tool")
	}
	if strings.Contains(body, `tool-qr-link`) {
		t.Error("a stand-alone installation shows the link to LinkHop's QR page")
	}
	rec := fetch(h, "/qr")
	if rec.Code != http.StatusFound || rec.Header().Get("Location") != "/tools" {
		t.Errorf("stand-alone /qr: %d -> %q, want 302 to /tools", rec.Code, rec.Header().Get("Location"))
	}

	_, sso := ssoApp(t) // connected to HEL.ink
	if body := fetch(sso, "/tools").Body.String(); !strings.Contains(body, `href="https://link.hel.ink/qr"`) {
		t.Error("a connected installation does not point at LinkHop's QR page")
	}
	rec = fetch(sso, "/qr")
	if rec.Code != http.StatusFound || rec.Header().Get("Location") != "https://link.hel.ink/qr" {
		t.Errorf("connected /qr: %d -> %q, want 302 to the LinkHop QR page", rec.Code, rec.Header().Get("Location"))
	}
}

// The per-image QR on image pages is a different feature and stays: a real image gets a PNG code.
func TestImageQRStillServed(t *testing.T) {
	a, h := ssoApp(t)
	_, key := member(t, a, "qr@example.com")
	id := upload(t, h, key, 1)[0]
	rec := fetch(h, "/qr/"+id+".png")
	if rec.Code != http.StatusOK || rec.Header().Get("Content-Type") != "image/png" || !strings.HasPrefix(rec.Body.String(), "\x89PNG") {
		t.Fatalf("/qr/%s.png: %d %q, want a PNG code", id, rec.Code, rec.Header().Get("Content-Type"))
	}
	if rec := fetch(h, "/qr/not-an-image-id!.png"); rec.Code != http.StatusNotFound {
		t.Errorf("a malformed id answers %d, want 404", rec.Code)
	}
}

// Explore is gone: the old address goes home for good, and neither the sitemap nor the public
// navigation offers it.
func TestExploreGone(t *testing.T) {
	h := newTestApp(t)
	rec := fetch(h, "/explore")
	if rec.Code != http.StatusMovedPermanently || rec.Header().Get("Location") != "/" {
		t.Errorf("/explore: %d -> %q, want 301 to /", rec.Code, rec.Header().Get("Location"))
	}
	if sm := fetch(h, "/sitemap.xml").Body.String(); strings.Contains(sm, "/explore") {
		t.Error("the sitemap still lists /explore")
	}
	if home := fetch(h, "/").Body.String(); strings.Contains(home, `href="/explore"`) {
		t.Error("the home page still links /explore")
	}
}

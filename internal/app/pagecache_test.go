package app

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// Rule 6 of the SEO and language contract: a public HTML page follows the hop_lang cookie, so a shared cache
// (CDN, proxy) must not store it for the next visitor. It answers "private, no-cache" (or "no-store"), and
// "Vary: Cookie" when the address does not choose the language itself. Read from the response as a client
// sees it (Result), so a header set after WriteHeader, which never reaches the wire, does not count.
func TestPublicPagesNotStorableByASharedCache(t *testing.T) {
	a, h := ssoApp(t)
	u, _ := member(t, a, "cache@example.com")
	im := addImage(t, a, u, "cachepic1")
	pages := []string{"/", "/about", "/features", "/faq", "/help", "/legal", "/contact", "/status", "/explore",
		"/docs", "/tools", "/batch", "/changelog", "/login", "/register",
		"/" + im.ID,            // the public page of an image
		"/u/nobody-here",       // a profile that does not exist: the 404 page
		"/no-such-page-at-all", // the 404 page
		"/result?id=nope",      // an old link: the 404 page
	}
	variants := []struct {
		name, query string
		cookie      *http.Cookie
	}{
		{"no cookie", "", nil},
		{"hop_lang=id", "", &http.Cookie{Name: "hop_lang", Value: "id"}},
		{"?lang=id", "lang=id", nil},
	}
	for _, p := range pages {
		for _, v := range variants {
			path := p
			if v.query != "" {
				sep := "?"
				if strings.Contains(p, "?") {
					sep = "&"
				}
				path += sep + v.query
			}
			req := httptest.NewRequest("GET", path, nil)
			if v.cookie != nil {
				req.AddCookie(v.cookie)
			}
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)
			res := rec.Result()
			if !strings.HasPrefix(res.Header.Get("Content-Type"), "text/html") && res.StatusCode < 400 {
				continue // not an HTML page (a redirect or a file): other rules
			}
			cc := res.Header.Get("Cache-Control")
			if !strings.Contains(cc, "private") && !strings.Contains(cc, "no-store") {
				t.Errorf("%s [%s]: Cache-Control %q can be stored by a shared cache", p, v.name, cc)
			}
			if v.query == "" && !strings.Contains(strings.Join(res.Header.Values("Vary"), ","), "Cookie") {
				t.Errorf("%s [%s]: no Vary: Cookie (%q)", p, v.name, res.Header.Values("Vary"))
			}
		}
	}
	// What set its own policy keeps it: the status page is "no-store".
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/status", nil))
	if cc := rec.Result().Header.Get("Cache-Control"); cc != "no-store" {
		t.Errorf("/status: Cache-Control %q, want no-store", cc)
	}
	// A signed-in member's page stays private, no-store.
	rec = httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/about", nil)
	req.AddCookie(sessionFor(t, a, u.ID))
	h.ServeHTTP(rec, req)
	if cc := rec.Result().Header.Get("Cache-Control"); cc != "private, no-store" {
		t.Errorf("signed in /about: Cache-Control %q, want private, no-store", cc)
	}
	// Files, JSON and images keep their own public policy.
	for _, p := range []string{"/static/css/app.css", "/api/v1/stats"} {
		rec = httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest("GET", p, nil))
		if cc := rec.Result().Header.Get("Cache-Control"); strings.Contains(cc, "private") {
			t.Errorf("%s: Cache-Control %q; it is not a page that follows the cookie", p, cc)
		}
	}
}

package app

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

// The dashboard test of 2026-11-08 saw p.hel.ink/i/<slug>.png hand out an adult image without the gate.
// This pins down what the server does: a direct link to a file with a content warning is sent to its page
// (where the gate is) unless the gate cookie came along, whatever the referer. The report's case most
// likely saw a copy a cache kept from before the warning was set: files without a warning are public and
// cacheable for 30 days, and nothing can call such a copy back.
func TestDirectLinksToWarnedFilesGoThroughTheGate(t *testing.T) {
	a, h := linkApp(t)
	ctx := context.Background()
	im := addImage(t, a, nil, "adult-direct-AbC123")
	im.ContentWarning = "adult"
	if err := a.db.UpdateImageMeta(ctx, im); err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct{ host, referer, dest string }{
		{"p.example", "", ""},
		{"p.example", "https://p.example/" + im.ID, "image"},
		{"p.example", "https://pixel.example/dashboard", "image"},
		{"p.example", "https://forum.example/thread", ""},
		{"pixel.example", "", ""},
	} {
		for _, path := range []string{"/i/" + im.ID + "." + im.Ext, "/t/" + im.ID + ".jpg"} {
			req := httptest.NewRequest(http.MethodGet, "https://"+c.host+path, nil)
			if c.referer != "" {
				req.Header.Set("Referer", c.referer)
			}
			if c.dest != "" {
				req.Header.Set("Sec-Fetch-Dest", c.dest)
			}
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)
			if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "https://p.example/"+im.ID || rec.Header().Get("Cache-Control") != "private, no-store" {
				t.Errorf("%s%s (referer %q): %d → %q, Cache-Control %q; want 303 to the page, uncached", c.host, path, c.referer, rec.Code, rec.Header().Get("Location"), rec.Header().Get("Cache-Control"))
			}
		}
	}
	// With the gate passed the file comes, and is never cached by a shared cache.
	req := httptest.NewRequest(http.MethodGet, "https://p.example/i/"+im.ID+"."+im.Ext, nil)
	req.AddCookie(&http.Cookie{Name: gateCookieName(im.ID), Value: "1"})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code == http.StatusSeeOther || rec.Header().Get("Cache-Control") != "private, no-store" {
		t.Errorf("with the gate cookie: %d, Cache-Control %q", rec.Code, rec.Header().Get("Cache-Control"))
	}
}

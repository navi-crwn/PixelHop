package app

import (
	"net/http/httptest"
	"strings"
	"testing"
)

// publicApps counts the public apps in the bundled manifest (the page must follow it, not a fixed number),
// after checking the manifest names every app the family has today.
func publicApps(t *testing.T) int {
	t.Helper()
	n := 0
	seen := map[string]bool{}
	for _, a := range hopManifestOnce().Apps {
		seen[a.Slug] = true
		if a.Public {
			n++
		}
	}
	for _, s := range []string{"hub", "linkhop", "biohop", "pixelhop", "notehop", "filehop", "payhop"} {
		if !seen[s] {
			t.Errorf("manifest lacks %s", s)
		}
	}
	return n
}

func TestHopFamilyTypography(t *testing.T) {
	h := newTestApp(t)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/", nil))
	if rec.Code != 200 {
		t.Fatalf("home: status %d", rec.Code)
	}
	body := rec.Body.String()
	for _, want := range []string{
		`href="https://hel.ink/hop-assets/` + hopManifestOnce().Version + `/fonts-amnesia.css" integrity="sha384-`,
		`https://hel.ink/hop-assets/` + hopManifestOnce().Version + `/apps/linkhop/mark.svg`,
		`https://hel.ink/hop-assets/` + hopManifestOnce().Version + `/apps/payhop/mark.svg`,
		`>BioHop<`,
		`Part of the HOP family`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("home page lacks %q", want)
		}
	}
	_, footer, _ := strings.Cut(body, "<footer")
	if strings.Contains(footer, "apps/pixelhop/mark.svg") {
		t.Error("the footer family list should not link PixelHop to itself")
	}
	if !strings.Contains(body, `href="https://pixel.hel.ink" aria-current="true"`) || len(hopFamily("")) != publicApps(t) {
		t.Error("the switcher must list every public app in the manifest and identify PixelHop")
	}
	csp := rec.Header().Get("Content-Security-Policy")
	for _, d := range []string{"style-src 'self' 'unsafe-inline' https://hel.ink", "font-src 'self' https://hel.ink"} {
		if !strings.Contains(csp, d) {
			t.Errorf("CSP lacks %q: %s", d, csp)
		}
	}
}

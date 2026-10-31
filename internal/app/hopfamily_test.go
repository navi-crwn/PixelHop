package app

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHopFamilyTypography(t *testing.T) {
	h := newTestApp(t)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/", nil))
	if rec.Code != 200 {
		t.Fatalf("home: status %d", rec.Code)
	}
	body := rec.Body.String()
	for _, want := range []string{
		`href="https://hel.ink/hop-assets/v1/fonts-amnesia.css" integrity="sha384-`,
		`https://hel.ink/hop-assets/v1/apps/linkhop/mark.svg`,
		`https://hel.ink/hop-assets/v1/apps/notehop/mark.svg`,
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
	if !strings.Contains(body, `href="https://pixel.hel.ink" aria-current="true"`) || len(hopFamily("")) != 5 {
		t.Error("the switcher must list all five apps and identify PixelHop")
	}
	csp := rec.Header().Get("Content-Security-Policy")
	for _, d := range []string{"style-src 'self' 'unsafe-inline' https://hel.ink", "font-src 'self' https://hel.ink"} {
		if !strings.Contains(csp, d) {
			t.Errorf("CSP lacks %q: %s", d, csp)
		}
	}
}

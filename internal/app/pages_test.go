package app

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDedicatedPages(t *testing.T) {
	h := newTestApp(t)
	for path, want := range map[string]string{
		"/about":    "About PixelHop",
		"/features": "Features",
		"/faq":      "FAQ",
		"/contact":  "github.com/navi-crwn/PixelHop/issues/new/choose",
		"/status":   "is up",
		"/help":     "Help",
		"/legal":    "Privacy",
	} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest("GET", path, nil))
		if rec.Code != 200 {
			t.Errorf("%s: status %d", path, rec.Code)
			continue
		}
		body := rec.Body.String()
		if !strings.Contains(body, want) {
			t.Errorf("%s: lacks %q", path, want)
		}
		if !strings.Contains(body, `class="back" href="/"`) {
			t.Errorf("%s: no back link to the home page", path)
		}
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/", nil))
	if strings.Contains(rec.Body.String(), `class="back"`) {
		t.Error("the home page should not show a back link")
	}
	for from, to := range map[string]string{"/contact.php": "/contact", "/features.php": "/features", "/terms": "/legal", "/privacy": "/legal#privacy"} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest("GET", from, nil))
		if loc := rec.Header().Get("Location"); rec.Code != 301 || loc != to {
			t.Errorf("%s: %d %q, want 301 %q", from, rec.Code, loc, to)
		}
	}
}

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

func TestPageLanguages(t *testing.T) {
	h := newTestApp(t)

	// Default is English, Accept-Language is ignored
	req := httptest.NewRequest("GET", "/about", nil)
	req.Header.Set("Accept-Language", "id-ID,id;q=0.9")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("/about: status %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, `<html lang="en" dir="ltr">`) {
		t.Errorf("default html lang tag mismatch: %s", body[:100])
	}
	if !strings.Contains(body, "About PixelHop") {
		t.Errorf("default should be English, got body: %s", body[:200])
	}

	// ?lang=id switches to Indonesian and sets cookie
	req = httptest.NewRequest("GET", "/about?lang=id", nil)
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("/about?lang=id: status %d", rec.Code)
	}
	body = rec.Body.String()
	if !strings.Contains(body, `<html lang="id" dir="ltr">`) {
		t.Errorf("expected html lang=id dir=ltr")
	}
	if !strings.Contains(body, "Tentang PixelHop") {
		t.Errorf("expected Indonesian title, got body: %s", body[:200])
	}
	cookies := rec.Result().Cookies()
	var foundCookie bool
	for _, c := range cookies {
		if c.Name == "pixel_lang" && c.Value == "id" {
			foundCookie = true
			break
		}
	}
	if !foundCookie {
		t.Errorf("pixel_lang=id cookie was not set")
	}

	// ?lang=ar sets RTL
	req = httptest.NewRequest("GET", "/about?lang=ar", nil)
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("/about?lang=ar: status %d", rec.Code)
	}
	body = rec.Body.String()
	if !strings.Contains(body, `<html lang="ar" dir="rtl">`) {
		t.Errorf("expected html lang=ar dir=rtl, got: %s", body[:100])
	}

	// GET /i18n/{lang} returns JavaScript catalog
	req = httptest.NewRequest("GET", "/i18n/id", nil)
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("/i18n/id: status %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "window.PIXEL_I18N =") {
		t.Errorf("expected window.PIXEL_I18N in catalog JS")
	}
}

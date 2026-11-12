package app

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// The family language cookie (train 4 contract): hop_lang, plain text, a year
// long, readable by scripts; Domain=hel.ink on hel.ink hosts and host-only
// anywhere else. Order: ?lang= → hop_lang → PixelHop's old pixel_lang → English.
func TestFamilyLanguageCookie(t *testing.T) {
	_, h := ssoApp(t)
	get := func(target string, cookies ...*http.Cookie) *httptest.ResponseRecorder {
		req := httptest.NewRequest("GET", target, nil)
		for _, c := range cookies {
			req.AddCookie(c)
		}
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec
	}
	setCookie := func(rec *httptest.ResponseRecorder, name string) string {
		for _, line := range rec.Result().Header.Values("Set-Cookie") {
			if strings.HasPrefix(line, name+"=") {
				return line
			}
		}
		return ""
	}

	// On a hel.ink host: one cookie for the whole family.
	rec := get("https://pixel.hel.ink/about?lang=de")
	sc := setCookie(rec, "hop_lang")
	for _, want := range []string{"hop_lang=de", "Path=/", "Domain=hel.ink", "Max-Age=31536000", "Secure", "SameSite=Lax"} {
		if !strings.Contains(sc, want) {
			t.Errorf("hel.ink: Set-Cookie %q lacks %q", sc, want)
		}
	}
	if strings.Contains(sc, "HttpOnly") {
		t.Errorf("hop_lang must be readable by scripts: %q", sc)
	}
	if setCookie(rec, "pixel_lang") != "" {
		t.Error("the old pixel_lang cookie is no longer written")
	}
	// Self-hosted or localhost: host-only, so the cookie still works there.
	sc = setCookie(get("http://localhost:8104/about?lang=pt"), "hop_lang")
	if !strings.HasPrefix(sc, "hop_lang=pt-BR;") || strings.Contains(sc, "Domain=") {
		t.Errorf("localhost: Set-Cookie %q, want hop_lang=pt-BR host-only", sc)
	}

	// hop_lang is read, and wins over the old cookie.
	rec = get("http://localhost:8104/about", &http.Cookie{Name: "hop_lang", Value: "ja"}, &http.Cookie{Name: "pixel_lang", Value: "id"})
	if !strings.Contains(rec.Body.String(), `<html lang="ja"`) {
		t.Error("hop_lang=ja did not choose Japanese over pixel_lang=id")
	}
	if setCookie(rec, "hop_lang") != "" {
		t.Error("nothing new chosen: no cookie should be written")
	}
	// Only the old cookie: its language is used and copied into hop_lang once.
	rec = get("https://pixel.hel.ink/about", &http.Cookie{Name: "pixel_lang", Value: "id"})
	if !strings.Contains(rec.Body.String(), `<html lang="id"`) {
		t.Error("pixel_lang=id alone did not choose Indonesian")
	}
	if sc := setCookie(rec, "hop_lang"); !strings.HasPrefix(sc, "hop_lang=id;") || !strings.Contains(sc, "Domain=hel.ink") {
		t.Errorf("pixel_lang was not carried into hop_lang: %q", sc)
	}
	// The URL still beats both cookies.
	if body := get("http://localhost:8104/about?lang=fr", &http.Cookie{Name: "hop_lang", Value: "ja"}).Body.String(); !strings.Contains(body, `<html lang="fr"`) {
		t.Error("?lang=fr did not win over hop_lang")
	}
	// Server messages (tr) read the same cookie.
	req := httptest.NewRequest("POST", "http://localhost:8104/api/v1/upload", nil)
	req.AddCookie(&http.Cookie{Name: "hop_lang", Value: "id"})
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if !strings.Contains(rec.Body.String(), "Pilih file") {
		t.Errorf("API message with hop_lang=id: %s", rec.Body.String())
	}
}

func TestFamilyCookieDomain(t *testing.T) {
	for host, want := range map[string]string{
		"hel.ink": "hel.ink", "pixel.hel.ink": "hel.ink", "p.hel.ink:443": "hel.ink", "PIXEL.HEL.INK.": "hel.ink",
		"localhost:8104": "", "img.example.com": "", "notahel.ink": "", "hel.ink.example.com": "",
	} {
		r := httptest.NewRequest("GET", "/", nil)
		r.Host = host
		if got := familyCookieDomain(r); got != want {
			t.Errorf("%s: %q, want %q", host, got, want)
		}
	}
}

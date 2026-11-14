package app

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"io"
	"log/slog"
	"maps"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/navi-crwn/pixelhop/internal/config"
	"github.com/navi-crwn/pixelhop/internal/db"
)

// fakeHelink answers /api/pixelhop/* like HEL.ink does, after checking the signature.
func fakeHelink(t *testing.T, accounts map[string]map[string]any) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		m := hmac.New(sha256.New, []byte(testSecret))
		m.Write([]byte("pixelhop-api|" + r.Header.Get("X-PixelHop-Timestamp") + "|"))
		m.Write(body)
		if r.Header.Get("X-PixelHop-Signature") != base64.RawURLEncoding.EncodeToString(m.Sum(nil)) {
			w.WriteHeader(401)
			w.Write([]byte(`{"error":"bad_signature"}`))
			return
		}
		var in map[string]any
		json.Unmarshal(body, &in)
		email := in["email"].(string)
		switch r.URL.Path {
		case "/api/pixelhop/login":
			acc, ok := accounts[email]
			if !ok || acc["password"] != in["password"] {
				w.WriteHeader(401)
				w.Write([]byte(`{"error":"bad_credentials"}`))
				return
			}
			if acc["verified"] != true {
				w.WriteHeader(403)
				w.Write([]byte(`{"error":"unverified"}`))
				return
			}
			if acc["2fa"] == true {
				w.WriteHeader(409)
				w.Write([]byte(`{"error":"two_factor"}`))
				return
			}
			json.NewEncoder(w).Encode(map[string]any{"user": map[string]any{"sub": acc["sub"], "email": email, "name": "CS Team", "verified": acc["verified"]}})
		case "/api/pixelhop/register":
			if _, ok := accounts[email]; ok {
				w.WriteHeader(422)
				w.Write([]byte(`{"error":"invalid","message":"An account with this email already exists. Sign in instead."}`))
				return
			}
			accounts[email] = map[string]any{"sub": "500", "password": in["password"], "verified": false}
			w.WriteHeader(201)
			json.NewEncoder(w).Encode(map[string]any{"user": map[string]any{"sub": "500", "email": email, "name": in["name"], "verified": false}})
		}
	}))
}

func apiSSOApp(t *testing.T, helinkURL string) (*App, http.Handler) {
	t.Helper()
	t.Setenv("PIXELHOP_DATA", t.TempDir())
	t.Setenv("PIXELHOP_BASE_URL", "https://p.example.com")
	t.Setenv("HELINK_URL", helinkURL)
	t.Setenv("HELINK_SSO_SECRET", testSecret)
	fakeTurnstile(t)
	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	d, err := db.Open(cfg.DataDir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { d.Close() })
	a, err := New(cfg, d, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	return a, a.Handler()
}

// fakeTurnstile switches Turnstile on and answers siteverify: tokens other than "pass" fail.
func fakeTurnstile(t *testing.T) {
	t.Setenv("TURNSTILE_SITE_KEY", "site")
	t.Setenv("TURNSTILE_SECRET_KEY", "secret")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.ParseForm()
		json.NewEncoder(w).Encode(map[string]bool{"success": r.FormValue("response") == "pass"})
	}))
	old := turnstileVerifyURL
	turnstileVerifyURL = srv.URL
	t.Cleanup(func() { turnstileVerifyURL = old; srv.Close() })
}

// postForm posts a form that passed the Turnstile check, unless it names its own token.
func postForm(h http.Handler, path string, v url.Values) *httptest.ResponseRecorder {
	if !v.Has("cf-turnstile-response") {
		v = url.Values(maps.Clone(v))
		v.Set("cf-turnstile-response", "pass")
	}
	req := httptest.NewRequest("POST", path, strings.NewReader(v.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func hasSession(rec *httptest.ResponseRecorder) bool {
	for _, c := range rec.Result().Cookies() {
		if c.Name == sessionCookie && c.Value != "" {
			return true
		}
	}
	return false
}

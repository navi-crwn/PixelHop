package app

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
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

func TestPasswordLoginWithHelinkAccount(t *testing.T) {
	accounts := map[string]map[string]any{
		"tim@example.com": {"sub": "99", "password": "random123", "verified": true},
		"tf@example.com":  {"sub": "98", "password": "random123", "verified": true, "2fa": true},
		"new@example.com": {"sub": "97", "password": "random123", "verified": false},
	}
	srv := fakeHelink(t, accounts)
	defer srv.Close()
	a, h := apiSSOApp(t, srv.URL)

	rec := postForm(h, "/login", url.Values{"email": {"tim@example.com"}, "password": {"random123"}})
	if rec.Code != http.StatusSeeOther || !hasSession(rec) {
		t.Fatalf("login with HEL.ink password: %d %s", rec.Code, rec.Body.String())
	}
	u, err := a.db.UserByEmail(t.Context(), "tim@example.com")
	if err != nil || u.HelinkID.String != "99" {
		t.Fatalf("profile not linked: %+v %v", u, err)
	}
	if rec := postForm(h, "/login", url.Values{"email": {"tim@example.com"}, "password": {"nope"}}); rec.Code != http.StatusUnauthorized || hasSession(rec) {
		t.Fatalf("wrong password: %d", rec.Code)
	}
	if rec := postForm(h, "/login", url.Values{"email": {"tf@example.com"}, "password": {"random123"}}); rec.Code != http.StatusSeeOther || !strings.HasPrefix(rec.Header().Get("Location"), "/auth/helink") {
		t.Fatalf("two-factor accounts go through HEL.ink: %d %s", rec.Code, rec.Header().Get("Location"))
	}
	// An unconfirmed HEL.ink address is not signed in until it is confirmed.
	if rec := postForm(h, "/login", url.Values{"email": {"new@example.com"}, "password": {"random123"}}); hasSession(rec) || !strings.Contains(rec.Body.String(), "Confirm your email") {
		t.Fatalf("unconfirmed sign-in: %d", rec.Code)
	}
	if _, err := a.db.UserByEmail(t.Context(), "new@example.com"); err == nil {
		t.Fatal("no profile before the email is confirmed")
	}
	// The security check is required.
	if rec := postForm(h, "/login", url.Values{"email": {"tim@example.com"}, "password": {"random123"}, "cf-turnstile-response": {"nope"}}); hasSession(rec) {
		t.Fatal("login without passing Turnstile")
	}
}

func TestUnverifiedAccountNeverSignsIn(t *testing.T) {
	// Even if HEL.ink answered with an unconfirmed account, PixelHop doesn't use it.
	a, _ := apiSSOApp(t, "https://hel.example.com")
	if _, err := a.accountFromHelink(t.Context(), &helinkAccount{Sub: "7", Email: "u@example.com", Verified: false}); !errors.Is(err, errNeedsVerifiedEmail) {
		t.Fatalf("unverified account accepted: %v", err)
	}
}

func TestSSOWithoutTurnstileKeysFailsClosed(t *testing.T) {
	a, _ := apiSSOApp(t, "https://hel.example.com")
	a.cfg.TurnstileSiteKey, a.cfg.TurnstileSecret = "", ""
	if a.turnstileOK(httptest.NewRequest("POST", "/login", nil)) {
		t.Fatal("password forms must not run unprotected while HEL.ink accounts are on")
	}
}

func TestRegisterCreatesHelinkAccount(t *testing.T) {
	accounts := map[string]map[string]any{}
	srv := fakeHelink(t, accounts)
	defer srv.Close()
	a, h := apiSSOApp(t, srv.URL)
	if rec := fetch(h, "/register"); rec.Code != 200 || !strings.Contains(rec.Body.String(), "One account, two apps") {
		t.Fatalf("register page: %d", rec.Code)
	}
	v := url.Values{"name": {"CS"}, "email": {"cs@example.com"}, "password": {"a-strong-pass"}}
	if rec := postForm(h, "/register", v); hasSession(rec) {
		t.Fatal("terms must be accepted")
	}
	v.Set("tos", "1")
	rec := postForm(h, "/register", v)
	if hasSession(rec) || !strings.Contains(rec.Body.String(), "confirm your email") {
		t.Fatalf("sign-up must wait for the email to be confirmed: %d %s", rec.Code, rec.Body.String())
	}
	if accounts["cs@example.com"] == nil {
		t.Fatal("the account must be made on HEL.ink")
	}
	if rec := postForm(h, "/login", url.Values{"email": {"cs@example.com"}, "password": {"a-strong-pass"}}); hasSession(rec) {
		t.Fatal("login before confirming the email")
	}
	// Once the email is confirmed on HEL.ink, signing in works and links the profile.
	accounts["cs@example.com"]["verified"] = true
	if rec := postForm(h, "/login", url.Values{"email": {"cs@example.com"}, "password": {"a-strong-pass"}}); !hasSession(rec) {
		t.Fatalf("login after confirming: %d", rec.Code)
	}
	if u, err := a.db.UserByEmail(t.Context(), "cs@example.com"); err != nil || u.HelinkID.String != "500" {
		t.Fatalf("profile: %+v %v", u, err)
	}
}

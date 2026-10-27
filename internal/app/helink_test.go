package app

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/navi-crwn/pixelhop/internal/config"
	"github.com/navi-crwn/pixelhop/internal/db"
)

const testSecret = "a-shared-secret-that-is-long-enough-123456"

// signTicket builds a ticket exactly like HEL.ink's PixelHopSsoController.
func signTicket(t helinkTicket, secret string) string {
	js, _ := json.Marshal(t)
	body := base64.RawURLEncoding.EncodeToString(js)
	m := hmac.New(sha256.New, []byte(secret))
	m.Write([]byte(body))
	return body + "." + base64.RawURLEncoding.EncodeToString(m.Sum(nil))
}

func ssoApp(t *testing.T) (*App, http.Handler) {
	t.Helper()
	t.Setenv("PIXELHOP_DATA", t.TempDir())
	t.Setenv("PIXELHOP_BASE_URL", "https://p.example.com")
	t.Setenv("HELINK_URL", "https://hel.example.com")
	t.Setenv("HELINK_SSO_SECRET", testSecret)
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

// start runs /auth/helink and returns the state cookie and the state sent to HEL.ink.
func start(t *testing.T, h http.Handler) (*http.Cookie, string) {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/auth/helink?next=/dashboard", nil))
	if rec.Code != http.StatusFound {
		t.Fatalf("start: %d", rec.Code)
	}
	loc, _ := url.Parse(rec.Header().Get("Location"))
	if loc.Host != "hel.example.com" || loc.Path != "/sso/pixelhop" || loc.Query().Get("return_to") != "https://p.example.com/auth/helink/callback" {
		t.Fatalf("unexpected redirect %s", loc)
	}
	return rec.Result().Cookies()[0], loc.Query().Get("state")
}

func callback(h http.Handler, c *http.Cookie, ticket string) *httptest.ResponseRecorder {
	req := httptest.NewRequest("GET", "/auth/helink/callback?ticket="+url.QueryEscape(ticket), nil)
	if c != nil {
		req.AddCookie(c)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func ticketFor(state string) helinkTicket {
	now := time.Now().Unix()
	return helinkTicket{Sub: "42", Email: "ivan@example.com", Name: "Ivan", State: state, Iat: now, Exp: now + 120, Jti: randomToken(12)}
}

func TestHelinkSignIn(t *testing.T) {
	a, h := ssoApp(t)
	ctx := context.Background()
	// An older PixelHop account with the same email gets linked, not duplicated.
	old := &db.User{Email: "ivan@example.com", QuotaBytes: 1 << 20}
	if err := a.db.InsertUser(ctx, old); err != nil {
		t.Fatal(err)
	}

	c, state := start(t, h)
	tk := signTicket(ticketFor(state), testSecret)
	rec := callback(h, c, tk)
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/dashboard" {
		t.Fatalf("callback: %d %s %s", rec.Code, rec.Header().Get("Location"), rec.Body.String())
	}
	if !strings.Contains(rec.Header().Get("Set-Cookie"), sessionCookie) {
		t.Fatal("no session started")
	}
	u, err := a.db.UserByHelink(ctx, "42")
	if err != nil || u.ID != old.ID || u.Name != "Ivan" {
		t.Fatalf("not linked to the existing account: %+v %v", u, err)
	}

	// The same ticket can't be used twice.
	if rec := callback(h, c, tk); rec.Code != http.StatusBadRequest {
		t.Fatalf("replay accepted: %d", rec.Code)
	}
}

func TestHelinkRejectsBadTickets(t *testing.T) {
	_, h := ssoApp(t)
	c, state := start(t, h)

	expired := ticketFor(state)
	expired.Iat, expired.Exp = time.Now().Add(-10*time.Minute).Unix(), time.Now().Add(-8*time.Minute).Unix()
	wrongState := ticketFor("someone-elses-state-value")
	otherApp := ticketFor(state)
	otherApp.Aud = "notehop"
	good := signTicket(ticketFor(state), testSecret)
	tampered := strings.Replace(good, good[5:9], "AAAA", 1)

	for name, tk := range map[string]string{
		"wrong secret": signTicket(ticketFor(state), "another-secret-that-is-long-enough-000000"),
		"expired":      signTicket(expired, testSecret),
		"wrong state":  signTicket(wrongState, testSecret),
		"other app":    signTicket(otherApp, testSecret),
		"tampered":     tampered,
		"garbage":      "not-a-ticket",
	} {
		if rec := callback(h, c, tk); rec.Code != http.StatusBadRequest {
			t.Errorf("%s: accepted (%d)", name, rec.Code)
		}
	}
	// No state cookie (e.g. ticket opened in another browser).
	if rec := callback(h, nil, good); rec.Code != http.StatusBadRequest {
		t.Errorf("missing cookie: accepted (%d)", rec.Code)
	}
}

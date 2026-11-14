package app

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/navi-crwn/pixelhop/internal/config"
	"github.com/navi-crwn/pixelhop/internal/db"
)

// formPost posts a plain form (no Turnstile) carrying the given cookies.
func formPost(h http.Handler, path string, v url.Values, cookies ...*http.Cookie) *httptest.ResponseRecorder {
	req := httptest.NewRequest("POST", path, strings.NewReader(v.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	for _, c := range cookies {
		req.AddCookie(c)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func sessionOf(rec *httptest.ResponseRecorder) *http.Cookie {
	for _, c := range rec.Result().Cookies() {
		if c.Name == sessionCookie && c.Value != "" {
			return c
		}
	}
	return nil
}

// Without a hub (the self-hosted install) people sign up and sign in with a password, the first
// account runs the server, and signing out ends the session. This guards that path while the
// hub-connected sign-in changes around it.
func TestStandaloneSignUpAndPasswordSignIn(t *testing.T) {
	a, h := standaloneApp(t)
	if a.cfg.SSOEnabled() {
		t.Fatal("test app must run without a hub")
	}

	// First sign-up: becomes admin, lands on the dashboard, holds a session.
	rec := formPost(h, "/register", url.Values{"email": {"first@example.com"}, "name": {"First"}, "password": {"correct horse 1"}})
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/dashboard?welcome=1" {
		t.Fatalf("first sign-up: %d %s", rec.Code, rec.Header().Get("Location"))
	}
	first := sessionOf(rec)
	if first == nil {
		t.Fatal("first sign-up started no session")
	}
	u, err := a.db.UserByEmail(t.Context(), "first@example.com")
	if err != nil || u.Role != "admin" {
		t.Fatalf("first account: %+v %v", u, err)
	}
	if u.HelinkID.Valid {
		t.Fatal("a stand-alone account must not carry a HOP ID")
	}

	// Second sign-up: an ordinary member.
	rec = formPost(h, "/register", url.Values{"email": {"second@example.com"}, "password": {"correct horse 2"}})
	if rec.Code != http.StatusSeeOther || sessionOf(rec) == nil {
		t.Fatalf("second sign-up: %d", rec.Code)
	}
	if u2, _ := a.db.UserByEmail(t.Context(), "second@example.com"); u2 == nil || u2.Role == "admin" {
		t.Fatalf("second account must not be admin: %+v", u2)
	}
	// The same e-mail cannot sign up twice.
	if rec := formPost(h, "/register", url.Values{"email": {"first@example.com"}, "password": {"another pass 3"}}); rec.Code != http.StatusBadRequest || sessionOf(rec) != nil {
		t.Fatalf("duplicate sign-up: %d", rec.Code)
	}

	// Sign out ends the session: the dashboard sends the browser to /login again.
	req := httptest.NewRequest("GET", "/dashboard", nil)
	req.AddCookie(first)
	r1 := httptest.NewRecorder()
	h.ServeHTTP(r1, req)
	if r1.Code != 200 {
		t.Fatalf("dashboard closed to a signed-in member: %d", r1.Code)
	}
	rec = formPost(h, "/logout", url.Values{}, first)
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/" {
		t.Fatalf("logout: %d %s", rec.Code, rec.Header().Get("Location"))
	}
	req = httptest.NewRequest("GET", "/dashboard", nil)
	req.AddCookie(first)
	r2 := httptest.NewRecorder()
	h.ServeHTTP(r2, req)
	if r2.Code != http.StatusSeeOther || !strings.HasPrefix(r2.Header().Get("Location"), "/login") {
		t.Fatalf("session survived logout: %d %s", r2.Code, r2.Header().Get("Location"))
	}

	// Sign in with the password; a wrong password and an unknown e-mail are refused alike.
	rec = formPost(h, "/login", url.Values{"email": {"first@example.com"}, "password": {"correct horse 1"}, "next": {"/albums"}})
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/albums" || sessionOf(rec) == nil {
		t.Fatalf("password sign-in: %d %s", rec.Code, rec.Header().Get("Location"))
	}
	bad := formPost(h, "/login", url.Values{"email": {"first@example.com"}, "password": {"wrong password"}})
	unknown := formPost(h, "/login", url.Values{"email": {"nobody@example.com"}, "password": {"wrong password"}})
	for _, r := range []*httptest.ResponseRecorder{bad, unknown} {
		if r.Code != http.StatusUnauthorized || sessionOf(r) != nil {
			t.Fatalf("bad sign-in: %d", r.Code)
		}
	}
}

// standaloneApp is newTestApp with the App returned too: no hub, so sign-in is by password.
func standaloneApp(t *testing.T) (*App, http.Handler) {
	t.Helper()
	t.Setenv("PIXELHOP_DATA", t.TempDir())
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

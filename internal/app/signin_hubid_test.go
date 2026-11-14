package app

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strings"
	"testing"

	"github.com/navi-crwn/pixelhop/internal/db"
)

// hubMember is an account linked to HOP ID that still holds the password it had before linking.
func hubMember(t *testing.T, a *App, email, sub, pw, role string) *db.User {
	t.Helper()
	u := &db.User{Email: email, Name: "Helena", Role: role, QuotaBytes: 1 << 20, PassHash: HashPassword(pw),
		HelinkID: sql.NullString{String: sub, Valid: true}}
	if err := a.db.InsertUser(t.Context(), u); err != nil {
		t.Fatal(err)
	}
	return u
}

func plainMember(t *testing.T, a *App, email, pw string) *db.User {
	t.Helper()
	u := &db.User{Email: email, Name: "Old Timer", Role: "user", QuotaBytes: 1 << 20, PassHash: HashPassword(pw)}
	if err := a.db.InsertUser(t.Context(), u); err != nil {
		t.Fatal(err)
	}
	return u
}

// With a hub the only way in is HOP ID: the login and register screens hold one card, the old forms are
// refused by the server (nothing is created, nobody signs in), and direct Google is off.
func TestHubModeAcceptsOnlyHopID(t *testing.T) {
	t.Setenv("GOOGLE_CLIENT_ID", "gid")
	t.Setenv("GOOGLE_CLIENT_SECRET", "gsecret")
	hub := fakeHelink(t, map[string]map[string]any{})
	defer hub.Close()
	a, h := apiSSOApp(t, hub.URL)
	hubMember(t, a, "helena@example.com", "501", "old-password-1", "user")
	plainMember(t, a, "legacy@example.com", "legacy-pass-1")
	users := a.db.CountUsers(t.Context())

	for _, path := range []string{"/login?next=/albums", "/register?next=/albums"} {
		rec := fetch(h, path)
		body := rec.Body.String()
		if rec.Code != 200 || !regexp.MustCompile(`href="/auth/helink\?next=(/|%2[Ff])albums"`).MatchString(body) || !strings.Contains(body, "Continue with HOP ID") {
			t.Errorf("%s: %d, no HOP ID card carrying next", path, rec.Code)
		}
		for _, gone := range []string{`name="password"`, `name="email"`, `name="tos"`, `/auth/google`, `action="/login"`, `action="/register"`, "cf-turnstile"} {
			if strings.Contains(body, gone) {
				t.Errorf("%s still offers %s", path, gone)
			}
		}
	}

	// The old forms answer with a pointer back to the card and change nothing.
	for _, c := range []struct{ path, email, pw string }{
		{"/login", "helena@example.com", "old-password-1"}, {"/login", "legacy@example.com", "legacy-pass-1"},
		{"/register", "brand-new@example.com", "a-fresh-password-1"}} {
		rec := postForm(h, c.path, url.Values{"email": {c.email}, "password": {c.pw}, "name": {"X"}, "tos": {"1"}, "next": {"/albums"}})
		if rec.Code != http.StatusSeeOther || !strings.HasPrefix(rec.Header().Get("Location"), c.path) || hasSession(rec) {
			t.Errorf("POST %s %s: %d %q session=%v", c.path, c.email, rec.Code, rec.Header().Get("Location"), hasSession(rec))
		}
	}
	if a.db.CountUsers(t.Context()) != users {
		t.Error("an old form created an account")
	}
	if _, err := a.db.UserByEmail(t.Context(), "brand-new@example.com"); err == nil {
		t.Error("sign-up through the old form worked")
	}

	// Direct Google is gone, even with Google configured and a person who would have matched.
	if rec := fetch(h, "/auth/google"); rec.Code != http.StatusNotFound {
		t.Errorf("/auth/google: %d, want 404", rec.Code)
	}
	fakeGoogle(t, "google-1", "legacy@example.com", true)
	if rec := googleReturn(h); hasSession(rec) || rec.Code < 400 {
		t.Errorf("Google callback signed someone in: %d", rec.Code)
	}
	if u, _ := a.db.UserByEmail(t.Context(), "legacy@example.com"); u.GoogleID.Valid {
		t.Error("a refused Google callback wrote google_id")
	}

	// HOP ID still takes next= through.
	rec := fetch(h, "/auth/helink?next=/albums")
	if rec.Code != http.StatusFound || !strings.Contains(rec.Result().Header.Get("Set-Cookie"), "%2Falbums") && !strings.Contains(rec.Result().Header.Get("Set-Cookie"), "/albums") {
		t.Errorf("next= lost on the way to HOP ID: %d %q", rec.Code, rec.Result().Header.Get("Set-Cookie"))
	}
}

// fakeGoogle answers Google's token and userinfo endpoints for one person.
func fakeGoogle(t *testing.T, sub, email string, verified bool) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/token") {
			json.NewEncoder(w).Encode(map[string]string{"access_token": "tok"})
			return
		}
		json.NewEncoder(w).Encode(map[string]any{"sub": sub, "email": email, "email_verified": verified, "name": "G Person"})
	}))
	oldT, oldU := googleTokenURL, googleUserinfoURL
	googleTokenURL, googleUserinfoURL = srv.URL+"/token", srv.URL+"/userinfo"
	t.Cleanup(func() { googleTokenURL, googleUserinfoURL = oldT, oldU; srv.Close() })
}

func googleReturn(h http.Handler) *httptest.ResponseRecorder {
	req := httptest.NewRequest("GET", "/auth/google/callback?state=st&code=c", nil)
	req.AddCookie(&http.Cookie{Name: googleState, Value: "st|/dashboard"})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

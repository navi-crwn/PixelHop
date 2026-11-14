package app

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
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

const hubIDOnlyMsg = "signs in with HOP ID"

// With a hub, a member who has a HOP ID signs in there (2FA, account status). The old local password
// must not open a session; accounts without a HOP ID sign in as before.
func TestHubMemberCannotSignInWithLocalPassword(t *testing.T) {
	hub := fakeHelink(t, map[string]map[string]any{})
	defer hub.Close()
	a, h := apiSSOApp(t, hub.URL)
	hubMember(t, a, "helena@example.com", "501", "old-password-1", "user")
	hubMember(t, a, "boss@example.com", "502", "old-password-2", "admin")
	plainMember(t, a, "legacy@example.com", "legacy-pass-1")

	for _, email := range []string{"helena@example.com", "boss@example.com"} {
		pw := map[string]string{"helena@example.com": "old-password-1", "boss@example.com": "old-password-2"}[email]
		rec := postForm(h, "/login", url.Values{"email": {email}, "password": {pw}, "next": {"/dashboard"}})
		if rec.Code != http.StatusForbidden || hasSession(rec) || !strings.Contains(rec.Body.String(), hubIDOnlyMsg) {
			t.Fatalf("%s: local password opened or mis-answered: %d session=%v", email, rec.Code, hasSession(rec))
		}
	}

	// A wrong password gives the same answer as an unknown e-mail: nothing new is revealed.
	wrong := postForm(h, "/login", url.Values{"email": {"helena@example.com"}, "password": {"not-it-at-all"}})
	unknown := postForm(h, "/login", url.Values{"email": {"nobody@example.com"}, "password": {"not-it-at-all"}})
	if wrong.Code != http.StatusUnauthorized || unknown.Code != http.StatusUnauthorized || hasSession(wrong) || hasSession(unknown) {
		t.Fatalf("wrong/unknown: %d %d", wrong.Code, unknown.Code)
	}
	if strings.Contains(wrong.Body.String(), hubIDOnlyMsg) || strings.Contains(unknown.Body.String(), hubIDOnlyMsg) {
		t.Fatal("a wrong password was told the account uses HOP ID")
	}

	// A member without a HOP ID signs in with the password exactly as before.
	rec := postForm(h, "/login", url.Values{"email": {"legacy@example.com"}, "password": {"legacy-pass-1"}, "next": {"/albums"}})
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/albums" || !hasSession(rec) {
		t.Fatalf("legacy member: %d %s", rec.Code, rec.Header().Get("Location"))
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

// Google sign-in straight into PixelHop is refused for a member with a HOP ID too, whether the Google
// account was linked before or is only matched by e-mail now; the match must not be written either.
func TestHubMemberCannotSignInWithGoogle(t *testing.T) {
	t.Setenv("GOOGLE_CLIENT_ID", "gid")
	t.Setenv("GOOGLE_CLIENT_SECRET", "gsecret")
	hub := fakeHelink(t, map[string]map[string]any{})
	defer hub.Close()
	a, h := apiSSOApp(t, hub.URL)
	ctx := t.Context()

	linked := hubMember(t, a, "g1@example.com", "601", "x-password-1", "user")
	linked.GoogleID = sql.NullString{String: "google-1", Valid: true}
	a.db.UpdateUser(ctx, linked)
	byMail := hubMember(t, a, "g2@example.com", "602", "x-password-2", "user")
	plain := plainMember(t, a, "g3@example.com", "x-password-3")

	fakeGoogle(t, "google-1", "g1@example.com", true)
	if rec := googleReturn(h); rec.Code != http.StatusForbidden || hasSession(rec) || !strings.Contains(rec.Body.String(), hubIDOnlyMsg) {
		t.Fatalf("linked Google account of a HOP ID member: %d", rec.Code)
	}

	fakeGoogle(t, "google-2", "g2@example.com", true)
	if rec := googleReturn(h); rec.Code != http.StatusForbidden || hasSession(rec) {
		t.Fatalf("Google matched by e-mail to a HOP ID member: %d", rec.Code)
	}
	if u, _ := a.db.UserByEmail(ctx, "g2@example.com"); u.GoogleID.Valid || u.ID != byMail.ID {
		t.Fatalf("a refused Google sign-in still wrote google_id: %+v", u.GoogleID)
	}

	// Without a HOP ID: unchanged (matched by verified e-mail, signed in).
	fakeGoogle(t, "google-3", "g3@example.com", true)
	rec := googleReturn(h)
	if rec.Code != http.StatusSeeOther || !hasSession(rec) {
		t.Fatalf("Google sign-in of a member without HOP ID: %d", rec.Code)
	}
	if u, _ := a.db.UserByEmail(ctx, "g3@example.com"); u.ID != plain.ID || u.GoogleID.String != "google-3" {
		t.Fatalf("not linked as before: %+v", u)
	}
}

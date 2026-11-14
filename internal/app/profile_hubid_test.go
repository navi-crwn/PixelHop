package app

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"
)

// A member with a HOP ID sees the profile read-only with one way to change it, at the hub; the server
// refuses every form that would change it here, and the photo uploaded here before is not shown.
func TestHubMemberProfileIsReadOnly(t *testing.T) {
	a, h := ssoApp(t)
	ctx := t.Context()
	u := linked(t, a, "91", "helena@example.com")
	u.Name, u.Bio, u.Website = "Helena", "hub bio", "https://helena.example"
	u.AvatarKey, u.AvatarURL = "avatars/old-upload.jpg", "https://hel.example.com/storage/a.png"
	u.ShowOwner = true
	a.db.UpdateUser(ctx, u)
	c := sessionFor(t, a, u.ID)

	for _, p := range []string{"/settings/profile", "/settings/avatar", "/settings/avatar/delete"} {
		rec := post(h, p, url.Values{"name": {"Mallory"}, "username": {"mallory"}, "bio": {"changed"}, "show_owner": {"on"}}, c)
		if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "managed in HOP ID") {
			t.Errorf("POST %s: %d, want a 400 saying the profile is managed in HOP ID", p, rec.Code)
		}
	}
	got, _ := a.db.UserByID(ctx, u.ID)
	if got.Name != "Helena" || got.Bio != "hub bio" || got.Username.String != "user91" || got.AvatarKey != "avatars/old-upload.jpg" {
		t.Fatalf("a refused form still changed the profile: %+v", got)
	}

	// The Profile tab: read-only fields and a single link to the hub's account settings.
	body := fetch(h, "/settings", c).Body.String()
	if !strings.Contains(body, `href="https://hel.example.com/settings#account"`) {
		t.Error("no link to the hub's account settings")
	}
	for _, gone := range []string{`action="/settings/profile"`, `action="/settings/avatar"`, `action="/settings/avatar/delete"`} {
		if strings.Contains(body, gone) {
			t.Errorf("the page still offers %s", gone)
		}
	}
	if !strings.Contains(body, `id="bio" rows="3" readonly>hub bio`) || !strings.Contains(body, `id="name" value="Helena" readonly`) {
		t.Error("the profile is not shown read-only")
	}

	// The hub's picture wins over the file uploaded here, which stays on disk.
	if got := a.avatarOf(u); got != "https://hel.example.com/storage/a.png" {
		t.Errorf("avatarOf = %q, want the hub picture", got)
	}
	rec := fetch(h, "/avatar/"+strconv.FormatInt(u.ID, 10), c)
	if rec.Code != http.StatusFound || rec.Header().Get("Location") != "https://hel.example.com/storage/a.png" {
		t.Errorf("/avatar answered %d %q, want a redirect to the hub picture", rec.Code, rec.Header().Get("Location"))
	}

	// Without a HOP ID the profile is still edited here, in hub mode or not.
	local, _ := member(t, a, "legacy@example.com")
	lc := sessionFor(t, a, local.ID)
	if rec := post(h, "/settings/profile", url.Values{"name": {"Old Timer"}, "username": {"oldtimer"}}, lc); rec.Code != http.StatusSeeOther {
		t.Errorf("member without HOP ID saving a profile: %d", rec.Code)
	}
	if body := fetch(h, "/settings", lc).Body.String(); !strings.Contains(body, `action="/settings/profile"`) {
		t.Error("member without HOP ID lost the profile form")
	}
}

func TestStandaloneProfileStaysEditable(t *testing.T) {
	a, h := standaloneApp(t)
	u, _ := member(t, a, "solo@example.com")
	c := sessionFor(t, a, u.ID)
	if rec := post(h, "/settings/profile", url.Values{"name": {"Solo"}, "username": {"solo-user"}}, c); rec.Code != http.StatusSeeOther {
		t.Fatalf("stand-alone profile save: %d", rec.Code)
	}
	if body := fetch(h, "/settings", c).Body.String(); !strings.Contains(body, `action="/settings/profile"`) || !strings.Contains(body, `action="/settings/avatar"`) {
		t.Error("stand-alone profile lost its forms")
	}
}

// deleteTrip walks the deletion of a HOP ID member's data to the hub and back: the form, then the
// ticket the hub sends home. authTime is what the hub puts in that ticket.
func deleteTrip(t *testing.T, h http.Handler, c *http.Cookie, email, sub string, authTime int64) *httptest.ResponseRecorder {
	t.Helper()
	rec := post(h, "/settings/delete-account", url.Values{"confirm": {email}}, c)
	if rec.Code != http.StatusFound {
		t.Fatalf("delete form: %d %s", rec.Code, rec.Body.String())
	}
	loc, _ := url.Parse(rec.Header().Get("Location"))
	if loc.Host != "hel.example.com" || loc.Path != "/sso/pixelhop" || loc.Query().Get("max_age") != "300" ||
		loc.Query().Get("return_to") != "https://p.example.com/auth/helink/callback" {
		t.Fatalf("the form did not send the member to the hub for a fresh sign-in: %s", loc)
	}
	var trip *http.Cookie
	for _, ck := range rec.Result().Cookies() {
		if ck.Name == helinkCookie {
			trip = ck
		}
	}
	if trip == nil {
		t.Fatal("no trip cookie")
	}
	tk := ticketFor(loc.Query().Get("state"))
	tk.Sub, tk.Email, tk.AuthTime = sub, email, authTime
	req := httptest.NewRequest("GET", "/auth/helink/callback?ticket="+url.QueryEscape(signTicket(tk, testSecret)), nil)
	req.AddCookie(trip)
	if c != nil {
		req.AddCookie(c)
	}
	out := httptest.NewRecorder()
	h.ServeHTTP(out, req)
	return out
}

// Deleting a HOP ID member's images and profile asks the hub for a fresh sign-in and checks, in the signed
// ticket, that it happened a few minutes ago at most. An older hub that sends no time, a stale sign-in, a
// ticket for somebody else and a visit with no session delete nothing; a fresh one deletes and goes home
// without signing the person out of the hub.
func TestHubMemberDeleteNeedsAFreshSignIn(t *testing.T) {
	a, h := ssoApp(t)
	ctx := t.Context()
	u := linked(t, a, "91", "helena@example.com")
	addImage(t, a, u, "p1")
	c := sessionFor(t, a, u.ID)
	other := linked(t, a, "92", "other@example.com")
	addImage(t, a, other, "p2")

	// The e-mail must still be typed.
	if rec := post(h, "/settings/delete-account", url.Values{"confirm": {"wrong@example.com"}}, c); rec.Code != http.StatusBadRequest {
		t.Fatalf("wrong e-mail: %d", rec.Code)
	}
	// A leftover local password is not what confirms it any more.
	u.PassHash = HashPassword("old-password-1")
	a.db.UpdateUser(ctx, u)
	if rec := post(h, "/settings/delete-account", url.Values{"password": {"old-password-1"}}, c); rec.Code != http.StatusBadRequest {
		t.Fatalf("local password accepted for a HOP ID member: %d", rec.Code)
	}

	untouched := func(why string) {
		t.Helper()
		if _, err := a.db.UserByID(ctx, u.ID); err != nil {
			t.Fatalf("%s: the account was deleted", why)
		}
		if _, err := a.db.Image(ctx, "p1"); err != nil {
			t.Fatalf("%s: the image was deleted", why)
		}
		if _, err := a.db.SessionUser(ctx, hashToken(c.Value)); err != nil {
			t.Fatalf("%s: the session ended", why)
		}
	}
	now := time.Now().Unix()
	for name, at := range map[string]int64{"older hub, no auth_time": 0, "signed in an hour ago": now - 3600, "from the future": now + 3600} {
		rec := deleteTrip(t, h, c, u.Email, "91", at)
		if rec.Code != http.StatusForbidden || !strings.Contains(rec.Body.String(), "nothing was deleted") {
			t.Fatalf("%s: %d", name, rec.Code)
		}
		untouched(name)
	}
	// A fresh sign-in, but of another HOP ID than the one signed in here.
	if rec := deleteTrip(t, h, c, u.Email, "92", now-20); rec.Code != http.StatusForbidden {
		t.Fatalf("ticket of another account: %d", rec.Code)
	}
	untouched("another account's ticket")
	if _, err := a.db.Image(ctx, "p2"); err != nil {
		t.Fatal("the other account's image was deleted")
	}
	// No session at all: the trip cookie alone deletes nothing.
	rec := post(h, "/settings/delete-account", url.Values{"confirm": {u.Email}}, c)
	loc, _ := url.Parse(rec.Header().Get("Location"))
	tk := ticketFor(loc.Query().Get("state"))
	tk.Sub, tk.AuthTime = "91", now
	req := httptest.NewRequest("GET", "/auth/helink/callback?ticket="+url.QueryEscape(signTicket(tk, testSecret)), nil)
	req.AddCookie(rec.Result().Cookies()[0])
	out := httptest.NewRecorder()
	h.ServeHTTP(out, req)
	if out.Code != http.StatusForbidden {
		t.Fatalf("no session: %d", out.Code)
	}
	untouched("no session")

	// Fresh: deleted, back on PixelHop's own home page, never sent to the hub's logout.
	rec = deleteTrip(t, h, c, u.Email, "91", now-30)
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/" {
		t.Fatalf("fresh sign-in: %d %q", rec.Code, rec.Header().Get("Location"))
	}
	if _, err := a.db.UserByID(ctx, u.ID); err == nil {
		t.Error("the account was not deleted")
	}
	if _, err := a.db.Image(ctx, "p1"); err == nil {
		t.Error("the image was not deleted")
	}
	if _, err := a.db.UserByID(ctx, other.ID); err != nil {
		t.Error("someone else's account went with it")
	}
	if !hasCookieNamed(rec, sessionCookie, "") {
		t.Error("the session cookie was not cleared")
	}
}

func hasCookieNamed(rec *httptest.ResponseRecorder, name, value string) bool {
	for _, c := range rec.Result().Cookies() {
		if c.Name == name && c.Value == value {
			return true
		}
	}
	return false
}

func TestSignedInRecently(t *testing.T) {
	now := time.Unix(1_800_000_000, 0)
	for _, c := range []struct {
		at   int64
		want bool
	}{{0, false}, {-5, false}, {now.Unix() - 30, true}, {now.Unix() - 359, true}, {now.Unix() - 361, false}, {now.Unix() + 30, true}, {now.Unix() + 120, false}} {
		if got := signedInRecently(c.at, now, 300*time.Second); got != c.want {
			t.Errorf("authTime offset %d: %v, want %v", c.at-now.Unix(), got, c.want)
		}
	}
}

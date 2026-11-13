package app

import (
	"database/sql"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/navi-crwn/pixelhop/internal/db"
)

// browser is one cookie jar talking to the handler.
type browser struct {
	t       *testing.T
	h       http.Handler
	cookies map[string]*http.Cookie
}

func newBrowser(t *testing.T, h http.Handler) *browser {
	return &browser{t: t, h: h, cookies: map[string]*http.Cookie{}}
}

func (b *browser) do(method, target string, hdr map[string]string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, target, nil)
	for _, c := range b.cookies {
		req.AddCookie(c)
	}
	for k, v := range hdr {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	b.h.ServeHTTP(rec, req)
	for _, c := range rec.Result().Cookies() {
		if c.MaxAge < 0 || c.Value == "" {
			delete(b.cookies, c.Name)
		} else {
			b.cookies[c.Name] = c
		}
	}
	return rec
}

// page loads a page the way a browser address bar does.
func (b *browser) page(target string) *httptest.ResponseRecorder {
	return b.do("GET", target, map[string]string{"Sec-Fetch-Dest": "document", "Accept": "text/html"})
}

func (b *browser) email() string {
	rec := b.do("GET", "/api/v1/me", map[string]string{"Accept": "application/json"})
	if rec.Code != 200 {
		return ""
	}
	body := rec.Body.String()
	i := strings.Index(body, `"email":"`)
	if i < 0 {
		return ""
	}
	s := body[i+9:]
	return s[:strings.Index(s, `"`)]
}

// signIn runs the normal ticket flow for hub account sub/email in this browser.
func (b *browser) signIn(sub, email string) {
	b.t.Helper()
	rec := b.page("/auth/helink?next=/dashboard")
	loc, _ := url.Parse(rec.Header().Get("Location"))
	rec = b.page(hubAnswer(loc.Query().Get("state"), sub, email))
	if rec.Code != http.StatusSeeOther || b.cookies[sessionCookie] == nil {
		b.t.Fatalf("sign-in as %s: %d %s", email, rec.Code, rec.Body.String())
	}
}

// age makes the hub's last word on every hub session older than d.
func (b *browser) age(d time.Duration, a *App) {
	b.t.Helper()
	if _, err := a.db.Exec(`UPDATE sessions SET verified_at=? WHERE verified_at>0`, time.Now().Add(-d).Unix()); err != nil {
		b.t.Fatal(err)
	}
	delete(b.cookies, recheckCookie)
}

// silent starts the quiet trip a stale page load was sent on and returns its state.
func (b *browser) silent(loc string) string {
	b.t.Helper()
	if !strings.HasPrefix(loc, "/auth/helink?prompt=none&") {
		b.t.Fatalf("not a quiet trip: %s", loc)
	}
	rec := b.page(loc)
	hub, _ := url.Parse(rec.Header().Get("Location"))
	if hub.Host != "hel.example.com" || hub.Path != "/sso/pixelhop" || hub.Query().Get("prompt") != "none" {
		b.t.Fatalf("hub request: %s", hub)
	}
	return hub.Query().Get("state")
}

func hubAnswer(state, sub, email string) string {
	tk := ticketFor(state)
	tk.Sub, tk.Email = sub, email
	return "/auth/helink/callback?ticket=" + url.QueryEscape(signTicket(tk, testSecret))
}

func TestSilentRecheckSwitchesToTheHubAccount(t *testing.T) {
	a, h := ssoApp(t)
	a.hubUp = func() bool { return true }
	b := newBrowser(t, h)
	b.signIn("42", "a@example.com")
	if b.email() != "a@example.com" {
		t.Fatal("not signed in as A")
	}
	oldSession := b.cookies[sessionCookie].Value

	// Fresh session: pages load, nobody is sent anywhere.
	if rec := b.page("/dashboard"); rec.Code != 200 {
		t.Fatalf("fresh dashboard: %d", rec.Code)
	}

	// The hub switched to B while this app still holds A.
	b.age(10*time.Minute, a)
	// Not a page load (fetch, image, API): served as before, never redirected.
	if rec := b.do("GET", "/api/v1/me", map[string]string{"Accept": "application/json"}); rec.Code != 200 {
		t.Fatalf("api while stale: %d", rec.Code)
	}
	if rec := b.do("GET", "/dashboard", map[string]string{"Sec-Fetch-Dest": "empty", "Accept": "*/*"}); rec.Code != 200 {
		t.Fatalf("fetch while stale: %d", rec.Code)
	}
	rec := b.page("/dashboard")
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("stale page load not re-checked: %d", rec.Code)
	}
	state := b.silent(rec.Header().Get("Location"))

	// The same page load again in the same minute does not start a second trip.
	if rec := b.page("/dashboard"); rec.Code != 200 {
		t.Fatalf("second trip inside the cooldown: %d", rec.Code)
	}

	rec = b.page(hubAnswer(state, "43", "b@example.com"))
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/dashboard" {
		t.Fatalf("callback: %d %s", rec.Code, rec.Header().Get("Location"))
	}
	if b.email() != "b@example.com" {
		t.Fatalf("still signed in as %q", b.email())
	}
	if _, err := a.db.SessionUser(t.Context(), hashToken(oldSession)); err == nil {
		t.Fatal("A's session still alive in this browser")
	}
}

func TestSilentRecheckSameAccountCarriesOn(t *testing.T) {
	a, h := ssoApp(t)
	a.hubUp = func() bool { return true }
	b := newBrowser(t, h)
	b.signIn("42", "a@example.com")
	b.age(10*time.Minute, a)
	rec := b.page("/about?x=1")
	state := b.silent(rec.Header().Get("Location"))
	rec = b.page(hubAnswer(state, "42", "a@example.com"))
	if rec.Header().Get("Location") != "/about?x=1" || b.email() != "a@example.com" {
		t.Fatalf("same account: %s %q", rec.Header().Get("Location"), b.email())
	}
	// Vouched for again: no new trip.
	delete(b.cookies, recheckCookie)
	if rec := b.page("/dashboard"); rec.Code != 200 {
		t.Fatalf("re-verified session re-checked again: %d", rec.Code)
	}
}

func TestSilentRecheckLoginRequiredEndsTheSession(t *testing.T) {
	a, h := ssoApp(t)
	a.hubUp = func() bool { return true }
	b := newBrowser(t, h)
	b.signIn("42", "a@example.com")
	b.age(10*time.Minute, a)

	// A public page: the visitor carries on as a guest.
	rec := b.page("/about")
	state := b.silent(rec.Header().Get("Location"))
	rec = b.page("/auth/helink/callback?error=login_required&state=" + url.QueryEscape(state))
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/about" {
		t.Fatalf("login_required: %d %s", rec.Code, rec.Header().Get("Location"))
	}
	if b.cookies[sessionCookie] != nil || b.email() != "" {
		t.Fatal("session survived login_required")
	}
	if rec := b.page("/about"); rec.Code != 200 {
		t.Fatalf("public page as guest: %d", rec.Code)
	}
	// The dashboard goes to the sign-in flow.
	if rec := b.page("/dashboard"); rec.Code != http.StatusSeeOther || !strings.HasPrefix(rec.Header().Get("Location"), "/login") {
		t.Fatalf("dashboard as guest: %d %s", rec.Code, rec.Header().Get("Location"))
	}
}

func TestSilentRecheckSafeguards(t *testing.T) {
	a, h := ssoApp(t)
	up := true
	a.hubUp = func() bool { return up }
	b := newBrowser(t, h)
	b.signIn("42", "a@example.com")
	b.age(10*time.Minute, a)

	// Hub unreachable: no redirect, the session keeps working.
	up = false
	if rec := b.page("/dashboard"); rec.Code != 200 {
		t.Fatalf("hub down: %d", rec.Code)
	}
	up = true

	// Prefetches, images, frames and probes are never sent round.
	for _, hdr := range []map[string]string{
		{"Sec-Fetch-Dest": "document", "Sec-Purpose": "prefetch"},
		{"Sec-Fetch-Dest": "image"},
		{"Sec-Fetch-Dest": "iframe"},
	} {
		if rec := b.do("GET", "/dashboard", hdr); rec.Code != 200 {
			t.Fatalf("%v: %d", hdr, rec.Code)
		}
	}
	if rec := b.page("/healthz"); rec.Code != 200 {
		t.Fatalf("healthz: %d", rec.Code)
	}

	// A quiet trip the hub answers with "try later" keeps the session and does not loop.
	rec := b.page("/dashboard")
	state := b.silent(rec.Header().Get("Location"))
	rec = b.page("/auth/helink/callback?error=temporarily_unavailable&state=" + url.QueryEscape(state))
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/dashboard" || b.cookies[sessionCookie] == nil {
		t.Fatalf("temporarily_unavailable: %d %s", rec.Code, rec.Header().Get("Location"))
	}
	if rec := b.page("/dashboard"); rec.Code != 200 {
		t.Fatalf("looped after temporarily_unavailable: %d", rec.Code)
	}

	// The error answer is accepted only for our own quiet trip with our own state.
	rec = b.page("/auth/helink?next=/dashboard") // a normal trip
	loc, _ := url.Parse(rec.Header().Get("Location"))
	if rec := b.page("/auth/helink/callback?error=login_required&state=" + url.QueryEscape(loc.Query().Get("state"))); rec.Code != http.StatusBadRequest || b.cookies[sessionCookie] == nil {
		t.Fatalf("error answer to a normal trip: %d", rec.Code)
	}
	b.age(10*time.Minute, a)
	rec = b.page("/dashboard")
	b.silent(rec.Header().Get("Location"))
	if rec := b.page("/auth/helink/callback?error=login_required&state=forged"); rec.Code != http.StatusBadRequest || b.cookies[sessionCookie] == nil {
		t.Fatalf("forged state: %d", rec.Code)
	}
}

func TestSilentRecheckLeavesOtherSessionsAlone(t *testing.T) {
	a, h := ssoApp(t)
	a.hubUp = func() bool { return true }
	u := &db.User{Email: "local@example.com", QuotaBytes: 1 << 20, HelinkID: sql.NullString{String: "77", Valid: true}}
	if err := a.db.InsertUser(t.Context(), u); err != nil {
		t.Fatal(err)
	}

	// A session the hub never vouched for (password, Google) is not touched, however old.
	tok := randomToken(32)
	if err := a.db.InsertSession(t.Context(), hashToken(tok), u.ID, time.Hour, "", ""); err != nil {
		t.Fatal(err)
	}
	b := newBrowser(t, h)
	b.cookies[sessionCookie] = &http.Cookie{Name: sessionCookie, Value: tok}
	if rec := b.page("/dashboard"); rec.Code != 200 {
		t.Fatalf("detached session re-checked: %d", rec.Code)
	}

	// A hub-linked session from before the column existed (0) is checked once.
	if _, err := a.db.Exec(`UPDATE sessions SET verified_at=0`); err != nil {
		t.Fatal(err)
	}
	if rec := b.page("/dashboard"); rec.Code != http.StatusSeeOther || !strings.Contains(rec.Header().Get("Location"), "prompt=none") {
		t.Fatalf("old session not re-checked: %d %s", rec.Code, rec.Header().Get("Location"))
	}

	// An API key is never part of this.
	c := newBrowser(t, h)
	rec := c.do("GET", "/dashboard", map[string]string{"Authorization": "Bearer ph_nonsense", "Sec-Fetch-Dest": "document", "Accept": "text/html"})
	if strings.Contains(rec.Header().Get("Location"), "prompt=none") {
		t.Fatal("API key request sent through the hub")
	}
}

func TestSignInEndsTheOldSessionInThisBrowser(t *testing.T) {
	a, h := ssoApp(t)
	b := newBrowser(t, h)
	b.signIn("42", "a@example.com")
	old := b.cookies[sessionCookie].Value
	b.signIn("43", "b@example.com")
	if _, err := a.db.SessionUser(t.Context(), hashToken(old)); err == nil {
		t.Fatal("old session survived a new sign-in")
	}
	if b.email() != "b@example.com" {
		t.Fatalf("signed in as %q", b.email())
	}
}

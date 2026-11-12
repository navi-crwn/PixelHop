package app

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// The getting-started card is a family reminder card: "Hide" (hop-ui.js) posts
// here, the card stays away for seven days and comes back while steps are
// still open, only for the member who hid it, and is gone for good once all
// four steps are done. The endpoint takes known keys from a signed-in member only.
func TestGettingStartedReminder(t *testing.T) {
	a, h := ssoApp(t)
	ctx := context.Background()
	u, key := member(t, a, "remind@example.com")
	other, _ := member(t, a, "remind2@example.com")
	session := func(id int64) *http.Cookie {
		tok := randomToken(32)
		a.db.InsertSession(ctx, hashToken(tok), id, time.Hour, "", "")
		return &http.Cookie{Name: sessionCookie, Value: tok}
	}
	me, them := session(u.ID), session(other.ID)
	card := func(c *http.Cookie) bool {
		return strings.Contains(fetch(h, "/dashboard", c).Body.String(),
			`class="dash-start" data-hop-remind="getting-started" data-hop-remind-url="/api/me/reminders/dismiss"`)
	}
	dismiss := func(key string, c *http.Cookie) int {
		req := httptest.NewRequest("POST", "/api/me/reminders/dismiss", strings.NewReader(`{"key":"`+key+`"}`))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Sec-Fetch-Site", "same-origin")
		if c != nil {
			req.AddCookie(c)
		}
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec.Code
	}

	if got := dismiss("getting-started", nil); got != http.StatusUnauthorized {
		t.Fatalf("signed out: %d", got)
	}
	if !card(me) || !card(them) {
		t.Fatal("steps open: both members see the card")
	}
	if got := dismiss("something-else", me); got != http.StatusBadRequest || !card(me) {
		t.Fatalf("an unknown key must hide nothing: %d", got)
	}
	if got := dismiss("getting-started", me); got != http.StatusOK {
		t.Fatalf("dismiss: %d", got)
	}
	if card(me) {
		t.Fatal("hidden just now: the card must be gone (also after a reload)")
	}
	if !card(them) {
		t.Fatal("another member's card must stay")
	}
	if body := fetch(h, "/dashboard").Body.String(); strings.Contains(body, "dash-start") {
		t.Fatal("a guest sees no card")
	}

	age := func(days int) {
		if _, err := a.db.Exec(`UPDATE reminders SET dismissed_at = ? WHERE user_id = ?`, time.Now().Add(-time.Duration(days)*24*time.Hour).Unix(), u.ID); err != nil {
			t.Fatal(err)
		}
	}
	age(6)
	if card(me) {
		t.Fatal("six days after hiding: still hidden")
	}
	age(7)
	if !card(me) {
		t.Fatal("seven days after hiding, steps still open: the card must be back")
	}

	// All four steps done: gone for good, however old the dismissal is.
	upload(t, h, key, 2) // files and an album
	if _, err := a.db.Exec(`UPDATE users SET username = 'helena', show_owner = 1 WHERE id = ?`, u.ID); err != nil {
		t.Fatal(err)
	}
	age(30)
	if card(me) {
		t.Fatal("all four steps done: no card")
	}
}

// The migration can meet a database that already has the table.
func TestReminderMigrationIsIdempotent(t *testing.T) {
	a, _ := ssoApp(t)
	stmt := `CREATE TABLE IF NOT EXISTS reminders (user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE, key TEXT NOT NULL,
	dismissed_at INTEGER NOT NULL, PRIMARY KEY (user_id, key));`
	if _, err := a.db.Exec(stmt); err != nil {
		t.Fatalf("second run: %v", err)
	}
}

// The dashboard no longer advertises the hub as "Short links with the same
// account": short links are LinkHop's, and the sidebar lists the family.
func TestDashboardHasNoHubShortcut(t *testing.T) {
	a, h := ssoApp(t)
	u, _ := member(t, a, "nohub@example.com")
	tok := randomToken(32)
	a.db.InsertSession(context.Background(), hashToken(tok), u.ID, time.Hour, "", "")
	body := fetch(h, "/dashboard", &http.Cookie{Name: sessionCookie, Value: tok}).Body.String()
	if strings.Contains(body, "Short links with the same account") || strings.Contains(body, `class="shortcut" href="https://hel.example.com/dashboard"`) {
		t.Fatal("the HEL.ink shortcut card is still on the dashboard")
	}
}

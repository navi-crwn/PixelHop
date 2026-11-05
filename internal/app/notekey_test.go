package app

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

// An encrypted NoteHop link keeps the note key in its #fragment. PixelHop stores profile addresses and report
// text and shows them or hands them to the control center, so it must not keep that part (fake key only).
const (
	fakeNoteKey = "FAKEKEY0notehop0s2audit0000000000000000000"
	fakeNoteURL = "https://note.hel.ink/AbC123xyz0#k=" + fakeNoteKey
)

func TestProfileWebsiteRefusesEncryptedNoteLink(t *testing.T) {
	a, h := ssoApp(t)
	ctx := context.Background()
	u, _, _ := seedOwner(t, a)
	tok := randomToken(32)
	a.db.InsertSession(ctx, hashToken(tok), u.ID, time.Hour, "", "")
	post := func(site string) *httptest.ResponseRecorder {
		form := "name=M&username=maya.r&website=" + strings.NewReplacer("#", "%23", "&", "%26").Replace(site)
		req := httptest.NewRequest("POST", "/settings/profile", strings.NewReader(form))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.AddCookie(&http.Cookie{Name: sessionCookie, Value: tok})
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec
	}
	for name, site := range map[string]string{
		"plain":      fakeNoteURL,
		"short host": "https://n.hel.ink/AbC123xyz0#k=" + fakeNoteKey,
		"no scheme":  "note.hel.ink/AbC123xyz0#k=" + fakeNoteKey,
		"owner link": "https://note.hel.ink/AbC123xyz0#k=" + fakeNoteKey + "&a=ADMINTOKEN0FAKE000000000000000000000000000",
		"full-width": "https://ｎote.hel.ink/AbC123xyz0#k=" + fakeNoteKey,
		"upper+dot":  "HTTPS://NOTE.HEL.INK./AbC123xyz0#k=" + fakeNoteKey,
		"host:port":  "note.hel.ink:443/AbC123xyz0#k=" + fakeNoteKey,
		"hash-query": "https://note.hel.ink/AbC123xyz0#?k=" + fakeNoteKey,
	} {
		if rec := post(site); rec.Code != 400 {
			t.Errorf("%s: an encrypted note address was accepted as the website (%d)", name, rec.Code)
		}
		if got, _ := a.db.UserByID(ctx, u.ID); strings.Contains(got.Website, fakeNoteKey) {
			t.Fatalf("%s: the key is stored in users.website: %q", name, got.Website)
		}
	}
	// Ordinary addresses, and a note address without a key, still work.
	for _, site := range []string{"https://hel.ink/b/maya#k=1", "https://note.hel.ink/AbC123xyz0", "https://example.com/#a=1"} {
		if rec := post(site); rec.Code != http.StatusSeeOther {
			t.Errorf("%s was refused: %d %s", site, rec.Code, rec.Body.String())
		}
	}
}

func TestReportNoteDropsNoteKey(t *testing.T) {
	a, h := ssoApp(t)
	_, im, _ := seedOwner(t, a)
	form := "reason=spam&note=" + strings.NewReplacer("#", "%23", "&", "%26").Replace("look at "+fakeNoteURL+"&a=ADMINTOKEN0FAKE000000000000000000000000000 and https://example.com/#a=1")
	req := httptest.NewRequest("POST", "/report/"+im.ID, strings.NewReader(form))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("report: %d %s", rec.Code, rec.Body.String())
	}
	reports, _ := a.db.Reports(context.Background(), "open")
	if len(reports) != 1 {
		t.Fatalf("reports: %v", reports)
	}
	if n := reports[0].Note; strings.Contains(n, fakeNoteKey) || strings.Contains(n, "ADMINTOKEN0FAKE") || !strings.Contains(n, "https://example.com/#a=1") {
		t.Fatalf("report note: %q", n)
	}
}

// The hub's profile validation only asks for an http(s) address, so what it pushes or puts in a ticket goes through
// the same rule as the settings form.
func TestHubProfileRefusesEncryptedNoteLink(t *testing.T) {
	spellings := map[string]string{
		"plain":      fakeNoteURL,
		"short host": "https://n.hel.ink/AbC123xyz0#k=" + fakeNoteKey,
		"no scheme":  "note.hel.ink/AbC123xyz0#k=" + fakeNoteKey,
		"owner link": "https://note.hel.ink/AbC123xyz0#k=" + fakeNoteKey + "&a=ADMINTOKEN0FAKE000000000000000000000000000",
		"escaped":    "https://note.hel.ink/AbC123xyz0%23k=" + fakeNoteKey,
		"nested":     "https://example.com/?to=https%3A%2F%2Fnote.hel.ink%2FAbC123xyz0%23k%3D" + fakeNoteKey,
		"long":       "https://note.hel.ink/AbC123xyz0#pad=" + strings.Repeat("x", 200) + "&k=" + fakeNoteKey,
	}
	for name, site := range spellings {
		t.Run("push/"+name, func(t *testing.T) {
			a, h := ssoApp(t)
			ctx := context.Background()
			u := linked(t, a, "push", "push@example.com")
			u.Website = "https://example.com/mine"
			a.db.UpdateUser(ctx, u)
			if rec, _ := hopCall(t, h, "helink", "POST", "/hop/profile", map[string]any{"hop_id": "push", "website": site, "bio": "still applied"}); rec.Code != 200 {
				t.Fatalf("push: %d %s", rec.Code, rec.Body.String())
			}
			got, _ := a.db.UserByID(ctx, u.ID)
			if got.Website != "https://example.com/mine" {
				t.Errorf("website = %q, want the earlier address kept", got.Website)
			}
			if got.Bio != "still applied" {
				t.Errorf("the rest of the push was dropped: bio = %q", got.Bio)
			}
		})
		t.Run("ticket/"+name, func(t *testing.T) {
			a, h := ssoApp(t)
			c, state := start(t, h)
			tk := ticketFor(state)
			tk.Website = &site
			if rec := callback(h, c, signTicket(tk, testSecret)); rec.Code != http.StatusSeeOther {
				t.Fatalf("callback: %d %s", rec.Code, rec.Body.String())
			}
			got, err := a.db.UserByHelink(context.Background(), tk.Sub)
			if err != nil {
				t.Fatal(err)
			}
			if got.Website != "" {
				t.Errorf("website = %q, want nothing stored", got.Website)
			}
		})
	}
	// Ordinary addresses, and a note address without a key, still arrive.
	for _, site := range []string{"https://hel.ink/b/maya#k=1", "https://note.hel.ink/AbC123xyz0", "https://example.com/#a=1"} {
		a, h := ssoApp(t)
		u := linked(t, a, "push", "push@example.com")
		hopCall(t, h, "helink", "POST", "/hop/profile", map[string]any{"hop_id": "push", "website": site})
		if got, _ := a.db.UserByID(context.Background(), u.ID); got.Website != site {
			t.Errorf("website = %q, want %q", got.Website, site)
		}
	}
}

// The free-text fields of a hub profile are public too: a pasted link loses its key, the text around it stays.
func TestHubProfileScrubsNoteKeyFromText(t *testing.T) {
	check := func(t *testing.T, a *App, sub string) {
		t.Helper()
		got, err := a.db.UserByHelink(context.Background(), sub)
		if err != nil {
			t.Fatal(err)
		}
		for field, v := range map[string]string{"bio": got.Bio, "name": got.Name} {
			if strings.Contains(v, fakeNoteKey) {
				t.Errorf("%s holds the key: %q", field, v)
			}
		}
		if !strings.HasPrefix(got.Bio, "my notes https://note.hel.ink/AbC123xyz0") {
			t.Errorf("bio lost its text: %q", got.Bio)
		}
	}
	bio := "my notes " + fakeNoteURL
	name := "n.hel.ink/x#k=" + fakeNoteKey

	t.Run("push", func(t *testing.T) {
		a, h := ssoApp(t)
		linked(t, a, "push", "push@example.com")
		hopCall(t, h, "helink", "POST", "/hop/profile", map[string]any{"hop_id": "push", "bio": bio, "name": name})
		check(t, a, "push")
	})
	t.Run("new account ticket", func(t *testing.T) {
		a, h := ssoApp(t)
		c, state := start(t, h)
		tk := ticketFor(state)
		tk.Bio, tk.Name = &bio, name
		callback(h, c, signTicket(tk, testSecret))
		check(t, a, tk.Sub)
	})
	t.Run("existing account ticket", func(t *testing.T) {
		a, h := ssoApp(t)
		u := linked(t, a, "42", "ivan@example.com")
		u.Name = ""
		a.db.UpdateUser(context.Background(), u)
		c, state := start(t, h)
		tk := ticketFor(state)
		tk.Bio, tk.Name = &bio, name
		callback(h, c, signTicket(tk, testSecret))
		check(t, a, tk.Sub)
	})
}

// A key stored before these checks existed is not handed out while the row waits for its owner to change it.
func TestStoredNoteKeyWebsiteIsNotShown(t *testing.T) {
	a, h := ssoApp(t)
	u := linked(t, a, "old", "old@example.com")
	u.Website, u.ShowOwner = fakeNoteURL, true
	if err := a.db.UpdateUser(context.Background(), u); err != nil {
		t.Fatal(err)
	}
	if rec, _ := hopCall(t, h, "helink", "GET", "/hop/users/old", nil); rec.Code != 200 || strings.Contains(rec.Body.String(), fakeNoteKey) {
		t.Errorf("GET /hop/users: %d %s", rec.Code, rec.Body.String())
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/u/"+u.Username.String, nil))
	if rec.Code != 200 || strings.Contains(rec.Body.String(), fakeNoteKey) {
		t.Errorf("profile page: %d, holds key: %v", rec.Code, strings.Contains(rec.Body.String(), fakeNoteKey))
	}
}

// The settings form reads the same rule: an escaped # and an address inside another one are refused, and a bio keeps
// its text but not the key.
func TestProfileFormRefusesEscapedNoteKey(t *testing.T) {
	a, h := ssoApp(t)
	ctx := context.Background()
	u, _, _ := seedOwner(t, a)
	tok := randomToken(32)
	a.db.InsertSession(ctx, hashToken(tok), u.ID, time.Hour, "", "")
	post := func(field, value string) *httptest.ResponseRecorder {
		form := url.Values{"name": {"M"}, "username": {"maya.r"}, field: {value}}.Encode()
		req := httptest.NewRequest("POST", "/settings/profile", strings.NewReader(form))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.AddCookie(&http.Cookie{Name: sessionCookie, Value: tok})
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec
	}
	for _, site := range []string{
		"https://note.hel.ink/AbC123xyz0%23k=" + fakeNoteKey,
		"https://example.com/?to=https%3A%2F%2Fnote.hel.ink%2FAbC123xyz0%23k%3D" + fakeNoteKey,
	} {
		if rec := post("website", site); rec.Code != 400 {
			t.Errorf("%s: %d", site, rec.Code)
		}
	}
	if got, _ := a.db.UserByID(ctx, u.ID); strings.Contains(got.Website, fakeNoteKey) {
		t.Fatalf("website holds the key: %q", got.Website)
	}
	if rec := post("bio", "my notes "+fakeNoteURL); rec.Code != http.StatusSeeOther {
		t.Fatalf("bio: %d", rec.Code)
	}
	if got, _ := a.db.UserByID(ctx, u.ID); strings.Contains(got.Bio, fakeNoteKey) || !strings.HasPrefix(got.Bio, "my notes ") {
		t.Fatalf("bio = %q", got.Bio)
	}
}

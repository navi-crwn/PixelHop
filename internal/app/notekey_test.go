package app

import (
	"context"
	"net/http"
	"net/http/httptest"
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

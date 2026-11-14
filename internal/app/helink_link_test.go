package app

import (
	"database/sql"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/navi-crwn/pixelhop/internal/db"
)

func strp(s string) *string { return &s }

// An older PixelHop account that meets HOP ID through the same e-mail is linked, but the password and
// Google login it carried stop working, its old sessions end, and what the person wrote in the bio
// and website here is not wiped by an empty profile on the hub.
func TestLinkingByEmailClosesTheOldDoors(t *testing.T) {
	a, h := ssoApp(t)
	ctx := t.Context()
	old := &db.User{Email: "ivan@example.com", Name: "Ivan", QuotaBytes: 1 << 20, PassHash: HashPassword("old-password-1"),
		GoogleID: sql.NullString{String: "g-1", Valid: true}, Bio: "Photographer", Website: "https://ivan.example",
		Username: sql.NullString{String: "ivan", Valid: true}}
	if err := a.db.InsertUser(ctx, old); err != nil {
		t.Fatal(err)
	}
	a.db.UpdateUser(ctx, old) // InsertUser keeps only the sign-up columns
	oldTok := randomToken(32)
	if err := a.db.InsertSession(ctx, hashToken(oldTok), old.ID, time.Hour, "", ""); err != nil {
		t.Fatal(err)
	}

	c, state := start(t, h)
	tk := ticketFor(state)
	tk.Bio, tk.Website = strp(""), strp("") // the hub profile is empty
	rec := callback(h, c, signTicket(tk, testSecret))
	if rec.Code != http.StatusSeeOther || !hasSession(rec) {
		t.Fatalf("callback: %d", rec.Code)
	}
	got, _ := a.db.UserByID(ctx, old.ID)
	if !got.HelinkID.Valid || got.HelinkID.String != "42" {
		t.Fatalf("not linked: %+v", got.HelinkID)
	}
	if got.PassHash != "" || CheckPassword(got.PassHash, "old-password-1") || got.GoogleID.Valid {
		t.Fatalf("old password or Google login survived linking: pass=%q google=%v", got.PassHash, got.GoogleID)
	}
	if _, err := a.db.SessionUser(ctx, hashToken(oldTok)); err == nil {
		t.Fatal("the account's old session survived linking")
	}
	if got.Bio != "Photographer" || got.Website != "https://ivan.example" || got.Username.String != "ivan" {
		t.Fatalf("an empty hub profile wiped the local one: bio=%q website=%q username=%q", got.Bio, got.Website, got.Username.String)
	}

	// The new session works, and from now on the hub's profile wins, empty or not.
	if _, err := a.db.UserByHelink(ctx, "42"); err != nil {
		t.Fatal(err)
	}
	c2, state2 := start(t, h)
	tk2 := ticketFor(state2)
	tk2.Bio, tk2.Website = strp("From the hub"), strp("")
	if rec := callback(h, c2, signTicket(tk2, testSecret)); rec.Code != http.StatusSeeOther {
		t.Fatalf("second sign-in: %d", rec.Code)
	}
	got, _ = a.db.UserByID(ctx, old.ID)
	if got.Bio != "From the hub" || got.Website != "" {
		t.Fatalf("hub did not win on a later sign-in: bio=%q website=%q", got.Bio, got.Website)
	}
}

// A non-empty hub profile does win at the first link; only emptiness is kept out.
func TestLinkingByEmailTakesFilledHubFields(t *testing.T) {
	a, h := ssoApp(t)
	ctx := t.Context()
	old := &db.User{Email: "ivan@example.com", QuotaBytes: 1 << 20, Bio: "local bio", Website: "https://local.example"}
	a.db.InsertUser(ctx, old)
	a.db.UpdateUser(ctx, old)
	c, state := start(t, h)
	tk := ticketFor(state)
	tk.Bio, tk.Website = strp("hub bio"), strp("")
	callback(h, c, signTicket(tk, testSecret))
	got, _ := a.db.UserByID(ctx, old.ID)
	if got.Bio != "hub bio" || got.Website != "https://local.example" {
		t.Fatalf("bio=%q website=%q", got.Bio, got.Website)
	}
}

// A row linked to another HOP ID is never handed to a new one just because the e-mail matches.
func TestLinkingRefusesARowOfAnotherHopID(t *testing.T) {
	a, h := ssoApp(t)
	ctx := t.Context()
	owner := &db.User{Email: "ivan@example.com", QuotaBytes: 1 << 20, HelinkID: sql.NullString{String: "10", Valid: true}, Bio: "mine"}
	a.db.InsertUser(ctx, owner)
	a.db.UpdateUser(ctx, owner)
	n := a.db.CountUsers(ctx)

	c, state := start(t, h)
	rec := callback(h, c, signTicket(ticketFor(state), testSecret)) // sub 42, the same e-mail
	if rec.Code != http.StatusConflict || hasSession(rec) || !strings.Contains(rec.Body.String(), "already belongs to another PixelHop account") {
		t.Fatalf("callback: %d session=%v", rec.Code, hasSession(rec))
	}
	got, _ := a.db.UserByID(ctx, owner.ID)
	if got.HelinkID.String != "10" || got.Bio != "mine" || a.db.CountUsers(ctx) != n {
		t.Fatalf("data moved or created: helink=%q users %d→%d", got.HelinkID.String, n, a.db.CountUsers(ctx))
	}
	if _, err := a.db.UserByHelink(ctx, "42"); err == nil {
		t.Fatal("the new HOP ID was linked to some row")
	}
}

// A member whose hub e-mail changed to one another PixelHop row holds gets a message, not a 500, and
// nothing changes.
func TestLinkingHubEmailTakenByAnotherRow(t *testing.T) {
	a, h := ssoApp(t)
	ctx := t.Context()
	me := &db.User{Email: "ivan@example.com", QuotaBytes: 1 << 20, HelinkID: sql.NullString{String: "42", Valid: true}}
	other := &db.User{Email: "taken@example.com", QuotaBytes: 1 << 20}
	a.db.InsertUser(ctx, me)
	a.db.InsertUser(ctx, other)

	c, state := start(t, h)
	tk := ticketFor(state)
	tk.Email = "Taken@Example.com"
	rec := callback(h, c, signTicket(tk, testSecret))
	if rec.Code != http.StatusConflict || hasSession(rec) {
		t.Fatalf("callback: %d session=%v", rec.Code, hasSession(rec))
	}
	got, _ := a.db.UserByID(ctx, me.ID)
	if got.Email != "ivan@example.com" {
		t.Fatalf("e-mail changed to %q", got.Email)
	}
}

package app

import (
	"database/sql"
	"net/http"
	"testing"

	"github.com/navi-crwn/pixelhop/internal/db"
)

// An old member who used Google only and now signs in with HOP ID: the same e-mail gets the old account
// and its pictures back; a different e-mail gets an empty account, and the old one stays untouched for the
// owner's merge command.
func TestOldGoogleMemberMeetsHopID(t *testing.T) {
	a, h := ssoApp(t)
	ctx := t.Context()
	old := &db.User{Email: "ivan@example.com", Name: "Ivan", QuotaBytes: 1 << 30, GoogleID: sql.NullString{String: "g-1", Valid: true}}
	if err := a.db.InsertUser(ctx, old); err != nil {
		t.Fatal(err)
	}
	pic := addImage(t, a, old, "old-pic")

	// Same e-mail (the usual case): linked, pictures still there, the Google link closed.
	c, state := start(t, h)
	if rec := callback(h, c, signTicket(ticketFor(state), testSecret)); rec.Code != http.StatusSeeOther {
		t.Fatalf("callback %d", rec.Code)
	}
	got, err := a.db.UserByHelink(ctx, "42")
	if err != nil || got.ID != old.ID || got.GoogleID.Valid || got.PassHash != "" {
		t.Fatalf("not the old account, or an old way in is left: %+v %v", got, err)
	}
	if im, err := a.db.Image(ctx, pic.ID); err != nil || im.UserID.Int64 != old.ID {
		t.Fatalf("the old picture is not with the account: %v", err)
	}
	n := a.db.CountUsers(ctx)

	// A different e-mail: a new empty account, the old one untouched.
	c2, st2 := start(t, h)
	tk := ticketFor(st2)
	tk.Sub, tk.Email, tk.Jti = "99", "ivan.other@example.com", randomToken(12)
	if rec := callback(h, c2, signTicket(tk, testSecret)); rec.Code != http.StatusSeeOther {
		t.Fatalf("callback %d", rec.Code)
	}
	fresh, err := a.db.UserByHelink(ctx, "99")
	if err != nil || fresh.ID == old.ID || a.db.CountUsers(ctx) != n+1 {
		t.Fatalf("expected one new account: %+v %v", fresh, err)
	}
	if _, count := a.db.UserStorage(ctx, fresh.ID); count != 0 {
		t.Error("the new account is not empty")
	}
}

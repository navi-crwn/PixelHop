package app

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"testing"
)

// The password form is hidden from HOP ID members because the password belongs to the hub, but a hand-made
// POST used to set a PixelHop-only password on the account anyway (and sign every other browser out).
// The server refuses it now; a member with a local password still changes it.
func TestSavePasswordRefusedForHopIDMembers(t *testing.T) {
	a, h := ssoApp(t)
	ctx := context.Background()
	hop := linked(t, a, "91", "sso-pw@example.com")
	s := sessionFor(t, a, hop.ID)
	other := sessionFor(t, a, hop.ID) // a second browser of the same member

	rec := post(h, "/settings/password", url.Values{"password": {"a-new-password-1"}}, s)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("HOP ID member's password POST: %d, want 400", rec.Code)
	}
	// The refusal is a Settings page that opens the Security & sessions tab, where the form would have been.
	if body := rec.Body.String(); !strings.Contains(body, `data-start="sessions"`) || !strings.Contains(body, "managed in HOP ID") {
		t.Error("the refusal page does not open the sessions tab or does not say why")
	}
	if cc := rec.Result().Header.Get("Cache-Control"); cc != "private, no-store" {
		t.Errorf("the refusal page: Cache-Control %q, want private, no-store", cc)
	}
	if got, _ := a.db.UserByID(ctx, hop.ID); got.PassHash != "" {
		t.Error("a local password was set on a HOP ID account")
	}
	if _, err := a.db.SessionUser(ctx, hashToken(other.Value)); err != nil {
		t.Errorf("the refused request signed the other browser out: %v", err)
	}

	local, _ := member(t, a, "local-pw@example.com")
	local.PassHash = HashPassword("current-password-0")
	a.db.UpdateUser(ctx, local)
	ls := sessionFor(t, a, local.ID)
	before, _ := a.db.UserByID(ctx, local.ID)
	rec = post(h, "/settings/password", url.Values{"current": {"current-password-0"}, "password": {"another-password-2"}}, ls)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("local member's password change: %d, want 303", rec.Code)
	}
	if after, _ := a.db.UserByID(ctx, local.ID); after.PassHash == before.PassHash {
		t.Error("a local member's password did not change")
	}
}

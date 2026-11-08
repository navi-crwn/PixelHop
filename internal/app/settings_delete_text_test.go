package app

import (
	"net/http"
	"net/url"
	"strings"
	"testing"
)

// Settings said "Delete my images" above a button that read "Delete my account" and a confirmation about
// deleting the account (dashboard test 2026-11-08, finding #13). For a HOP ID member the action deletes the
// PixelHop images and profile while the account stays at the hub; every word now says that. A member with a
// local account still deletes the account.
func TestDeleteWordingMatchesTheAction(t *testing.T) {
	a, h := ssoApp(t)
	hop := linked(t, a, "77", "hop@example.com")
	body := fetch(h, "/settings", sessionFor(t, a, hop.ID)).Body.String()
	for _, want := range []string{">Delete my images and profile<", "Your HOP ID stays", "Delete your PixelHop profile and all your images?"} {
		if !strings.Contains(body, want) {
			t.Errorf("HOP ID member's settings lack %q", want)
		}
	}
	for _, stray := range []string{"Delete my account", "Delete your account"} {
		if strings.Contains(body, stray) {
			t.Errorf("HOP ID member's settings still say %q", stray)
		}
	}
	// The error for a wrong confirmation speaks of the same thing.
	rec := post(h, "/settings/delete-account", url.Values{"confirm": {"wrong@example.com"}}, sessionFor(t, a, hop.ID))
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "confirm deleting your images and profile") {
		t.Errorf("wrong confirmation: %d, wording about images and profile: %v", rec.Code, strings.Contains(rec.Body.String(), "confirm deleting your images and profile"))
	}
	if rec := fetch(h, "/settings", sessionFor(t, a, hop.ID)); rec.Code != http.StatusOK {
		t.Errorf("after a wrong confirmation the member is gone: settings answers %d", rec.Code)
	}

	local, _ := member(t, a, "local@example.com")
	body = fetch(h, "/settings", sessionFor(t, a, local.ID)).Body.String()
	for _, want := range []string{">Delete account<", ">Delete my account<", "Delete your account and all your images?"} {
		if !strings.Contains(body, want) {
			t.Errorf("local member's settings lack %q", want)
		}
	}
}

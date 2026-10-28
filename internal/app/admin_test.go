package app

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"
)

func sessionFor(t *testing.T, a *App, id int64) *http.Cookie {
	t.Helper()
	tok := randomToken(32)
	if err := a.db.InsertSession(context.Background(), hashToken(tok), id, time.Hour, "", ""); err != nil {
		t.Fatal(err)
	}
	return &http.Cookie{Name: sessionCookie, Value: tok}
}

// The admin pages are gone: /admin tells a signed-in admin where things
// went, or sends them to the control center when its address is set, and
// keeps everyone else out as before.
func TestAdminPagesRetired(t *testing.T) {
	a, h := ssoApp(t)
	ctx := context.Background()
	admin, _ := member(t, a, "boss@example.com")
	admin.Role = "admin"
	a.db.UpdateUser(ctx, admin)
	plain, _ := member(t, a, "tim@example.com")
	bossCookie, timCookie := sessionFor(t, a, admin.ID), sessionFor(t, a, plain.ID)

	if rec := fetch(h, "/admin"); rec.Code != http.StatusSeeOther || !strings.HasPrefix(rec.Header().Get("Location"), "/login?next=") {
		t.Fatalf("guest: %d %s", rec.Code, rec.Header().Get("Location"))
	}
	if rec := fetch(h, "/admin/users", timCookie); rec.Code != http.StatusForbidden {
		t.Fatalf("member: %d", rec.Code)
	}
	for _, path := range []string{"/admin", "/admin/users?q=tim", "/admin/reports"} {
		rec := fetch(h, path, bossCookie)
		if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "Moderation and settings now live in the HOP control center") {
			t.Fatalf("%s: %d", path, rec.Code)
		}
	}
	// Nothing left of the old pages, and no "Admin" entry in the menu.
	for _, want := range []string{`href="/admin"`, `>Admin<`} {
		if body := fetch(h, "/dashboard", bossCookie).Body.String(); strings.Contains(body, want) {
			t.Errorf("dashboard still has %s", want)
		}
	}
	if rec := fetch(h, "/admin/settings", bossCookie); rec.Code != http.StatusOK {
		t.Fatalf("settings: %d", rec.Code)
	}

	a.cfg.ControlURL = "https://control.example.com"
	if rec := fetch(h, "/admin/bans", bossCookie); rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "https://control.example.com" {
		t.Fatalf("redirect: %d %s", rec.Code, rec.Header().Get("Location"))
	}
	if rec := fetch(h, "/admin", timCookie); rec.Code != http.StatusForbidden {
		t.Fatalf("member with control URL: %d", rec.Code)
	}
	body := fetch(h, "/dashboard", bossCookie).Body.String()
	if !strings.Contains(body, `href="https://control.example.com"`) || !strings.Contains(body, "Control center") {
		t.Error("admin menu has no control center link")
	}
	if body := fetch(h, "/dashboard", timCookie).Body.String(); strings.Contains(body, "control.example.com") {
		t.Error("members see the control center link")
	}
}

// Admins still see hidden images (to review them); everyone else gets 451.
func TestAdminSeesHiddenImages(t *testing.T) {
	a, h := ssoApp(t)
	ctx := context.Background()
	admin, _ := member(t, a, "boss@example.com")
	admin.Role = "admin"
	a.db.UpdateUser(ctx, admin)
	owner, key := member(t, a, "owner@example.com")
	id := upload(t, h, key, 1)[0]
	if err := a.db.SetImageHidden(ctx, id, true); err != nil {
		t.Fatal(err)
	}
	im, _ := a.db.Image(ctx, id)

	for _, path := range []string{"/" + id, "/i/" + im.ID + "." + im.Ext} {
		if rec := fetch(h, path); rec.Code != http.StatusUnavailableForLegalReasons {
			t.Errorf("guest %s: %d", path, rec.Code)
		}
		if rec := fetch(h, path, sessionFor(t, a, owner.ID)); rec.Code != http.StatusUnavailableForLegalReasons {
			t.Errorf("owner %s: %d", path, rec.Code)
		}
		if rec := fetch(h, path, sessionFor(t, a, admin.ID)); rec.Code != http.StatusOK {
			t.Errorf("admin %s: %d", path, rec.Code)
		}
	}
}

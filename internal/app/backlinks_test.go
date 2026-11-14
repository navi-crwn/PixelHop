package app

import (
	"database/sql"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"

	"github.com/navi-crwn/pixelhop/internal/db"
)

// Someone who is signed in is never sent to the public home page by a "Back" link, a logo or a
// button on a tool or information page: their way back is the dashboard. A guest still goes home.
func TestBackAndLogoLinksFollowSignIn(t *testing.T) {
	a, h := ssoApp(t)
	u, _ := member(t, a, "back@example.com")
	c := sessionFor(t, a, u.ID)
	back := regexp.MustCompile(`class="back" href="([^"]*)"`)
	brand := regexp.MustCompile(`class="(?:brand|shell-brand)" href="([^"]*)"`)

	for _, path := range []string{"/changelog", "/help", "/legal", "/docs", "/about", "/features", "/faq", "/contact", "/status"} {
		for who, want := range map[string]string{"member": "/dashboard", "guest": "/"} {
			var body string
			if who == "member" {
				body = fetch(h, path, c).Body.String()
			} else {
				body = fetch(h, path).Body.String()
			}
			if m := back.FindStringSubmatch(body); m == nil || m[1] != want {
				t.Errorf("%s as %s: Back link is %v, want %s", path, who, m, want)
			}
			if m := brand.FindStringSubmatch(body); m == nil || m[1] != want {
				t.Errorf("%s as %s: logo link is %v, want %s", path, who, m, want)
			}
		}
	}
	// Error and notice pages offer "Upload" and "Go back" buttons.
	for who, want := range map[string]string{"member": "/dashboard", "guest": "/"} {
		var rec = fetch(h, "/no-such-page-here")
		if who == "member" {
			rec = fetch(h, "/no-such-page-here", c)
		}
		if body := rec.Body.String(); !strings.Contains(body, `class="btn primary" href="`+want+`"`) {
			t.Errorf("404 page as %s: the Upload button does not lead to %s", who, want)
		}
	}
}

// The studio, opened by someone who is signed in, is a page inside the dashboard frame (sidebar and top
// bar), so nothing in it needs a "Back" link. A guest keeps the public page and its Back link.
func TestStudioInsideTheDashboardFrame(t *testing.T) {
	a, h := ssoApp(t)
	u, _ := member(t, a, "studio@example.com")
	c := sessionFor(t, a, u.ID)
	for _, path := range []string{"/tools", "/tools/batch"} {
		body := fetch(h, path, c).Body.String()
		if !strings.Contains(body, `class="shell"`) || !strings.Contains(body, `class="shell-side"`) {
			t.Errorf("%s as a member is outside the dashboard frame", path)
		}
		if strings.Contains(body, `class="back"`) {
			t.Errorf("%s inside the frame still has a Back link", path)
		}
		if guest := fetch(h, path).Body.String(); strings.Contains(guest, `class="shell"`) {
			t.Errorf("%s: a guest got the dashboard frame", path)
		}
	}
	body := fetch(h, "/tools", c).Body.String()
	if !strings.Contains(body, `href="/tools" aria-current="page"`) {
		t.Error("the sidebar does not mark Studio as the current page")
	}
	if !strings.Contains(fetch(h, "/tools").Body.String(), `class="back"`) {
		t.Error("a guest's studio lost its Back link")
	}
}

// An owner who opens their own album, its editor or their own profile from the dashboard has a way back to it.
// Everyone else (other members, guests) sees the page exactly as before, with no such link, and the public pages
// stay indexable.
func TestOwnerPagesLinkBackToTheDashboard(t *testing.T) {
	a, h := ssoApp(t)
	owner, key := member(t, a, "owner@example.com")
	owner.Username = sql.NullString{String: "helena", Valid: true}
	owner.ShowOwner = true
	if err := a.db.UpdateUser(t.Context(), owner); err != nil {
		t.Fatal(err)
	}
	ids := upload(t, h, key, 2)
	al := &db.Album{ID: "alb-OwnerTest1", Title: "Trip", UserID: sql.NullInt64{Int64: owner.ID, Valid: true}}
	if err := a.db.InsertAlbum(t.Context(), al); err != nil {
		t.Fatal(err)
	}
	for _, id := range ids {
		im, _ := a.db.Image(t.Context(), id)
		im.AlbumID = sql.NullString{String: al.ID, Valid: true}
		a.db.UpdateImageMeta(t.Context(), im)
	}
	other, _ := member(t, a, "other@example.com")
	own, oth := sessionFor(t, a, owner.ID), sessionFor(t, a, other.ID)
	backAlbums := regexp.MustCompile(`class="back" href="/dashboard/albums"`)

	for _, path := range []string{"/a/" + al.ID, "/a/" + al.ID + "/edit"} {
		if body := fetch(h, path, own).Body.String(); !backAlbums.MatchString(body) {
			t.Errorf("%s as the owner has no way back to My albums", path)
		}
	}
	if rec := fetch(h, "/a/"+al.ID, oth); backAlbums.MatchString(rec.Body.String()) || strings.Contains(rec.Body.String(), `class="back"`) {
		t.Error("another member sees a dashboard link on someone else's album")
	}
	guest := fetch(h, "/a/"+al.ID)
	if strings.Contains(guest.Body.String(), `class="back"`) || guest.Header().Get("X-Robots-Tag") != "" {
		t.Error("a guest's album page changed or became non-indexable")
	}
	if rec := fetch(h, "/a/"+al.ID+"/edit", oth); rec.Code != 404 {
		t.Errorf("another member opens the editor: %d, want 404", rec.Code)
	}
	// the owner's own profile keeps its member row with the Dashboard link; other visitors never get it
	if body := fetch(h, "/u/helena", own).Body.String(); !strings.Contains(body, `class="member-nav"`) || !strings.Contains(body, `href="/dashboard"`) {
		t.Error("the owner's profile has no way back to the dashboard")
	}
	for who, c := range map[string]*http.Cookie{"another member": oth, "a guest": nil} {
		var rec *httptest.ResponseRecorder
		if c == nil {
			rec = fetch(h, "/u/helena")
		} else {
			rec = fetch(h, "/u/helena", c)
		}
		if strings.Contains(rec.Body.String(), `class="member-nav"`) || rec.Header().Get("X-Robots-Tag") != "" {
			t.Errorf("%s sees the owner's member row or the profile became non-indexable", who)
		}
	}
}

// The "u" shortcut jumps to the uploader: for a signed-in member on a page without one, that is the dashboard's.
func TestUploadShortcutTargetsTheMembersUploader(t *testing.T) {
	a, h := ssoApp(t)
	u, _ := member(t, a, "shortcut@example.com")
	if body := fetch(h, "/about", sessionFor(t, a, u.ID)).Body.String(); !strings.Contains(body, `<body data-member="1">`) {
		t.Error("the page does not tell the script that a member is signed in")
	}
	if body := fetch(h, "/about").Body.String(); strings.Contains(body, `data-member`) {
		t.Error("a guest's page claims a member is signed in")
	}
}

package app

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"
)

// The member dashboard shows the at-a-glance numbers, the upload box, the
// member menu and a filter by file type.
func TestDashboard(t *testing.T) {
	a, h := ssoApp(t)
	u, key := member(t, a, "dash@example.com")
	upload(t, h, key, 2)

	ctx := context.Background()
	if s := a.db.UserSummary(ctx, u.ID); s.Images != 2 || s.Videos != 0 || s.PDFs != 0 || s.Week != 2 || s.Albums != 1 {
		t.Fatalf("summary: %+v", s)
	}

	tok := randomToken(32)
	a.db.InsertSession(ctx, hashToken(tok), u.ID, time.Hour, "", "")
	cookie := &http.Cookie{Name: sessionCookie, Value: tok}

	rec := fetch(h, "/dashboard", cookie)
	if rec.Code != 200 {
		t.Fatalf("dashboard: %d", rec.Code)
	}
	body := rec.Body.String()
	for _, want := range []string{`id="drop"`, `id="my-gallery"`, `class="member-nav"`, `data-stat="files">2<`, `2 images · 0 videos · 0 PDFs`, `class="dash-start"`, `<b>3/4</b>`} {
		if !strings.Contains(body, want) {
			t.Errorf("dashboard is missing %s", want)
		}
	}

	if body := fetch(h, "/dashboard?kind=video", cookie).Body.String(); !strings.Contains(body, `id="dash-empty"`) {
		t.Error("video filter should show no files")
	}
	if body := fetch(h, "/dashboard?kind=image", cookie).Body.String(); strings.Contains(body, `id="dash-empty"`) {
		t.Error("image filter should show the images")
	}

	// The member menu is on settings too, so every member page links back.
	if body := fetch(h, "/settings", cookie).Body.String(); !strings.Contains(body, `class="member-nav"`) {
		t.Error("settings has no member menu")
	}
}

// A new member sees the getting-started card with nothing done; it names the four steps.
func TestDashboardGettingStarted(t *testing.T) {
	a, h := ssoApp(t)
	u, _ := member(t, a, "fresh@example.com")
	tok := randomToken(32)
	a.db.InsertSession(context.Background(), hashToken(tok), u.ID, time.Hour, "", "")
	body := fetch(h, "/dashboard", &http.Cookie{Name: sessionCookie, Value: tok}).Body.String()
	for _, want := range []string{`class="dash-start"`, `<b>1/4</b>`, "Upload your first file", "Make an album", "Turn on your public profile", "Create an API key for ShareX"} {
		if !strings.Contains(body, want) {
			t.Errorf("getting-started card is missing %q", want)
		}
	}
}

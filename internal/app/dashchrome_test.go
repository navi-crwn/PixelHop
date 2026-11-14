package app

import (
	"regexp"
	"strings"
	"testing"
)

// "Part of Hop Easy Link" is the last line of the dashboard sidebar and appears once on a dashboard
// screen; the public footer keeps its own line.
func TestPartOfLivesInTheSidebarFoot(t *testing.T) {
	a, h := ssoApp(t)
	u, _ := member(t, a, "partof@example.com")
	c := sessionFor(t, a, u.ID)
	for _, path := range []string{"/dashboard", "/dashboard/albums", "/settings", "/tools"} {
		body := fetch(h, path, c).Body.String()
		if n := strings.Count(body, `class="part-of"`); n != 1 {
			t.Errorf("%s shows %d \"Part of\" lines, want 1", path, n)
		}
		side := body[strings.Index(body, `<aside class="shell-side"`):strings.Index(body, `</aside>`)]
		if !strings.Contains(side, `class="part-of"`) {
			t.Errorf("%s: the line is not in the sidebar", path)
		}
		if strings.Index(side, `class="part-of"`) < strings.Index(side, `class="shell-side-foot"`) {
			t.Errorf("%s: the line is above the sidebar's foot links", path)
		}
	}
	if body := fetch(h, "/about").Body.String(); !strings.Contains(body, `class="part-of"`) {
		t.Error("the public footer lost its \"Part of\" line")
	}
}

// The site announcement (setting "announcement", set from the control center) is a dismissible card at
// the top of the dashboard home. It never becomes a bar across the signed-in pages; the public pages keep
// their banner.
func TestAnnouncementCardOnTheDashboard(t *testing.T) {
	a, h := ssoApp(t)
	u, _ := member(t, a, "announce@example.com")
	c := sessionFor(t, a, u.ID)

	if body := fetch(h, "/dashboard", c).Body.String(); strings.Contains(body, `id="announce"`) || strings.Contains(body, `class="announce"`) {
		t.Error("no announcement is set, yet the dashboard shows one")
	}

	if err := a.db.SetSetting(t.Context(), "announcement", "Maintenance on Sunday, 02:00 to 03:00 UTC."); err != nil {
		t.Fatal(err)
	}
	body := fetch(h, "/dashboard", c).Body.String()
	card := strings.Index(body, `id="announce"`)
	if card < 0 {
		t.Fatal("the dashboard does not show the announcement card")
	}
	if card > strings.Index(body, `class="dash-head"`) {
		t.Error("the card is not above the dashboard's first section")
	}
	for _, want := range []string{"Maintenance on Sunday", `id="announce-close"`, `data-id="`} {
		if !strings.Contains(body, want) {
			t.Errorf("the announcement card lacks %s", want)
		}
	}
	if strings.Contains(body, `class="announce"`) {
		t.Error("the dashboard also shows the public banner")
	}
	// the card is only on the dashboard home, not on every signed-in page
	if body := fetch(h, "/settings", c).Body.String(); strings.Contains(body, `id="announce"`) || strings.Contains(body, `class="announce"`) {
		t.Error("Settings shows the announcement")
	}
	// a new text gets a new id, so a closed card comes back
	id := regexp.MustCompile(`id="announce"[^>]*data-id="([^"]+)"|data-id="([^"]+)"[^>]*id="announce"`)
	first := id.FindString(body)
	a.db.SetSetting(t.Context(), "announcement", "A different notice.")
	if second := id.FindString(fetch(h, "/dashboard", c).Body.String()); second == "" || second == first {
		t.Errorf("a new announcement keeps the old card id: %q vs %q", first, second)
	}
	// public pages keep the banner for everyone, signed in or not
	for _, ck := range [][]string{{}, {"c"}} {
		var pub string
		if len(ck) == 0 {
			pub = fetch(h, "/about").Body.String()
		} else {
			pub = fetch(h, "/about", c).Body.String()
		}
		if !strings.Contains(pub, `class="announce"`) {
			t.Error("a public page lost the announcement banner")
		}
	}
}

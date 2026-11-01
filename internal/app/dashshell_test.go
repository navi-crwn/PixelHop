package app

import (
	"strings"
	"testing"
)

// The signed-in dashboard has the same chrome as the other HOP dashboards: this app's menu and the HOP family in
// the sidebar, settings-type controls in the top bar (no second list of apps), and an evenly spread footer.
func TestDashboardShell(t *testing.T) {
	a, h := ssoApp(t)
	user, _ := member(t, a, "mira@example.com")
	cookie := sessionFor(t, a, user.ID)

	for _, path := range []string{"/dashboard", "/dashboard/albums", "/settings"} {
		rec := fetch(h, path, cookie)
		if rec.Code != 200 {
			t.Fatalf("%s: status %d", path, rec.Code)
		}
		body := rec.Body.String()
		for _, want := range []string{`class="shell"`, `class="shell-side"`, `class="shell-nav shell-family"`, `>HOP ID<`, `>LinkHop`, `>BioHop`, `>NoteHop`, `class="shell-top"`, `class="shell-foot"`} {
			if !strings.Contains(body, want) {
				t.Errorf("%s lacks %s", path, want)
			}
		}
		top, _, _ := strings.Cut(strings.SplitN(body, `class="shell-top"`, 2)[1], "</header>")
		if strings.Contains(top, "family-switcher") {
			t.Errorf("%s: the app grid is back in the top bar", path)
		}
		for _, want := range []string{`id="lang-menu"`, `id="theme"`, `id="menu"`} {
			if !strings.Contains(top, want) {
				t.Errorf("%s: top bar lacks %s", path, want)
			}
		}
	}
	// pages for everyone else keep the public header and footer
	body := fetch(h, "/about", cookie).Body.String()
	if strings.Contains(body, `class="shell-side"`) || !strings.Contains(body, `class="footer-columns"`) {
		t.Error("public pages must keep the public chrome")
	}
	if guest := fetch(h, "/").Body.String(); strings.Contains(guest, `class="shell"`) {
		t.Error("a guest never gets the dashboard shell")
	}
}

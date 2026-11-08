package app

import (
	"strings"
	"testing"
)

// The trash was only in the member tabs, which the dashboard shell hides, so from the dashboard, the albums
// and the settings there was no way to reach it (dashboard test 2026-11-08, round 3). It is in the app menu
// of the sidebar now, marked as the current page when open.
func TestTrashIsInTheDashboardSidebar(t *testing.T) {
	a, h := ssoApp(t)
	user, _ := member(t, a, "bin@example.com")
	cookie := sessionFor(t, a, user.ID)
	for _, path := range []string{"/dashboard", "/dashboard/albums", "/settings", "/dashboard/trash"} {
		body := fetch(h, path, cookie).Body.String()
		side, _, _ := strings.Cut(strings.SplitN(body, `class="shell-nav"`, 2)[1], "</nav>")
		want := `href="/dashboard/trash" >`
		if path == "/dashboard/trash" {
			want = `href="/dashboard/trash" aria-current="page">`
		}
		if !strings.Contains(side, want) {
			t.Errorf("%s: sidebar lacks %s", path, want)
		}
	}
}

package app

import (
	"regexp"
	"strings"
	"testing"
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

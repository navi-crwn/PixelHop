package app

import (
	"strings"
	"testing"
)

// Settings is a subtab page: each card group is its own tab and every deep link the app hands out
// (#profile, #sessions, #api, #webhooks) still names a card on the page.
func TestSettingsSubtabs(t *testing.T) {
	a, h := ssoApp(t)
	u, _ := member(t, a, "tabs@example.com")
	body := fetch(h, "/settings", sessionFor(t, a, u.ID)).Body.String()
	for _, want := range []string{`id="set-tabs"`, `data-settab="profile"`, `data-settab="sessions"`, `data-settab="api"`, `data-settab="data"`, `data-settab="delete"`,
		`id="profile"`, `id="sessions"`, `id="api"`, `id="webhooks"`, `data-setpanel="api"`} {
		if !strings.Contains(body, want) {
			t.Errorf("settings is missing %s", want)
		}
	}
	// The dashboard's shortcuts still point at the tab's address.
	if body := fetch(h, "/dashboard", sessionFor(t, a, u.ID)).Body.String(); !strings.Contains(body, `href="/settings#api"`) {
		t.Error("the dashboard no longer links the API tab")
	}
}

// "Upload more" on the result page brings a member back to the dashboard uploader; guests go to the
// home uploader as before.
func TestUploadMoreLink(t *testing.T) {
	a, h := ssoApp(t)
	u, key := member(t, a, "more@example.com")
	ids := upload(t, h, key, 1)
	body := fetch(h, "/uploaded?id="+strings.Join(ids, ","), sessionFor(t, a, u.ID)).Body.String()
	if !strings.Contains(body, `href="/dashboard#upload"`) {
		t.Error("a member's Upload more does not lead to the dashboard uploader")
	}
	if body := fetch(h, "/uploaded?id="+strings.Join(ids, ",")).Body.String(); !strings.Contains(body, `href="/"`) {
		t.Error("a guest's Upload more no longer leads home")
	}
}

// The top bar carries the family's primary action on every signed-in page, the dashboard included
// (there it jumps to the uploader on the page).
func TestTopBarUploadOnEveryMemberPage(t *testing.T) {
	a, h := ssoApp(t)
	u, _ := member(t, a, "topbar@example.com")
	c := sessionFor(t, a, u.ID)
	for path, href := range map[string]string{"/dashboard": `href="#upload"`, "/settings": `href="/dashboard#upload"`, "/dashboard/albums": `href="/dashboard#upload"`, "/dashboard/trash": `href="/dashboard#upload"`} {
		body := fetch(h, path, c).Body.String()
		i := strings.Index(body, `class="shell-tools"`)
		if i < 0 {
			t.Errorf("%s: no top bar", path)
			continue
		}
		if bar := body[i:min(len(body), i+400)]; !strings.Contains(bar, `btn primary sm" `+href) {
			t.Errorf("%s: the top bar has no Upload button with %s", path, href)
		}
	}
	if body := fetch(h, "/dashboard", c).Body.String(); !strings.Contains(body, `id="upload"`) {
		t.Error(`the dashboard has no id="upload" for the top bar button to jump to`)
	}
}

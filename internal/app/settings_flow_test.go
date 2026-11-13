package app

import (
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/navi-crwn/pixelhop/internal/db"
)

// panelOf returns the name of the Settings panel that holds the first occurrence of needle.
func panelOf(t *testing.T, body, needle string) string {
	t.Helper()
	i := strings.Index(body, needle)
	if i < 0 {
		t.Fatalf("settings page has no %s", needle)
	}
	marks := regexp.MustCompile(`data-setpanel="([a-z]+)"`).FindAllStringSubmatchIndex(body[:i], -1)
	if len(marks) == 0 {
		t.Fatalf("%s is outside every panel", needle)
	}
	m := marks[len(marks)-1]
	return body[m[2]:m[3]]
}

// startOf reads the tab the server asks the page to open first ("" = the first tab).
func startOf(body string) string {
	m := regexp.MustCompile(`id="set-tabs"[^>]*data-start="([a-z]*)"`).FindStringSubmatch(body)
	if m == nil {
		return ""
	}
	return m[1]
}

func settingsMember(t *testing.T, a *App, email string) (*db.User, *http.Cookie) {
	t.Helper()
	u, _ := member(t, a, email)
	u.PassHash = HashPassword("correct horse")
	if err := a.db.UpdateUser(t.Context(), u); err != nil {
		t.Fatal(err)
	}
	return u, sessionFor(t, a, u.ID)
}

// A page that answers a form (instead of redirecting) must open the tab where that form lives:
// the one-time API key, the webhook secret and every error message are on that page only.
func TestSettingsAnswerOpensTheFormsTab(t *testing.T) {
	a, h := ssoApp(t)
	_, c := settingsMember(t, a, "start@example.com")

	if s := startOf(fetch(h, "/settings", c).Body.String()); s != "" {
		t.Errorf("a plain visit starts on %q, want the first tab", s)
	}

	body := post(h, "/settings/keys", url.Values{"name": {"CI"}}, c).Body.String()
	if !strings.Contains(body, `id="new-key"`) {
		t.Fatal("creating a key does not show the key")
	}
	if got, in := startOf(body), panelOf(t, body, `id="new-key"`); got != in {
		t.Errorf("new key: the page starts on tab %q but the key is in tab %q", got, in)
	}

	body = post(h, "/settings/webhooks", url.Values{"url": {"https://example.com/hook"}}, c).Body.String()
	if !strings.Contains(body, `id="wh-secret"`) {
		t.Fatal("creating a webhook does not show the secret")
	}
	if got, in := startOf(body), panelOf(t, body, `id="wh-secret"`); got != in {
		t.Errorf("new webhook secret: starts on tab %q, secret is in tab %q", got, in)
	}

	rec := post(h, "/settings/webhooks", url.Values{"url": {"http://127.0.0.1/hook"}}, c)
	body = rec.Body.String()
	if rec.Code != http.StatusBadRequest || startOf(body) != panelOf(t, body, `action="/settings/webhooks"`) {
		t.Errorf("bad webhook URL: %d, starts on %q, form is in %q", rec.Code, startOf(body), panelOf(t, body, `action="/settings/webhooks"`))
	}

	rec = post(h, "/settings/password", url.Values{"current": {"wrong"}, "password": {"another long one"}}, c)
	body = rec.Body.String()
	if rec.Code != http.StatusBadRequest || startOf(body) != panelOf(t, body, `action="/settings/password"`) {
		t.Errorf("wrong password: %d, starts on %q, form is in %q", rec.Code, startOf(body), panelOf(t, body, `action="/settings/password"`))
	}

	rec = post(h, "/settings/delete-account", url.Values{"password": {"wrong"}}, c)
	body = rec.Body.String()
	if rec.Code != http.StatusBadRequest || startOf(body) != panelOf(t, body, `action="/settings/delete-account"`) {
		t.Errorf("wrong delete password: starts on %q, form is in %q", startOf(body), panelOf(t, body, `action="/settings/delete-account"`))
	}

	rec = post(h, "/settings/profile", url.Values{"username": {"!!"}}, c)
	if rec.Code != http.StatusBadRequest || startOf(rec.Body.String()) != "" {
		t.Errorf("profile error: %d starting on %q, want the first tab", rec.Code, startOf(rec.Body.String()))
	}
}

// Testing a webhook when none exists, and going past 20 keys, are answered pages too.
func TestSettingsOtherAnswersOpenTheAPITab(t *testing.T) {
	a, h := ssoApp(t)
	u, c := settingsMember(t, a, "limit@example.com")

	rec := post(h, "/settings/webhooks/test", url.Values{}, c)
	if body := rec.Body.String(); rec.Code != http.StatusBadRequest || startOf(body) != panelOf(t, body, `action="/settings/webhooks"`) {
		t.Errorf("webhook test with none added: %d, starts on %q", rec.Code, startOf(body))
	}

	for i := 0; i < 19; i++ { // member() already made one key
		k := "ph_" + randomToken(24)
		if err := a.db.InsertAPIKey(t.Context(), u.ID, "k", k[:9], hashToken(k)); err != nil {
			t.Fatal(err)
		}
	}
	body := post(h, "/settings/keys", url.Values{}, c).Body.String()
	if !strings.Contains(body, `class="alert err"`) {
		t.Fatal("the 21st key is not refused")
	}
	if got, in := startOf(body), panelOf(t, body, `action="/settings/keys"`); got != in {
		t.Errorf("key limit: starts on tab %q, the key form is in tab %q", got, in)
	}
}

// After a save that redirects, the address must name a card in the tab that holds the form that was
// pressed; otherwise the member lands on another tab and sees only a notice about something unseen.
func TestSettingsRedirectsLandOnTheFormsTab(t *testing.T) {
	a, h := ssoApp(t)
	u, c := settingsMember(t, a, "redir@example.com")
	hook := &db.Webhook{UserID: u.ID, URL: "https://example.com/h", Secret: "whsec_x", Events: "image.uploaded"}
	if err := a.db.InsertWebhook(t.Context(), hook); err != nil {
		t.Fatal(err)
	}
	keys, _ := a.db.APIKeys(t.Context(), u.ID)
	keyDel := "/settings/keys/" + strconv.FormatInt(keys[0].ID, 10) + "/delete"
	hookDel := "/settings/webhooks/" + strconv.FormatInt(hook.ID, 10) + "/delete"
	page := fetch(h, "/settings", c).Body.String()

	for _, tc := range []struct {
		action string
		form   url.Values
	}{
		{"/settings/profile", url.Values{"username": {"redirtester"}}},
		{"/settings/hotlinks", url.Values{"block_hotlinks": {"on"}}},
		{"/settings/sessions/revoke", url.Values{}},
		{keyDel, url.Values{}},
		{hookDel, url.Values{}},
		{"/settings/password", url.Values{"current": {"correct horse"}, "password": {"another long one"}}},
	} {
		rec := post(h, tc.action, tc.form, c)
		if rec.Code != http.StatusSeeOther {
			t.Errorf("%s: %d, want 303", tc.action, rec.Code)
			continue
		}
		loc := rec.Header().Get("Location")
		i := strings.Index(loc, "#")
		if i < 0 {
			t.Errorf("%s: redirects to %q, which names no card", tc.action, loc)
			continue
		}
		if landing, formTab := panelOf(t, page, `id="`+loc[i+1:]+`"`), panelOf(t, page, `action="`+tc.action+`"`); landing != formTab {
			t.Errorf("%s: redirects to %q (tab %q) but the form is in tab %q", tc.action, loc, landing, formTab)
		}
	}
}

// Every "/settings#card" link in the nine languages names a card that exists, so Help and the
// dashboard land on the tab that shows it.
func TestSettingsLinksInAllLanguagesLand(t *testing.T) {
	a, h := ssoApp(t)
	_, c := settingsMember(t, a, "links@example.com")
	page := fetch(h, "/settings", c).Body.String()
	files, _ := filepath.Glob("../../web/i18n/*.json")
	if len(files) != 9 {
		t.Fatalf("found %d language files, want 9", len(files))
	}
	re := regexp.MustCompile(`/settings#([a-z-]+)`)
	seen := map[string]bool{}
	for _, f := range files {
		raw, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		for _, m := range re.FindAllStringSubmatch(string(raw), -1) {
			seen[m[1]] = true
			if !strings.Contains(page, `id="`+m[1]+`"`) {
				t.Errorf("%s links /settings#%s but the page has no such card", filepath.Base(f), m[1])
			}
		}
	}
	for _, want := range []string{"export", "api", "embedding"} {
		if !seen[want] {
			t.Errorf("no language links /settings#%s any more; this check no longer covers it", want)
		}
	}
	if panelOf(t, page, `id="export"`) != "data" {
		t.Error("the export card is not in the Data tab")
	}
}

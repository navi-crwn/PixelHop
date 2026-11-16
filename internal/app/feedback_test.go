package app

import (
	"context"
	"html"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"testing"

	"github.com/navi-crwn/pixelhop/internal/config"
	"github.com/navi-crwn/pixelhop/internal/db"
)

// feedbackTestApp builds the app with the given hub address (HELINK_URL); "" is a self-hosted install without one.
func feedbackTestApp(t *testing.T, hub string) (*App, http.Handler) {
	t.Helper()
	t.Setenv("PIXELHOP_DATA", t.TempDir())
	t.Setenv("PIXELHOP_BASE_URL", "https://p.example.com")
	t.Setenv("HELINK_URL", hub)
	if hub != "" {
		t.Setenv("HELINK_SSO_SECRET", testSecret)
	}
	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	d, err := db.Open(cfg.DataDir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { d.Close() })
	a, err := New(cfg, d, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	return a, a.Handler()
}

func TestFeedbackURLContract(t *testing.T) {
	cases := []struct{ hub, screen, ver, want string }{
		{"https://hub.hel.ink", "home", "3.7.0", "https://hub.hel.ink/feedback?app=pixelhop&from=home&v=3.7.0"},
		{"https://hub.hel.ink/", "dashboard", "dev", "https://hub.hel.ink/feedback?app=pixelhop&from=dashboard&v=dev"},
		{"https://hub.hel.ink", "home", "", "https://hub.hel.ink/feedback?app=pixelhop&from=home"},
		{"https://hub.hel.ink", "a b?x=1#y", "1.0 beta<>", "https://hub.hel.ink/feedback?app=pixelhop&from=abx1y&v=1.0beta"},
		{"", "home", "1", ""},
		{"not a url", "home", "1", ""},
		{"javascript:alert(1)", "home", "1", ""},
	}
	for _, c := range cases {
		if got := feedbackURL(c.hub, c.screen, c.ver); got != c.want {
			t.Errorf("feedbackURL(%q,%q,%q) = %q, want %q", c.hub, c.screen, c.ver, got, c.want)
		}
	}
	long := feedbackURL("https://h.example", strings.Repeat("a", 200), strings.Repeat("1", 99))
	if !strings.Contains(long, "from="+strings.Repeat("a", 80)+"&") || !strings.HasSuffix(long, "v="+strings.Repeat("1", 24)) {
		t.Errorf("long values are not cut to 80 and 24: %s", long)
	}
}

var feedbackLinkRe = regexp.MustCompile(`<a href="([^"]*/feedback\?[^"]*)" target="_blank" rel="noopener">`)

func feedbackLinks(body string) []string {
	var out []string
	for _, m := range feedbackLinkRe.FindAllStringSubmatch(body, -1) {
		u, err := url.Parse(html.UnescapeString(m[1]))
		if err != nil {
			continue
		}
		q := u.Query() // the hub reads the decoded values
		out = append(out, u.Scheme+"://"+u.Host+u.Path+"?app="+q.Get("app")+"&from="+q.Get("from")+"&v="+q.Get("v"))
	}
	return out
}

func feedbackWant(screen string) string {
	return "https://hel.example.com/feedback?app=pixelhop&from=" + screen + "&v=" + Version
}

func optCookie(c *http.Cookie) []*http.Cookie {
	if c == nil {
		return nil
	}
	return []*http.Cookie{c}
}

// Every page a visitor can land on carries exactly one Feedback link, named by a fixed screen, never by the
// address that was asked for (an image or album address works as a key).
func TestFeedbackLinkEveryWay(t *testing.T) {
	a, h := feedbackTestApp(t, "https://hel.example.com")
	u, key := member(t, a, "fb@example.com")
	c := sessionFor(t, a, u.ID)
	img := upload(t, h, key, 1)[0]
	if res := batch(t, h, "", key, 1, map[string]string{"album": "force", "title": "Secret trip"}); res.code != 200 || res.images != 1 {
		t.Fatalf("album upload: %d, %d images, %q", res.code, res.images, res.err)
	}
	rows, err := a.db.Albums(context.Background(), u.ID)
	if err != nil || len(rows) != 1 {
		t.Fatalf("albums: %v, %d", err, len(rows))
	}
	al := struct{ ID string }{rows[0].ID}
	for _, p := range []struct {
		path, screen string
		cookie       *http.Cookie
	}{
		{"/", "home", nil}, {"/help", "help", nil}, {"/contact", "contact", nil}, {"/legal", "legal", nil},
		{"/about", "about", nil}, {"/faq", "faq", nil}, {"/changelog", "changelog", nil}, {"/docs", "docs", nil},
		{"/login", "login", nil}, {"/nothing-here/abc123", "other", nil},
		{"/?lang=ru&q=secret", "home", nil},
		{"/" + img, "/{id}", nil}, {"/a/" + al.ID, "/a/{id}", nil}, {"/" + img, "/{id}", c},
		{"/dashboard", "dashboard", c}, {"/dashboard/albums", "dashboard.albums", c}, {"/settings", "settings", c},
		{"/tools", "tools", c},
	} {
		rec := fetch(h, p.path, optCookie(p.cookie)...)
		if rec.Code >= 500 {
			t.Fatalf("%s: %d", p.path, rec.Code)
		}
		links := feedbackLinks(rec.Body.String())
		n := 1
		if p.screen == "help" || p.screen == "contact" {
			n = 2 // the footer, and the "Send feedback" link on the page itself
		}
		if len(links) != n {
			t.Errorf("%s: %d Feedback links %v, want %d", p.path, len(links), links, n)
		}
		for _, l := range links {
			if l != feedbackWant(p.screen) {
				t.Errorf("%s: Feedback link %s, want %s", p.path, l, feedbackWant(p.screen))
			}
		}
		for _, secret := range []string{img, al.ID, "secret", "abc123"} {
			if strings.Contains(strings.Join(links, " "), secret) {
				t.Errorf("%s: %q reached the Feedback link", p.path, secret)
			}
		}
	}
	for _, path := range []string{"/contact", "/help"} {
		if b := fetch(h, path).Body.String(); !strings.Contains(b, ">Send feedback</a>") {
			t.Errorf("%s lacks the Send feedback link", path)
		}
	}
	// The Privacy page has the sentence with a link to the hub's privacy page, not to hel.ink by name
	legal := fetch(h, "/legal").Body.String()
	if !regexp.MustCompile(`<li id="feedbackNote">[^<]+<a href="https://hel\.example\.com/privacy" rel="noopener">[^<]+</a>[^<]*</li>`).MatchString(legal) {
		t.Error("no Feedback sentence with a link to the hub's privacy page on /legal")
	}
	if strings.Contains(legal[strings.Index(legal, `id="feedbackNote"`):strings.Index(legal, `id="takedown"`)], "hel.ink") {
		t.Error("the Privacy sentence names hel.ink instead of the hub this app is connected to")
	}
	// the words exist in every language (no raw key, no empty text)
	for _, l := range []string{"en", "zh-CN", "de", "id", "ja", "pt-BR", "es", "fr", "ru"} {
		for _, path := range []string{"/?lang=" + l, "/contact?lang=" + l, "/legal?lang=" + l} {
			b := fetch(h, path).Body.String()
			if strings.Contains(b, "footer.feedback") || strings.Contains(b, "contact.feedback") || strings.Contains(b, "privacy.feedback") {
				t.Errorf("%s: a raw Feedback key shows", path)
			}
			if m := regexp.MustCompile(`target="_blank" rel="noopener">([^<]*)</a>`).FindStringSubmatch(b); m == nil || strings.TrimSpace(m[1]) == "" {
				t.Errorf("%s: Feedback link text %q", path, m)
			}
		}
	}
}

// A self-hosted install without a hub address shows no Feedback anywhere.
func TestNoFeedbackWithoutHubAddress(t *testing.T) {
	a, h := feedbackTestApp(t, "")
	u, _ := member(t, a, "nofb@example.com")
	c := sessionFor(t, a, u.ID)
	for _, path := range []string{"/", "/help", "/contact", "/legal", "/about", "/nothing-here", "/?lang=ru", "/dashboard", "/settings"} {
		b := fetch(h, path, c).Body.String()
		if strings.Contains(b, "/feedback") || strings.Contains(b, "feedbackNote") || strings.Contains(b, "Send feedback") {
			t.Errorf("%s: a Feedback link or sentence without a hub address", path)
		}
	}
	body := fetch(h, "/dashboard", c).Body.String()
	foot := body[strings.Index(body, `<nav class="shell-side-foot"`):]
	if strings.Count(foot[:strings.Index(foot, "</nav>")], "<li>") != 6 {
		t.Error("without a hub address the dashboard foot is not six items")
	}
}

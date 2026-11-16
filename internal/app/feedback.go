package app

import (
	"html"
	"html/template"
	"net/url"
	"regexp"
	"strings"

	"github.com/navi-crwn/pixelhop/internal/i18n"
)

/*
The Feedback link opens the HOP ID form in a new tab and does nothing else: this app has no form, no upload and
no route that receives anything. The address is {hub}/feedback?app=pixelhop&from=<screen>&v=<version>. The hub
address is HELINK_URL, the one the family announcement already uses; without it there is no link and no Privacy
sentence, and the dashboard foot keeps its six items.
*/

const feedbackApp = "pixelhop"

var (
	feedbackFromBad = regexp.MustCompile(`[^A-Za-z0-9._:/{}-]`)
	feedbackVerBad  = regexp.MustCompile(`[^A-Za-z0-9.+-]`)
)

// feedbackScreens names the screen each page template shows. An image, an album, a profile and a delete page
// have an address that works as a key, so they are named by route template and never by the address asked for.
// Any template not listed here is sent under the name "other".
var feedbackScreens = map[string]string{
	"home": "home", "about": "about", "features": "features", "faq": "faq", "help": "help", "contact": "contact",
	"docs": "docs", "legal": "legal", "status": "status", "changelog": "changelog", "login": "login",
	"register": "register", "tools": "tools", "batch": "batch", "dashboard": "dashboard", "albums": "dashboard.albums",
	"trash": "dashboard.trash", "settings": "settings", "album_edit": "/a/{id}/edit",
	"view": "/{id}", "album": "/a/{id}", "profile": "/u/{username}", "delete": "/delete/{id}/{token}",
	"deleted": "/delete/{id}/{token}", "locked": "/{id}", "uploaded": "uploaded", "error": "other",
}

func feedbackScreen(template string) string {
	if s, ok := feedbackScreens[template]; ok {
		return s
	}
	return "other"
}

// feedbackHub is the hub address the link and the Privacy sentence use: an http(s) address without a trailing
// slash, or "" (no link, no sentence).
func feedbackHub(raw string) string {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Host == "" {
		return ""
	}
	return u.Scheme + "://" + u.Host + strings.TrimRight(u.Path, "/")
}

// feedbackURL is the link for one screen, or "" without a hub address.
func feedbackURL(hub, screen, version string) string {
	base := feedbackHub(hub)
	if base == "" {
		return ""
	}
	q := url.Values{"app": {feedbackApp}}
	if from := feedbackFromBad.ReplaceAllString(screen, ""); from != "" {
		q.Set("from", feedbackTrunc(from, 80))
	}
	if v := feedbackVerBad.ReplaceAllString(version, ""); v != "" {
		q.Set("v", feedbackTrunc(v, 24))
	}
	return base + "/feedback?" + q.Encode()
}

func feedbackTrunc(s string, n int) string {
	if len(s) > n {
		return s[:n]
	}
	return s
}

// feedbackNote is the Privacy sentence with its link to the hub's privacy page; empty without a hub address.
func feedbackNote(l i18n.Locale, hub string) template.HTML {
	base := feedbackHub(hub)
	if base == "" {
		return ""
	}
	link := `<a href="` + html.EscapeString(base+"/privacy") + `" rel="noopener">` + html.EscapeString(l.T("privacy.feedback_link")) + `</a>`
	return template.HTML(l.T("privacy.feedback", "link", link))
}

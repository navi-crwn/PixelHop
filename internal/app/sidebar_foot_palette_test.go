package app

import (
	"io/fs"
	"regexp"
	"strings"
	"testing"

	"github.com/navi-crwn/pixelhop/web"
)

// The sidebar foot is one list going down in a fixed order, then "Part of Hop Easy Link"; above it the family
// colour picker. The palette only applies on the dashboard shell, and classic is "no attribute".
func TestSidebarFootAndPalettePicker(t *testing.T) {
	a, h := ssoApp(t)
	u, _ := member(t, a, "foot@example.com")
	c := sessionFor(t, a, u.ID)
	body := fetch(h, "/dashboard", c).Body.String()
	side := body[strings.Index(body, `<aside class="shell-side"`):strings.Index(body, `</aside>`)]
	foot := side[strings.Index(side, `<nav class="shell-side-foot"`):]
	hrefs := regexp.MustCompile(`href="([^"]+)"`).FindAllStringSubmatch(foot, -1)
	var got []string
	for _, m := range hrefs {
		got = append(got, m[1])
	}
	want := []string{"/help", "/legal#terms", "/legal#privacy", "/changelog", "https://github.com/navi-crwn/pixelhop", "https://hel.ink/announcements", "https://hel.ink"}
	if strings.Join(got, " ") != strings.Join(want, " ") {
		t.Fatalf("foot links %v, want %v", got, want)
	}
	if strings.Count(foot, "<li>") != 6 {
		t.Error("the foot is not six list items")
	}
	if strings.Index(side, `class="shell-pal"`) > strings.Index(side, `class="shell-side-foot"`) || strings.Count(side, `data-palette=`) != 3 {
		t.Error("the colour picker is missing or not above the foot")
	}
	if !strings.Contains(body, "<html") || !regexp.MustCompile(`<html[^>]* data-shell`).MatchString(body) {
		t.Error("the dashboard shell does not mark itself, so the palette cannot apply")
	}
	if public := fetch(h, "/about").Body.String(); regexp.MustCompile(`<html[^>]* data-shell`).MatchString(public) || strings.Contains(public, `data-palette=`) {
		t.Error("a public page carries the dashboard marker or picker")
	}
	js, _ := fs.ReadFile(web.Static, "static/js/theme.js")
	for _, w := range []string{"hop_palette", "hasAttribute('data-shell')", "p === 'mist' || p === 'sand'"} {
		if !strings.Contains(string(js), w) {
			t.Errorf("theme.js lacks %s", w)
		}
	}
	css, _ := fs.ReadFile(web.Static, "static/css/app.css")
	for _, w := range []string{`:root[data-palette="mist"]`, `:root[data-palette="sand"]`, "#f4f6fc", "#0d1120", "#faf6f1", "#17110d"} {
		if !strings.Contains(string(css), w) {
			t.Errorf("app.css lacks the frozen value %s", w)
		}
	}
}

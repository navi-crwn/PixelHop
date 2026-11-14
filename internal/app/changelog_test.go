package app

import (
	"net/http/httptest"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestChangelogHistoryAndPagination(t *testing.T) {
	history := changes()
	if len(history.Entries) < 20 || len(history.Sources) != 1 {
		t.Fatal("expected the app's whole release history")
	}
	seen := map[string]bool{}
	previous := time.Now().AddDate(1, 0, 0)
	for _, entry := range history.Entries {
		date, err := time.Parse(time.RFC3339, entry.Date)
		if err != nil || date.After(previous) {
			t.Fatalf("invalid history ordering: %+v", entry)
		}
		previous = date
		if seen[entry.URL] || entry.Repo != "PixelHop" || !strings.HasPrefix(entry.URL, "https://github.com/navi-crwn/PixelHop/") {
			t.Fatalf("duplicate or wrong source: %+v", entry)
		}
		seen[entry.URL] = true
		// every release is written in plain words: a title, a version, and items that each say where and what
		if entry.Title == "" || entry.Version == "" || len(entry.Items) == 0 {
			t.Fatalf("a release without plain text: %+v", entry)
		}
		for _, it := range entry.Items {
			if it.Type == "" || it.Where == "" || it.Text == "" {
				t.Fatalf("an item without its place or text in %s: %+v", entry.Version, it)
			}
		}
	}
	var visited int
	for path := "/changelog?lang=id"; path != ""; {
		p := historyPage(httptest.NewRequest("GET", path, nil))
		visited += len(p.Entries)
		if len(p.Entries) > 40 || p.Total != len(history.Entries) {
			t.Fatal("invalid page size or total")
		}
		if p.Next != "" {
			u, _ := url.Parse(p.Next)
			if u.Query().Get("lang") != "id" || u.Query().Get("page") != strconv.Itoa(p.Page+1) {
				t.Fatal("pagination lost locale or next page")
			}
		}
		path = p.Next
	}
	if visited != len(history.Entries) {
		t.Fatal("pagination omits history")
	}
	for _, value := range []string{"-1", "bad", "0", "999999"} {
		p := historyPage(httptest.NewRequest("GET", "/changelog?page="+value, nil))
		if p.Page < 1 || p.Page > p.Pages || len(p.Entries) == 0 {
			t.Fatalf("invalid page %q was not clamped", value)
		}
	}
}

func TestChangelogRoute(t *testing.T) {
	h := newTestApp(t)
	for _, path := range []string{"/changelog?lang=id", "/changelog?page=2"} { // page 2 is clamped to the last page
		r := httptest.NewRecorder()
		h.ServeHTTP(r, httptest.NewRequest("GET", path, nil))
		if r.Code != 200 || !strings.Contains(r.Body.String(), "github.com/navi-crwn/PixelHop/") {
			t.Fatalf("changelog %s: status %d", path, r.Code)
		}
		if strings.Contains(path, "lang=id") && !strings.Contains(r.Body.String(), "Riwayat perubahan") {
			t.Fatal("changelog locale missing")
		}
	}
	r := httptest.NewRecorder()
	h.ServeHTTP(r, httptest.NewRequest("GET", "/changelog.php", nil))
	if r.Header().Get("Location") != "/changelog" {
		t.Fatal("legacy changelog must redirect to local history")
	}
}

// The page shows releases in plain language, with its labels in the reader's language, and never a branch name,
// a merge line or the word "train".
func TestChangelogPageIsPlain(t *testing.T) {
	h := newTestApp(t)
	bad := regexp.MustCompile(`(?i)trains?|kereta|merge pull request|remote-tracking|(^|[^w/])(fix|feat|docs|chore|release|hotfix|refactor)/[w.-]+`)
	for lang, label := range map[string]string{"en": "Technical details", "de": "Technische Details", "ru": "Технические подробности"} {
		r := httptest.NewRecorder()
		h.ServeHTTP(r, httptest.NewRequest("GET", "/changelog?lang="+lang, nil))
		page := r.Body.String()
		if !strings.Contains(page, label) || !strings.Contains(page, "Dashboard polish") || !strings.Contains(page, "<b>Dashboard.</b>") {
			t.Errorf("%s: the page lacks the release text or its labels", lang)
		}
		if m := bad.FindString(page); m != "" {
			t.Errorf("%s: the page shows %q", lang, m)
		}
	}
	// an old snapshot (no items) still renders as a title
	e := changeEntry{Kind: "pr", Date: "2026-01-01T00:00:00Z", Title: "Old", URL: "https://example.test/x", ID: "#1"}
	if len(e.Groups()) != 0 {
		t.Fatal("an entry without items has groups")
	}
}

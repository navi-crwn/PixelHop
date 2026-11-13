package app

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strings"
	"testing"
)

var (
	refAttr = regexp.MustCompile(`(?i)\b(?:href|src|action)\s*=\s*"([^"]*)"`)
	idAttr  = regexp.MustCompile(`(?i)\bid\s*=\s*"([^"]+)"`)
)

// TestPublicPagesHaveNoDeadInternalLinks follows every internal href/src/action
// of the guest pages, in all nine languages, until the set is closed: no 4xx or
// 5xx, no redirect chain longer than two, and every #fragment exists.
func TestPublicPagesHaveNoDeadInternalLinks(t *testing.T) {
	h := newTestApp(t)
	get := func(path, lang string) (*httptest.ResponseRecorder, int) {
		hops := 0
		for {
			req := httptest.NewRequest("GET", path, nil)
			req.AddCookie(&http.Cookie{Name: langCookie, Value: lang})
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)
			if rec.Code >= 300 && rec.Code < 400 && hops < 5 {
				loc := rec.Header().Get("Location")
				if !strings.HasPrefix(loc, "/") || strings.HasPrefix(loc, "//") {
					return rec, hops // external or odd redirect: the caller only checks the status
				}
				path, hops = loc, hops+1
				continue
			}
			return rec, hops
		}
	}
	for _, lang := range []string{"en", "zh-CN", "de", "id", "ja", "pt-BR", "es", "fr", "ru"} {
		seen := map[string]bool{}
		queue := []string{"/", "/tools", "/docs", "/help", "/legal", "/about", "/changelog", "/features", "/faq", "/contact", "/status", "/login", "/register"}
		for len(queue) > 0 {
			page := queue[0]
			queue = queue[1:]
			if seen[page] {
				continue
			}
			seen[page] = true
			rec, hops := get(page, lang)
			if rec.Code >= 400 {
				t.Errorf("[%s] %s answers %d", lang, page, rec.Code)
				continue
			}
			if hops > 2 {
				t.Errorf("[%s] %s needs %d redirects", lang, page, hops)
			}
			if !strings.Contains(rec.Header().Get("Content-Type"), "text/html") {
				continue
			}
			body := rec.Body.String()
			ids := map[string]bool{}
			for _, m := range idAttr.FindAllStringSubmatch(body, -1) {
				ids[m[1]] = true
			}
			for _, m := range refAttr.FindAllStringSubmatch(body, -1) {
				ref := strings.TrimSpace(m[1])
				if ref == "" || strings.ContainsAny(ref, "{}") || strings.HasPrefix(ref, "mailto:") || strings.HasPrefix(ref, "data:") || strings.HasPrefix(ref, "blob:") {
					continue
				}
				u, err := url.Parse(ref)
				if err != nil || u.Host != "" || u.Scheme != "" {
					continue // external
				}
				path := u.Path
				if path == "" { // "?lang=de" or "#fragment": same page
					path = strings.SplitN(page, "?", 2)[0]
				}
				if u.Fragment != "" && path == strings.SplitN(page, "?", 2)[0] && !ids[u.Fragment] {
					t.Errorf("[%s] %s: #%s does not exist on the page", lang, page, u.Fragment)
				}
				if strings.HasPrefix(path, "//") || strings.Contains(path, "/../") {
					t.Errorf("[%s] %s: odd path %q", lang, page, path)
				}
				if path == "/logout" || strings.HasPrefix(path, "/auth/") || strings.HasPrefix(path, "/delete/") || strings.HasPrefix(path, "/api/") {
					continue // POST-only, redirects to another host, or needs a token
				}
				q := ""
				if u.RawQuery != "" && !strings.HasPrefix(u.RawQuery, "lang=") {
					q = "?" + u.RawQuery
				}
				if !seen[path+q] && path != "" {
					queue = append(queue, path+q)
				}
			}
		}
	}
}

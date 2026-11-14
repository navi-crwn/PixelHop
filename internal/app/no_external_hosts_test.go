package app

import (
	"io/fs"
	"regexp"
	"strings"
	"testing"

	"github.com/navi-crwn/pixelhop/web"
)

// Nothing the browser runs comes from a CDN or another third party: the page scripts and styles of this
// program name no such host, and the Content Security Policy does not allow one.
func TestNoThirdPartyHostsInWebAndCSP(t *testing.T) {
	banned := regexp.MustCompile(`jsdelivr|unpkg\.com|cdnjs|staticimgly|projectnaptha|googleapis|gstatic|cloudflareinsights`)
	walked := 0
	for root, fsys := range map[string]fs.FS{"static/js": web.Static, "static/css": web.Static, "templates": web.Templates} {
		err := fs.WalkDir(fsys, root, func(p string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return nil
			}
			walked++
			b, _ := fs.ReadFile(fsys, p)
			if m := banned.Find(b); m != nil {
				t.Errorf("%s names %q", p, m)
			}
			return nil
		})
		if err != nil {
			t.Fatalf("walk %s: %v", root, err)
		}
	}
	if walked < 30 {
		t.Fatalf("only %d files checked", walked)
	}
	a, h := ssoApp(t)
	u, _ := member(t, a, "csp@example.com")
	c := sessionFor(t, a, u.ID)
	for _, path := range []string{"/tools", "/tools/batch", "/dashboard", "/"} {
		csp := fetch(h, path, c).Header().Get("Content-Security-Policy")
		for _, bad := range []string{"jsdelivr", "staticimgly", "projectnaptha"} {
			if strings.Contains(csp, bad) {
				t.Errorf("%s: CSP allows %s", path, bad)
			}
		}
		if strings.Contains(strings.ReplaceAll(csp, "'wasm-unsafe-eval'", ""), "unsafe-eval") {
			t.Errorf("%s: CSP still allows unsafe-eval: %s", path, csp)
		}
	}
	js, _ := fs.ReadFile(web.Static, "static/js/studio.js")
	for _, gone := range []string{"imgly", "REMBG", "rembgWhere", "On this device"} {
		if strings.Contains(string(js), gone) {
			t.Errorf("studio.js still has %s", gone)
		}
	}
}

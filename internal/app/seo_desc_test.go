package app

import (
	"html"
	"regexp"
	"testing"
	"unicode/utf8"
)

var descRe = regexp.MustCompile(`<meta name="description" content="([^"]*)">`)

// Every page in the sitemap has a description of 50 to 160 characters in every one of the nine languages
// (search engines cut longer ones and replace shorter ones with text of their own pick).
func TestSEODescriptionsEveryPageEveryLanguage(t *testing.T) {
	_, h := linkApp(t)
	for _, p := range sitePages {
		for _, code := range []string{"en", "zh-CN", "de", "id", "ja", "pt-BR", "es", "fr", "ru"} {
			rec := hostGet(h, "pixel.example", p+"?lang="+code)
			m := descRe.FindStringSubmatch(rec.Body.String())
			if rec.Code != 200 || m == nil {
				t.Errorf("%s [%s]: status %d, no meta description", p, code, rec.Code)
				continue
			}
			d := html.UnescapeString(m[1])
			if n := utf8.RuneCountInString(d); n < 50 || n > 160 {
				t.Errorf("%s [%s]: description has %d characters (want 50-160): %q", p, code, n, d)
			}
		}
	}
}

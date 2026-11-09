package app

import (
	"net/http/httptest"
	"strings"
	"testing"
)

// Every page footer says the app belongs to Hop Easy Link, in the reader's language.
func TestFooterPartOfFamily(t *testing.T) {
	h := newTestApp(t)
	for lang, want := range map[string]string{"en": "Part of <b>Hop Easy Link</b>", "de": "Teil von <b>Hop Easy Link</b>"} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest("GET", "/about?lang="+lang, nil))
		if body := rec.Body.String(); !strings.Contains(body, `class="part-of" href="https://hel.ink"`) || !strings.Contains(body, want) {
			t.Fatalf("%s: footer row missing %q", lang, want)
		}
	}
}

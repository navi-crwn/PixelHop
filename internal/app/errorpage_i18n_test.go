package app

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// The 404 page and the 500 text were English in every language.
func TestNotFoundPageIsTranslated(t *testing.T) {
	h := newTestApp(t)
	english := "This link doesn&#39;t lead anywhere"
	for _, lang := range []string{"zh-CN", "de", "id", "ja", "pt-BR", "es", "fr", "ru"} {
		req := httptest.NewRequest("GET", "/no-such-page-at-all", nil)
		req.AddCookie(&http.Cookie{Name: langCookie, Value: lang})
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != 404 {
			t.Fatalf("%s: status %d", lang, rec.Code)
		}
		if strings.Contains(rec.Body.String(), english) {
			t.Errorf("%s: the 404 message is still English", lang)
		}
	}
}

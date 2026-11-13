package app

import (
	"net/http/httptest"
	"strings"
	"testing"
)

// Fixed public pages carry the site's social image, so a pasted link shows a card.
func TestFixedPagesHaveASocialImage(t *testing.T) {
	h := newTestApp(t)
	for _, path := range []string{"/", "/changelog", "/legal", "/about", "/features", "/faq", "/help", "/tools", "/tools/batch", "/contact", "/status", "/login", "/register"} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest("GET", path, nil))
		if rec.Code != 200 {
			t.Errorf("%s: status %d", path, rec.Code)
			continue
		}
		body := rec.Body.String()
		if !strings.Contains(body, `property="og:image" content="`) || !strings.Contains(body, "/static/img/social.jpg") || !strings.Contains(body, `name="twitter:card" content="summary_large_image"`) {
			t.Errorf("%s: no og:image", path)
		}
	}
}

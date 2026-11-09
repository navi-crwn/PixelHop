package app

import (
	"net/http"
	"strings"
	"testing"
)

// Shared pages load their translations from /i18n/{lang}.js. That route was not among the ones served on
// the link host, so the browser was sent to the app host for it, another origin that CSP script-src
// 'self' blocks: every text the scripts write (toasts, "Copied") fell back to English there.
func TestTranslationsAreServedOnTheLinkHost(t *testing.T) {
	a, h := linkApp(t)
	im := addImage(t, a, nil, "shared-AbC123")

	rec := hostGet(h, "p.example", "/i18n/id.js?v=test")
	if rec.Code != http.StatusOK || !strings.HasPrefix(rec.Header().Get("Content-Type"), "application/javascript") || !strings.Contains(rec.Body.String(), "window.PIXEL_I18N") {
		t.Fatalf("link host /i18n/id.js: %d %q, Content-Type %q", rec.Code, firstLine(rec.Body.String()), rec.Header().Get("Content-Type"))
	}
	// The shared page asks for the script by a same-origin path, which the page's CSP allows.
	page := hostGet(h, "p.example", "/"+im.ID+"?lang=id")
	if page.Code != http.StatusOK || !strings.Contains(page.Body.String(), `<script src="/i18n/id.js?v=`) {
		t.Errorf("shared page in Indonesian: %d, same-origin i18n script: %v", page.Code, strings.Contains(page.Body.String(), `<script src="/i18n/id.js?v=`))
	}
	if csp := page.Header().Get("Content-Security-Policy"); !strings.Contains(csp, "script-src 'self'") {
		t.Errorf("CSP without script-src 'self': %q", csp)
	}
}

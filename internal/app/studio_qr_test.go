package app

import (
	"net/http"
	"strings"
	"testing"
)

// The studio's QR tool moved to LinkHop's QR page (product decision): the tool button is gone from
// the studio, its server endpoints are gone, and the old /qr address points at LinkHop.
func TestStudioQRToolRemoved(t *testing.T) {
	h := newTestApp(t)
	for _, path := range []string{"/tools/qr.png?text=hi", "/tools/qr.svg?text=hi"} {
		if rec := fetch(h, path); rec.Code != http.StatusNotFound {
			t.Errorf("%s: %d, want 404", path, rec.Code)
		}
	}
	rec := fetch(h, "/qr")
	if rec.Code != http.StatusMovedPermanently || rec.Header().Get("Location") != "https://link.hel.ink/qr" {
		t.Fatalf("/qr: %d -> %q, want 301 to the LinkHop QR page", rec.Code, rec.Header().Get("Location"))
	}
	body := fetch(h, "/tools").Body.String()
	if strings.Contains(body, `data-tool="qr"`) {
		t.Error("the studio still lists a QR tool")
	}
	if !strings.Contains(body, "https://link.hel.ink/qr") {
		t.Error("the studio does not point at LinkHop's QR page")
	}
	// The per-image QR on image pages is a different feature and stays.
	if rec := fetch(h, "/qr/someid.png"); rec.Code == http.StatusMovedPermanently {
		t.Error("/qr/{id} must keep serving per-image QR codes")
	}
}

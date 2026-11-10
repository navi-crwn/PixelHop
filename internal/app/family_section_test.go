package app

import (
	"net/http/httptest"
	"strings"
	"testing"
)

// The home page's family section: one 3D tile per other app (FileHop included),
// name and host under it, no section numbering.
func TestHomeFamilySection(t *testing.T) {
	rec := httptest.NewRecorder()
	newTestApp(t).ServeHTTP(rec, httptest.NewRequest("GET", "/", nil))
	_, sec, _ := strings.Cut(rec.Body.String(), `id="family"`)
	sec, _, _ = strings.Cut(sec, "</section>")
	if n := strings.Count(sec, `class="hop-fam-tile"`); n != 5 {
		t.Fatalf("%d tiles, want 5", n)
	}
	for _, want := range []string{`data-hop-hero="filehop"`, `/static/hop-home/home/filehop-light.webp`, `<span>file.hel.ink</span>`} {
		if !strings.Contains(sec, want) {
			t.Errorf("family section lacks %q", want)
		}
	}
	if strings.Contains(sec, `class="h-label"`) || strings.Contains(sec, `data-hop-hero="pixelhop"`) {
		t.Error("the section keeps its numbered label or lists the app itself")
	}
}

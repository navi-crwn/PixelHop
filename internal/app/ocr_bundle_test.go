package app

import (
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/navi-crwn/pixelhop/web"
)

// The studio's text reading is part of the program: its files are embedded, English is built in, other
// languages come from the owner's tessdata folder, and nothing points at another site.
func TestOCRIsBundledAndServedByThisSite(t *testing.T) {
	a, h := ssoApp(t)
	for _, f := range []string{"tesseract.min.js", "worker.min.js", "tesseract-core-simd-lstm.wasm.js", "lang/eng.traineddata.gz", "LICENSE-tesseract.js.txt", "LICENSE-tesseract.js-core.txt"} {
		if _, err := fs.Stat(web.Static, "static/vendor/tesseract/"+f); err != nil {
			t.Errorf("missing bundled file %s", f)
		}
	}
	js, _ := fs.ReadFile(web.Static, "static/js/studio.js")
	for _, bad := range []string{"cdn.jsdelivr.net/npm/tesseract", "projectnaptha.com"} {
		if strings.Contains(string(js), bad) {
			t.Errorf("studio.js still reaches %s", bad)
		}
	}
	if !strings.Contains(string(js), "langPath: '/tessdata'") || !strings.Contains(string(js), "workerPath:") || !strings.Contains(string(js), "corePath:") {
		t.Error("Tesseract is not pointed at this site")
	}

	// English: the built-in model, gzip.
	rec := fetch(h, "/tessdata/eng.traineddata.gz")
	if rec.Code != 200 || rec.Body.Len() < 1<<20 || rec.Body.Bytes()[0] != 0x1f || rec.Body.Bytes()[1] != 0x8b {
		t.Fatalf("eng: %d, %d bytes", rec.Code, rec.Body.Len())
	}
	// Another language: only when the owner installed it; names are checked.
	if rec := fetch(h, "/tessdata/deu.traineddata.gz"); rec.Code != http.StatusNotFound {
		t.Errorf("deu before install: %d, want 404", rec.Code)
	}
	for _, bad := range []string{"/tessdata/..%2Fpixelhop.db", "/tessdata/DEU.traineddata.gz", "/tessdata/deu.traineddata", "/tessdata/a.traineddata.gz"} {
		if rec := fetch(h, bad); rec.Code == 200 {
			t.Errorf("%s answered 200", bad)
		}
	}
	dir := filepath.Join(a.cfg.DataDir, "tessdata")
	os.MkdirAll(dir, 0o755)
	os.WriteFile(filepath.Join(dir, "deu.traineddata.gz"), []byte("german model"), 0o644)
	os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("x"), 0o644)
	if rec := fetch(h, "/tessdata/deu.traineddata.gz"); rec.Code != 200 || rec.Body.String() != "german model" {
		t.Errorf("deu after install: %d", rec.Code)
	}
	// The studio offers exactly what is installed, English first.
	page := fetch(h, "/tools")
	if !strings.Contains(page.Body.String(), `data-ocr-langs="eng,deu"`) {
		t.Errorf("studio does not list the installed languages")
	}
	if csp := page.Header().Get("Content-Security-Policy"); strings.Contains(csp, "projectnaptha") {
		t.Errorf("CSP still allows the old model host: %s", csp)
	}
}

package app

import (
	"bytes"
	"io/fs"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/navi-crwn/pixelhop/web"
)

// The studio and the batch page open SVG through an <img> in the browser (a picture, never script); the
// server keeps refusing SVG, so only the PNG made in the browser can be uploaded.
func TestSVGOpensInTheBrowserButNeverReachesTheServer(t *testing.T) {
	a, h := ssoApp(t)
	js, err := fs.ReadFile(web.Static, "static/js/svg.js")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"new Image()", "createObjectURL", "createImageBitmap(c)", "window.phBitmap"} {
		if !strings.Contains(string(js), want) {
			t.Errorf("svg.js lacks %s", want)
		}
	}
	for _, page := range []string{"/tools", "/tools/batch"} {
		body := fetch(h, page).Body.String()
		if !strings.Contains(body, "/static/js/svg.js") || strings.Index(body, "/static/js/svg.js") > strings.Index(body, "/static/js/s") && false {
			t.Errorf("%s does not load svg.js", page)
		}
		if !strings.Contains(body, ".svg") {
			t.Errorf("%s does not offer SVG files in its picker", page)
		}
	}
	_, key := member(t, a, "svg@example.com")
	svg := `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 10 10"><script>alert(1)</script></svg>`
	for _, name := range []string{"x.svg", "x.png"} { // a renamed SVG is refused too (the type is read from the bytes)
		var body bytes.Buffer
		mw := multipart.NewWriter(&body)
		fw, _ := mw.CreateFormFile("file", name)
		fw.Write([]byte(svg))
		mw.Close()
		req := httptest.NewRequest("POST", "/api/v1/upload", &body)
		req.Header.Set("Content-Type", mw.FormDataContentType())
		req.Header.Set("Authorization", "Bearer "+key)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("%s: the server answered %d to an SVG, want 400", name, rec.Code)
		}
	}
}

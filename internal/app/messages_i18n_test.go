package app

import (
	"context"
	"encoding/json"
	"io/fs"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/navi-crwn/pixelhop/web"
)

// Every message the server words with a.tr / a.trc must be a key of the English catalog: a message that
// is not reaches people in English whatever their language (the defect this guards: a text written in
// Go and never added to the nine catalogs).
func TestEveryTranslatedMessageIsInTheCatalog(t *testing.T) {
	raw, err := fs.ReadFile(web.I18n, "i18n/en.json")
	if err != nil {
		t.Fatal(err)
	}
	en := map[string]string{}
	if err := json.Unmarshal(raw, &en); err != nil {
		t.Fatal(err)
	}
	call := regexp.MustCompile("a\\.trc?\\((?:r|ctx), \"((?:[^\"\\\\]|\\\\.)*)\"")
	files, _ := filepath.Glob("*.go")
	checked := 0
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		src, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		for _, m := range call.FindAllStringSubmatch(string(src), -1) {
			key := strings.NewReplacer(`\"`, `"`, `\\`, `\`).Replace(m[1])
			checked++
			if _, ok := en[key]; !ok {
				t.Errorf("%s: %q is not in the catalog (web/i18n/*.json)", f, key)
			}
		}
	}
	if checked < 100 {
		t.Fatalf("only %d messages found: the pattern no longer matches the code", checked)
	}
}

// The messages of the server AI tools, of the upload errors and of the image API reach people in their
// own language; what the imaging and download code writes with numbers is worded by its pattern.
func TestToolAndUploadMessagesAreTranslated(t *testing.T) {
	a, h := ssoApp(t)
	_, key := member(t, a, "msgs@example.com")
	get := func(method, path, k string) map[string]any {
		req := httptest.NewRequest(method, path, strings.NewReader(""))
		req.Header.Set("Authorization", "Bearer "+k)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		var out map[string]any
		json.Unmarshal(rec.Body.Bytes(), &out)
		return out
	}
	// No AI service on this server: the tool's refusal in Indonesian.
	if got := get("POST", "/api/v1/tools/upscale?lang=id", key)["error"]; got != "alat server belum disiapkan di sini" {
		t.Errorf("tools, not set up: %v", got)
	}
	// An image that is not there (image API), in German.
	if got := get("GET", "/api/v1/images/nope?lang=de", key)["error"]; got != "Bild nicht gefunden" {
		t.Errorf("image not found: %v", got)
	}

	ctx := context.WithValue(context.Background(), ctxLang{}, "id")
	for in, want := range map[string]string{
		"image is too large (9000×7000); the limit is 40 megapixels": "Gambar ini terlalu besar (9000×7000); batasnya 40 megapiksel.",
		"this file looks damaged: unexpected EOF":                    "File ini tampaknya rusak.",
		"that link answered 404 Not Found":                           "Tautan itu menjawab 404 Not Found.",
		"that image is larger than 20 MB":                            "Gambar itu lebih besar dari 20 MB.",
		"image has no pixels":                                        "gambar tidak memiliki piksel",
		"some message nobody wrote a key for":                        "some message nobody wrote a key for",
	} {
		if got := a.localizedError(ctx, in); got != want {
			t.Errorf("localizedError(%q) = %q, want %q", in, got, want)
		}
	}
	if got := a.tooLarge(ctx, "foto.png", nil, 20<<20).msg; !strings.HasPrefix(got, "foto.png lebih besar dari 20 MB.") {
		t.Errorf("tooLarge: %q", got)
	}
}

// /docs and openapi.json tell API users about the AI share (ai_day) and the X-Day-* headers.
func TestAPIDocsMentionTheAIShare(t *testing.T) {
	_, h := ssoApp(t)
	page := fetch(h, "/docs").Body.String()
	spec, err := fs.ReadFile(web.Static, "static/openapi.json")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"ai_day", "X-Day-Limit", "X-Day-Remaining"} {
		if !strings.Contains(page, want) {
			t.Errorf("/docs does not mention %s", want)
		}
		if !strings.Contains(string(spec), want) {
			t.Errorf("openapi.json does not mention %s", want)
		}
	}
	var parsed map[string]any
	if err := json.Unmarshal(spec, &parsed); err != nil {
		t.Fatalf("openapi.json is not valid JSON: %v", err)
	}
	if _, ok := parsed["paths"].(map[string]any)["/api/v1/tools/{tool}"]; !ok {
		t.Error("openapi.json has no /api/v1/tools/{tool}")
	}
}

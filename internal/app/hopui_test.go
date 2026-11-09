package app

import (
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

// The family's shared dialogs (hop-assets hop-ui, v5 on hel.ink) replace the browser's own
// confirm() and prompt(). The script is loaded from hel.ink with its SRI hash once the bundled
// manifest lists it; until then app.js falls back to the browser's dialogs, so both work.

func TestHopUIDialogsAreLoadedWithTheirHash(t *testing.T) {
	h := newTestApp(t)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/", nil))
	if rec.Code != 200 {
		t.Fatalf("home: status %d", rec.Code)
	}
	body := rec.Body.String()

	ui := hopUI()
	if ui == nil {
		t.Fatal("the bundled manifest should carry ui_css and ui_js with their hashes")
	}
	if got := rec.Header().Get("Content-Security-Policy"); !strings.Contains(got, "script-src 'self' 'wasm-unsafe-eval' https://hel.ink") {
		t.Errorf("CSP does not allow the family script: %s", got)
	}
	// html/template escapes the + of a base64 hash; the browser reads it back before checking the file.
	esc := strings.NewReplacer("+", "&#43;").Replace
	for _, want := range []string{
		`href="` + ui.CSS.URL + `" integrity="` + esc(ui.CSS.Integrity) + `" crossorigin="anonymous"`,
		`src="` + ui.JS.URL + `" integrity="` + esc(ui.JS.Integrity) + `" crossorigin="anonymous" defer`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("home page lacks %q", want)
		}
	}
	// The script comes before app.js, which asks the first question.
	if js, app := strings.Index(body, ui.JS.URL), strings.Index(body, "/static/js/app.js"); app >= 0 && js > app {
		t.Error("hop-ui.js must be on the page before app.js")
	}
}

func TestAManifestWithoutTheDialogsLeavesThemNative(t *testing.T) {
	full := hopManifestOnce().Assets

	// Before hel.ink #58 ships, the copy has neither file: no tag, and app.js keeps the browser's dialog.
	for _, missing := range []string{"ui_css", "ui_js"} {
		partial := map[string]hopAsset{}
		for k, v := range full {
			partial[k] = v
		}
		delete(partial, missing)
		if ui := hopUIFrom(partial); ui != nil {
			t.Errorf("hopUIFrom without %s = %+v, want nil", missing, ui)
		}
	}

	// A file without its hash is not loaded either: the browser could get an unverified script.
	partial := map[string]hopAsset{}
	for k, v := range full {
		partial[k] = v
	}
	js := partial["ui_js"]
	js.Integrity = ""
	partial["ui_js"] = js
	if ui := hopUIFrom(partial); ui != nil {
		t.Errorf("hopUIFrom without the script's hash = %+v, want nil", ui)
	}
}

// app.js is the only place PixelHop asks a question, and every question now goes through the
// helper: the browser's own confirm/prompt appear there twice, as the fallback.
func TestAppJSAsksThroughTheSharedDialogs(t *testing.T) {
	src, err := os.ReadFile("../../web/static/js/app.js")
	if err != nil {
		t.Fatal(err)
	}
	js := string(src)

	if !strings.Contains(js, "if (window.HopUI && HopUI.confirm) return HopUI.confirm(o);") {
		t.Error("the helper no longer prefers HopUI")
	}
	if !strings.Contains(js, "if (window.HopUI && HopUI.prompt) return HopUI.prompt(opts);") {
		t.Error("the prompt helper no longer prefers HopUI")
	}
	if got := strings.Count(js, "window.confirm("); got != 1 {
		t.Errorf("window.confirm appears %d times, want only the one fallback", got)
	}
	if got := strings.Count(js, "window.prompt("); got != 1 {
		t.Errorf("window.prompt appears %d times, want only the one fallback", got)
	}
	// A question built from the app's own text never reaches the browser directly.
	for _, bad := range []string{"confirm(T(", "prompt(T(", "!confirm(", "alert(T("} {
		if strings.Contains(js, bad) {
			t.Errorf("a native dialog is still asked directly: %q", bad)
		}
	}
}

// A delete asks with the name of what goes, so the shared dialog can put it on the red button.
func TestDeleteFormsNameWhatTheyRemove(t *testing.T) {
	h := newTestApp(t)
	for page, want := range map[string]string{
		"/dashboard/trash": `data-confirm-danger`,
	} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest("GET", page, nil))
		if rec.Code == 200 && !strings.Contains(rec.Body.String(), want) {
			t.Errorf("%s: delete forms lack %q", page, want)
		}
	}
}

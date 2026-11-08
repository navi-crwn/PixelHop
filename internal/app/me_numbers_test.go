package app

import (
	"encoding/json"
	"strings"
	"testing"
)

// The dashboard refreshes its numbers from /api/v1/me after an upload (they used to stay at zero until a
// reload). The answer carries live files by kind, with the trash left out, and the album count; the page
// marks the elements the script fills.
func TestMeCarriesTheDashboardNumbers(t *testing.T) {
	a, h := ssoApp(t)
	user, key := member(t, a, "numbers@example.com")
	ids := upload(t, h, key, 3)
	if err := a.db.TrashImage(t.Context(), ids[2]); err != nil {
		t.Fatal(err)
	}
	if _, out := call(t, h, "POST", "/api/v1/albums", key, map[string]any{"title": "Trip", "ids": ids[:2]}); !out.Success {
		t.Fatalf("album: %s", out.Error)
	}

	rec, out := call(t, h, "GET", "/api/v1/me", key, nil)
	var me struct {
		Files  int
		Albums int
		Kinds  map[string]int
	}
	if err := json.Unmarshal(out.Data, &me); err != nil || rec.Code != 200 {
		t.Fatalf("me: %d %s (%v)", rec.Code, rec.Body.String(), err)
	}
	// Three files uploaded at once already made an album of their own, so there are two.
	if me.Files != 2 || me.Kinds["image"] != 2 || me.Kinds["video"] != 0 || me.Kinds["pdf"] != 0 || me.Albums != 2 {
		t.Errorf("me = %+v, want 2 live files (2 images), 2 albums", me)
	}

	body := fetch(h, "/dashboard", sessionFor(t, a, user.ID)).Body.String()
	for _, want := range []string{`data-stat="files"`, `data-stat="files_sub"`, `data-stat="albums"`, `data-stat="storage"`, `data-stat="total"`, `data-count="all"`, `data-count="image"`, `data-count="video"`, `data-count="pdf"`} {
		if !strings.Contains(body, want) {
			t.Errorf("dashboard lacks %s", want)
		}
	}
	// The texts the script writes are in the browser catalog.
	js := fetch(h, "/i18n/id.js").Body.String()
	for _, key := range []string{"dash.files_sub", "dash.n_files", "album.n_images", "dash.n_videos", "dash.n_pdfs"} {
		if !strings.Contains(js, `"`+key+`"`) {
			t.Errorf("browser catalog lacks %s", key)
		}
	}
}

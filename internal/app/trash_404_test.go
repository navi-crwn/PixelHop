package app

import (
	"context"
	"net/http"
	"strings"
	"testing"
)

// A file its owner moved to the trash is hidden like a moderated one, so its links answered
// "451 · removed by the moderators" although nobody moderated anything (dashboard test 2026-11-08,
// finding #12). The trash gets a plain 404; 451 stays for take-downs.
func TestTrashedFileIsNotAModerationTakedown(t *testing.T) {
	a, h := linkApp(t)
	ctx := context.Background()
	owner, key := member(t, a, "trash@example.com")
	ids := upload(t, h, key, 2)
	trashed, moderated := ids[0], ids[1]
	if err := a.db.TrashImage(ctx, trashed); err != nil {
		t.Fatal(err)
	}
	if err := a.db.SetImageHidden(ctx, moderated, true); err != nil {
		t.Fatal(err)
	}
	im, _ := a.db.Image(ctx, trashed)
	mod, _ := a.db.Image(ctx, moderated)

	// Visitors, on the link host and on the app: not found, no word about moderators.
	for _, host := range []string{"p.example", "pixel.example"} {
		for _, path := range []string{"/" + im.ID, "/i/" + im.ID + "." + im.Ext, "/t/" + im.ID + ".jpg"} {
			rec := hostGet(h, host, path)
			if rec.Code != http.StatusNotFound || strings.Contains(rec.Body.String(), "moderator") || rec.Header().Get("Cache-Control") != "private, no-store" {
				t.Errorf("visitor %s%s: %d %q (Cache-Control %q), want a plain uncached 404", host, path, rec.Code, firstLine(rec.Body.String()), rec.Header().Get("Cache-Control"))
			}
		}
	}
	// The owner lands in the trash, and the trash page still shows the thumbnail.
	me := sessionFor(t, a, owner.ID)
	if rec := fetch(h, "/"+im.ID, me); rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/dashboard/trash" {
		t.Errorf("owner opening a trashed link: %d → %q, want 303 → /dashboard/trash", rec.Code, rec.Header().Get("Location"))
	}
	if rec := fetch(h, "/t/"+im.ID+".jpg", me); rec.Code != http.StatusOK {
		t.Errorf("owner's thumbnail of a trashed file: %d", rec.Code)
	}
	if rec := fetch(h, "/dashboard/trash", me); rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "/t/"+im.ID) {
		t.Errorf("trash page: %d, thumbnail listed: %v", rec.Code, strings.Contains(rec.Body.String(), "/t/"+im.ID))
	}
	// Restored, the link works again.
	if err := a.db.RestoreImage(ctx, trashed); err != nil {
		t.Fatal(err)
	}
	if rec := hostGet(h, "p.example", "/"+im.ID); rec.Code != http.StatusOK {
		t.Errorf("restored page: %d", rec.Code)
	}

	// A take-down by the moderators is still a 451, for the owner too.
	for _, c := range []*http.Cookie{nil, me} {
		for _, path := range []string{"/" + mod.ID, "/i/" + mod.ID + "." + mod.Ext} {
			var rec = fetch(h, path)
			if c != nil {
				rec = fetch(h, path, c)
			}
			if rec.Code != http.StatusUnavailableForLegalReasons {
				t.Errorf("moderated %s (cookie %v): %d, want 451", path, c != nil, rec.Code)
			}
		}
	}
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}

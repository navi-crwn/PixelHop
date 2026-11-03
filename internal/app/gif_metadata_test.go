package app

import (
	"context"
	"strings"
	"testing"
)

// GIFs are stored as uploaded, so the page must not claim their metadata is clean.
func TestGifPageDoesNotClaimCleanMetadata(t *testing.T) {
	a, h := ssoApp(t)
	jpg := addImage(t, a, nil, "clean-jpg")
	gif := addImage(t, a, nil, "clean-gif")
	gif.Mime, gif.Ext = "image/gif", "gif"
	if _, err := a.db.ExecContext(context.Background(), `UPDATE images SET mime=?, ext=? WHERE id=?`, gif.Mime, gif.Ext, gif.ID); err != nil {
		t.Fatal(err)
	}
	const claim = `class="pill ok"`
	if body := get(h, "/"+jpg.ID).Body.String(); !strings.Contains(body, claim) {
		t.Fatal("a photo page should still show the clean metadata pill")
	}
	if body := get(h, "/"+gif.ID).Body.String(); strings.Contains(body, claim) {
		t.Fatal("a GIF page claims clean metadata, but GIFs are stored untouched")
	}
}

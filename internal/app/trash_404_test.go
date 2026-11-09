package app

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
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

func TestHiddenMediaIsPrivateForPermittedViewers(t *testing.T) {
	a, h := linkApp(t)
	ctx := context.Background()
	owner, _ := member(t, a, "hidden-owner@example.com")
	admin, _ := member(t, a, "hidden-admin@example.com")
	admin.Role = "admin"
	if err := a.db.UpdateUser(ctx, admin); err != nil {
		t.Fatal(err)
	}
	ownerCookie, adminCookie := sessionFor(t, a, owner.ID), sessionFor(t, a, admin.ID)
	data := testPNG(40, 30)
	for _, reason := range []string{"trash", "moderation"} {
		t.Run(reason, func(t *testing.T) {
			im := addImage(t, a, owner, "hidden-"+reason)
			if reason == "trash" {
				if err := a.db.TrashImage(ctx, im.ID); err != nil {
					t.Fatal(err)
				}
			} else if err := a.db.SetImageHidden(ctx, im.ID, true); err != nil {
				t.Fatal(err)
			}
			for _, key := range []string{im.KeyFull, im.KeyThumb} {
				if err := a.backends[im.Backend].Put(ctx, key, bytes.NewReader(data), int64(len(data)), "image/png"); err != nil {
					t.Fatal(err)
				}
			}
			paths := []string{
				"/i/" + im.ID + ".jpg", "/t/" + im.ID + ".jpg", "/i/" + im.ID + ".jpg?w=20",
				"/i/2026/11/09/" + im.ID + "_original.jpg", "/i/2026/11/09/" + im.ID + "_thumb.jpg",
			}
			for _, path := range paths {
				want := http.StatusNotFound
				if reason == "moderation" {
					want = http.StatusUnavailableForLegalReasons
				}
				if rec := fetch(h, path); rec.Code != want || (reason == "trash" && rec.Header().Get("Cache-Control") != "private, no-store") {
					t.Errorf("visitor %s: %d, Cache-Control %q; want %d", path, rec.Code, rec.Header().Get("Cache-Control"), want)
				}
				for who, cookie := range map[string]*http.Cookie{"owner": ownerCookie, "admin": adminCookie} {
					req := httptest.NewRequest(http.MethodGet, path, nil)
					req.AddCookie(cookie)
					rec := httptest.NewRecorder()
					h.ServeHTTP(rec, req)
					if reason == "moderation" && who == "owner" {
						if rec.Code != http.StatusUnavailableForLegalReasons {
							t.Errorf("owner %s: %d; want 451", path, rec.Code)
						}
						continue
					}
					if rec.Code != http.StatusOK || rec.Body.Len() == 0 || rec.Header().Get("Cache-Control") != "private, no-store" {
						t.Errorf("%s %s: %d, %d bytes, Cache-Control %q; want an uncached file", who, path, rec.Code, rec.Body.Len(), rec.Header().Get("Cache-Control"))
					}
				}
			}
		})
	}
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}

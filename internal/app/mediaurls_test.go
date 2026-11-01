package app

import (
	"bytes"
	"context"
	"database/sql"
	"net/http"
	"strconv"
	"strings"
	"testing"

	"github.com/navi-crwn/pixelhop/internal/db"
)

// HOP ID and Google send the profile picture as they like; whatever arrives
// must end up as an address any of our pages can load.
func TestExternalAvatarIsLoadable(t *testing.T) {
	a, _ := ssoApp(t) // https site, accounts on https://hel.example.com
	for in, want := range map[string]string{
		"":                              "",
		"https://hel.example.com/a.png": "https://hel.example.com/a.png",
		"http://hel.example.com/a.png":  "https://hel.example.com/a.png", // http would be blocked on an https page
		"//hel.example.com/a.png":       "https://hel.example.com/a.png",
		"/storage/avatars/7-x.jpg":      "https://hel.example.com/storage/avatars/7-x.jpg", // a bare path would be asked of this host
		"  /storage/avatars/7-x.jpg  ":  "https://hel.example.com/storage/avatars/7-x.jpg",
		"avatars/7-x.jpg":               "",
		"javascript:alert(1)":           "",
		"data:image/png;base64,AAAA":    "",
	} {
		if got := a.externalAvatar(in); got != want {
			t.Errorf("externalAvatar(%q) = %q, want %q", in, got, want)
		}
	}
	if got := a.avatarOf(&db.User{ID: 7, AvatarKey: "avatars/7-x.jpg", AvatarURL: "https://x.example/a.png"}); !strings.HasPrefix(got, "/avatar/7?v=") {
		t.Errorf("an uploaded photo wins: %q", got)
	}
}

func TestOwnerPhotoFromAccountsIsAbsoluteOnPages(t *testing.T) {
	a, h := ssoApp(t)
	u, _, _ := seedOwner(t, a)
	u.AvatarURL = "/storage/avatars/1-x.jpg" // as HOP ID may send it
	if err := a.db.UpdateUser(context.Background(), u); err != nil {
		t.Fatal(err)
	}
	want := `src="https://hel.example.com/storage/avatars/1-x.jpg"`
	for _, path := range []string{"/public-shot", "/u/maya"} {
		if body := get(h, path).Body.String(); !strings.Contains(body, want) {
			t.Errorf("%s does not show the account photo at an absolute address", path)
		}
	}
	// The address /avatar/{id} hands out follows it too.
	rec := get(h, "/avatar/"+itoa(u.ID))
	if rec.Code != http.StatusFound || rec.Header().Get("Location") != "https://hel.example.com/storage/avatars/1-x.jpg" {
		t.Errorf("avatar redirect: %d → %q", rec.Code, rec.Header().Get("Location"))
	}
}

func itoa(n int64) string { return strconv.FormatInt(n, 10) }

// An uploaded profile photo is loaded by the owner's own pages (top bar,
// settings) even when the profile is hidden, and from the link host where
// the image pages' owner card lives.
func TestUploadedAvatarServedWhereItIsUsed(t *testing.T) {
	a, h := linkApp(t)
	ctx := context.Background()
	u := &db.User{Email: "av@example.com", Name: "Av", QuotaBytes: 1 << 30}
	if err := a.db.InsertUser(ctx, u); err != nil {
		t.Fatal(err)
	}
	pic := testPNG(8, 8)
	if err := a.store.Put(ctx, "avatars/1-x.jpg", bytes.NewReader(pic), int64(len(pic)), "image/jpeg"); err != nil {
		t.Fatal(err)
	}
	u.AvatarKey, u.ShowOwner = "avatars/1-x.jpg", true
	a.db.UpdateUser(ctx, u)
	path := "/avatar/" + itoa(u.ID)

	for _, host := range []string{"pixel.example", "p.example"} {
		rec := hostGet(h, host, path)
		if rec.Code != 200 || rec.Header().Get("Content-Type") != "image/jpeg" {
			t.Errorf("%s%s: %d %s", host, path, rec.Code, rec.Header().Get("Content-Type"))
		}
		if rec.Header().Get("Cross-Origin-Resource-Policy") != "cross-origin" {
			t.Errorf("%s: the photo cannot be embedded on other sites", host)
		}
	}

	u.ShowOwner = false
	a.db.UpdateUser(ctx, u)
	if rec := hostGet(h, "pixel.example", path); rec.Code != 404 {
		t.Errorf("a hidden profile's photo is public: %d", rec.Code)
	}
	rec := hostGet(h, "pixel.example", path, sessionFor(t, a, u.ID))
	if rec.Code != 200 || !strings.HasPrefix(rec.Header().Get("Cache-Control"), "private") {
		t.Errorf("the owner's own photo: %d, cache %q", rec.Code, rec.Header().Get("Cache-Control"))
	}
}

// Files are embedded and hot-linked from other hosts (a BioHop page, a forum):
// nothing may keep a browser from using them there.
func TestFilesCanBeEmbeddedFromOtherSites(t *testing.T) {
	a, h := linkApp(t)
	ctx := context.Background()
	_, key := member(t, a, "emb@example.com")
	d := uploadFile(t, h, key, "pic.png", testPNG(40, 30))
	id := d["id"].(string)
	if d["url"] != "https://p.example/i/"+id+".png" || d["thumb_url"] == "" || !strings.HasPrefix(d["thumb_url"].(string), "https://p.example/t/"+id+".") {
		t.Fatalf("links handed out: %v %v", d["url"], d["thumb_url"])
	}
	for _, path := range []string{"/i/" + id + ".png", "/t/" + id + ".jpg", "/i/" + id + ".png?w=20"} {
		for _, host := range []string{"p.example", "pixel.example"} {
			rec := hostGet(h, host, path)
			hd := rec.Header()
			if rec.Code != 200 || !strings.HasPrefix(hd.Get("Content-Type"), "image/") {
				t.Errorf("%s%s: %d %s", host, path, rec.Code, hd.Get("Content-Type"))
			}
			if hd.Get("Cross-Origin-Resource-Policy") != "cross-origin" || hd.Get("Access-Control-Allow-Origin") != "*" {
				t.Errorf("%s%s: not embeddable (CORP %q, ACAO %q)", host, path, hd.Get("Cross-Origin-Resource-Policy"), hd.Get("Access-Control-Allow-Origin"))
			}
			if hd.Get("X-Frame-Options") != "" && !strings.Contains(path, "?") {
				t.Errorf("%s%s: X-Frame-Options %q", host, path, hd.Get("X-Frame-Options"))
			}
		}
	}

	// A video opened on its own is played by the browser's media page, which obeys
	// the response's policy: it must allow the video itself.
	vid := &db.Image{ID: "clip-AbC123", Title: "clip", Mime: "video/mp4", Ext: "mp4", Size: 5, Width: 2, Height: 2,
		SHA256: "clip", Backend: "local", KeyFull: "clip.mp4", KeyThumb: "clip_t.jpg"}
	if err := a.db.InsertImage(ctx, vid); err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{vid.KeyFull, vid.KeyThumb} {
		a.backends["local"].Put(ctx, k, strings.NewReader("12345"), 5, "x")
	}
	csp := hostGet(h, "p.example", "/i/clip-AbC123.mp4").Header().Get("Content-Security-Policy")
	if !strings.Contains(csp, "media-src 'self'") {
		t.Errorf("video policy %q blocks the video", csp)
	}
	if poster := hostGet(h, "p.example", "/t/clip-AbC123.jpg").Header().Get("Content-Security-Policy"); strings.Contains(poster, "media-src") {
		t.Errorf("poster policy %q", poster)
	}
	// Pages let video play from a storage CDN that /i/ may redirect to.
	if csp := hostGet(h, "p.example", "/clip-AbC123").Header().Get("Content-Security-Policy"); !strings.Contains(csp, "media-src 'self' blob: https:") {
		t.Errorf("page policy %q", csp)
	}
}

// Link previews show a picture, never a video file; and a profile's preview
// picture must not leak a content-warned image.
func TestProfilePreviewPicture(t *testing.T) {
	a, h := ssoApp(t)
	ctx := context.Background()
	u, _, _ := seedOwner(t, a)
	vid := &db.Image{ID: "reel-AbC123", UserID: sql.NullInt64{Int64: u.ID, Valid: true}, Title: "reel", Mime: "video/mp4", Ext: "mp4",
		Size: 10, Width: 10, Height: 10, SHA256: "reel", Backend: "local", KeyFull: "reel.mp4", KeyThumb: "reel_t.jpg", Listed: true}
	if err := a.db.InsertImage(ctx, vid); err != nil {
		t.Fatal(err)
	}
	a.db.SetImageListed(ctx, "public-shot", false) // leave the video alone on the profile
	body := get(h, "/u/maya").Body.String()
	if !strings.Contains(body, `<meta property="og:image" content="https://p.example.com/t/reel-AbC123.jpg">`) {
		t.Fatalf("profile preview picture is not the video's poster: %s", ogImage(body))
	}
}

func ogImage(body string) string {
	_, rest, _ := strings.Cut(body, `property="og:image" content="`)
	v, _, _ := strings.Cut(rest, `"`)
	return v
}

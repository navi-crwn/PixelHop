package app

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/navi-crwn/pixelhop/internal/db"
	"github.com/navi-crwn/pixelhop/internal/hop"
)

// GET /api/v1/me/media: the files another HOP app (BioHop's media picker) may show on a page.

type mediaList struct {
	Success bool   `json:"success"`
	Error   string `json:"error"`
	Data    struct {
		Items []struct {
			ID        string `json:"id"`
			URL       string `json:"url"`
			ThumbURL  string `json:"thumb_url"`
			PageURL   string `json:"page_url"`
			Width     int    `json:"width"`
			Height    int    `json:"height"`
			Bytes     int64  `json:"bytes"`
			Mime      string `json:"mime"`
			Kind      string `json:"kind"`
			Title     string `json:"title"`
			Alt       string `json:"alt"`
			CreatedAt string `json:"created_at"`
			Album     string `json:"album"`
		} `json:"items"`
		Next            string `json:"next"`
		Total           int    `json:"total"`
		HotlinksBlocked bool   `json:"hotlinks_blocked"`
	} `json:"data"`
}

func addMedia(t *testing.T, a *App, u *db.User, id string, createdAt int64, mod func(*db.Image)) *db.Image {
	t.Helper()
	im := &db.Image{ID: id, Title: "Pic " + id, OrigName: id + ".jpg", Mime: "image/jpeg", Ext: "jpg", Size: 1234, Width: 640, Height: 480,
		SHA256: id, Backend: "local", KeyFull: id + ".jpg", KeyThumb: id + "_t.jpg", CreatedAt: createdAt, Alt: "alt " + id}
	if u != nil {
		im.UserID = sql.NullInt64{Int64: u.ID, Valid: true}
	}
	if mod != nil {
		mod(im)
	}
	if err := a.db.InsertImage(context.Background(), im); err != nil {
		t.Fatal(err)
	}
	return im
}

func getMedia(h http.Handler, path string, c *http.Cookie, headers http.Header) (*httptest.ResponseRecorder, mediaList) {
	req := httptest.NewRequest("GET", path, nil)
	if c != nil {
		req.AddCookie(c)
	}
	for k, v := range headers {
		req.Header[k] = v
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	var out mediaList
	json.Unmarshal(rec.Body.Bytes(), &out)
	return rec, out
}

func ids(l mediaList) []string {
	var out []string
	for _, it := range l.Data.Items {
		out = append(out, it.ID)
	}
	return out
}

func TestMyMediaListsOnlyTheCallersEmbeddableFiles(t *testing.T) {
	a, h := ssoApp(t)
	ctx := context.Background()
	me := linked(t, a, "101", "me@example.com")
	other := linked(t, a, "102", "other@example.com")
	mine := sql.NullInt64{Int64: me.ID, Valid: true}
	a.db.InsertAlbum(ctx, &db.Album{ID: "trip-abc123", UserID: mine, Title: "Trip"})
	a.db.InsertAlbum(ctx, &db.Album{ID: "nsfw-abc123", UserID: mine, Title: "Hidden", ContentWarning: "adult"})

	addMedia(t, a, me, "new-aaaaaa", 2000, func(im *db.Image) { im.AlbumID = sql.NullString{String: "trip-abc123", Valid: true} })
	addMedia(t, a, me, "old-aaaaaa", 1000, nil)
	addMedia(t, a, me, "doc-aaaaaa", 1500, func(im *db.Image) { im.Mime = "application/pdf"; im.Ext = "pdf"; im.KeyThumb = "doc_t.png" })
	addMedia(t, a, me, "mov-aaaaaa", 1600, func(im *db.Image) { im.Mime = "video/mp4"; im.Ext = "mp4" })
	// None of these may ever be offered to another app.
	addMedia(t, a, other, "oth-aaaaaa", 3000, nil)
	addMedia(t, a, nil, "gst-aaaaaa", 3000, nil)
	addMedia(t, a, me, "prv-aaaaaa", 3000, func(im *db.Image) { im.Private = true })
	addMedia(t, a, me, "pwd-aaaaaa", 3000, func(im *db.Image) { im.PassHash = HashPassword("x") })
	addMedia(t, a, me, "hid-aaaaaa", 3000, func(im *db.Image) { im.Hidden = true; im.HiddenReason = "moderation" })
	addMedia(t, a, me, "exp-aaaaaa", 3000, func(im *db.Image) { im.ExpiresAt = 1 })
	addMedia(t, a, me, "ttl-aaaaaa", 3000, func(im *db.Image) { im.ExpiresAt = 4102444800 })
	addMedia(t, a, me, "cw-aaaaaaa", 3000, func(im *db.Image) { im.ContentWarning = "sensitive" })
	addMedia(t, a, me, "cwa-aaaaaa", 3000, func(im *db.Image) { im.AlbumID = sql.NullString{String: "nsfw-abc123", Valid: true} })
	trash := addMedia(t, a, me, "trs-aaaaaa", 3000, nil)
	a.db.TrashImage(ctx, trash.ID)

	rec, out := getMedia(h, "/api/v1/me/media", sessionFor(t, a, me.ID), nil)
	if rec.Code != 200 || !out.Success {
		t.Fatalf("list: %d %s", rec.Code, rec.Body.String())
	}
	if got := ids(out); len(got) != 2 || got[0] != "new-aaaaaa" || got[1] != "old-aaaaaa" {
		t.Fatalf("images = %v, want [new-aaaaaa old-aaaaaa]", got)
	}
	if out.Data.Total != 2 || out.Data.Next != "" || out.Data.HotlinksBlocked {
		t.Fatalf("page = %+v", out.Data)
	}
	it := out.Data.Items[0]
	if it.URL != "https://p.example.com/i/new-aaaaaa.jpg" || it.ThumbURL != "https://p.example.com/t/new-aaaaaa.jpg" ||
		it.PageURL != "https://p.example.com/new-aaaaaa" || it.Width != 640 || it.Height != 480 || it.Bytes != 1234 ||
		it.Mime != "image/jpeg" || it.Kind != "image" || it.Title != "Pic new-aaaaaa" || it.Alt != "alt new-aaaaaa" ||
		it.CreatedAt != "1970-01-01T00:33:20Z" || it.Album != "Trip" {
		t.Fatalf("item = %+v", it)
	}
	if out.Data.Items[1].Album != "" {
		t.Fatalf("album of a loose file = %q", out.Data.Items[1].Album)
	}

	// PDFs are a separate list; videos are never offered.
	_, pdfs := getMedia(h, "/api/v1/me/media?type=pdf", sessionFor(t, a, me.ID), nil)
	if got := ids(pdfs); len(got) != 1 || got[0] != "doc-aaaaaa" || pdfs.Data.Items[0].Kind != "pdf" {
		t.Fatalf("pdfs = %v", got)
	}
	for _, bad := range []string{"video", "all", "x"} {
		if rec, _ := getMedia(h, "/api/v1/me/media?type="+bad, sessionFor(t, a, me.ID), nil); rec.Code != 400 {
			t.Errorf("type=%s: %d", bad, rec.Code)
		}
	}

	// Paging with an opaque cursor.
	_, p1 := getMedia(h, "/api/v1/me/media?limit=1", sessionFor(t, a, me.ID), nil)
	if got := ids(p1); len(got) != 1 || got[0] != "new-aaaaaa" || p1.Data.Next == "" || p1.Data.Total != 2 {
		t.Fatalf("page 1 = %v next=%q total=%d", got, p1.Data.Next, p1.Data.Total)
	}
	_, p2 := getMedia(h, "/api/v1/me/media?limit=1&cursor="+p1.Data.Next, sessionFor(t, a, me.ID), nil)
	if got := ids(p2); len(got) != 1 || got[0] != "old-aaaaaa" || p2.Data.Next != "" {
		t.Fatalf("page 2 = %v next=%q", got, p2.Data.Next)
	}

	// The owner switched off embedding on other sites: say so, the pictures would not load on a bio page.
	a.db.SetBlockHotlinks(ctx, me.ID, true)
	if _, out := getMedia(h, "/api/v1/me/media", sessionFor(t, a, me.ID), nil); !out.Data.HotlinksBlocked {
		t.Fatal("hotlinks_blocked should be true")
	}
}

func TestMyMediaNeedsAnAccount(t *testing.T) {
	a, h := ssoApp(t)
	ctx := context.Background()
	me := linked(t, a, "101", "me@example.com")
	addMedia(t, a, me, "new-aaaaaa", 2000, nil)

	if rec, _ := getMedia(h, "/api/v1/me/media", nil, nil); rec.Code != 401 {
		t.Fatalf("anonymous: %d", rec.Code)
	}
	full := "ph_" + randomToken(24)
	a.db.InsertAPIKey(ctx, me.ID, "k", full[:9], hashToken(full))
	if rec, out := getMedia(h, "/api/v1/me/media", nil, http.Header{"Authorization": {"Bearer " + full}}); rec.Code != 200 || len(out.Data.Items) != 1 {
		t.Fatalf("full key: %d %s", rec.Code, rec.Body.String())
	}
	uploadOnly := "ph_" + randomToken(24)
	a.db.InsertAPIKeyScoped(ctx, me.ID, "u", uploadOnly[:9], hashToken(uploadOnly), "upload")
	if rec, _ := getMedia(h, "/api/v1/me/media", nil, http.Header{"Authorization": {"Bearer " + uploadOnly}}); rec.Code != 403 {
		t.Fatalf("upload-only key: %d", rec.Code)
	}
}

// signedMedia signs a call to /api/v1/me/media as another HOP app acting for a HOP ID.
func signedMedia(h http.Handler, app, secret, actor, path string) (*httptest.ResponseRecorder, mediaList) {
	return getMedia(h, path, nil, hop.Headers(app, secret, actor, "GET", path, nil))
}

func TestMyMediaSignedByAnotherHopApp(t *testing.T) {
	a, h := ssoApp(t)
	me := linked(t, a, "101", "me@example.com")
	addMedia(t, a, me, "new-aaaaaa", 2000, nil)
	addMedia(t, a, linked(t, a, "102", "other@example.com"), "oth-aaaaaa", 3000, nil)
	path := "/api/v1/me/media?type=image&limit=20"

	// BioHop, signed with the family's SSO secret, acting for HOP ID 101: that member's files only.
	rec, out := signedMedia(h, "biohop", testSecret, "101", path)
	if rec.Code != 200 || !out.Success {
		t.Fatalf("biohop: %d %s", rec.Code, rec.Body.String())
	}
	if got := ids(out); len(got) != 1 || got[0] != "new-aaaaaa" {
		t.Fatalf("biohop sees %v", got)
	}
	// A HOP ID that never signed in here has no files to show.
	if rec, out := signedMedia(h, "biohop", testSecret, "999", path); rec.Code != 404 || out.Error != "no_account" {
		t.Fatalf("unknown actor: %d %s", rec.Code, rec.Body.String())
	}
	// Signed, but for whom? Refused rather than guessed.
	if rec, out := signedMedia(h, "biohop", testSecret, "", path); rec.Code != 400 || out.Error != "no_actor" {
		t.Fatalf("no actor: %d %s", rec.Code, rec.Body.String())
	}
	// Only the configured media clients, and only with the right secret.
	if rec, out := signedMedia(h, "stranger", testSecret, "101", path); rec.Code != 401 || out.Error != "bad_signature" {
		t.Fatalf("stranger: %d %s", rec.Code, rec.Body.String())
	}
	if rec, out := signedMedia(h, "imperium", testSecret, "101", path); rec.Code != 401 || out.Error != "bad_signature" {
		t.Fatalf("admin client is not a media client: %d %s", rec.Code, rec.Body.String())
	}
	if rec, out := signedMedia(h, "biohop", "another-secret-that-is-long-enough-000000", "101", path); rec.Code != 401 || out.Error != "bad_signature" {
		t.Fatalf("wrong secret: %d %s", rec.Code, rec.Body.String())
	}
	// The query string is part of what is signed.
	rec, out = getMedia(h, path+"&cursor=5", nil, hop.Headers("biohop", testSecret, "101", "GET", path, nil))
	if rec.Code != 401 || out.Error != "bad_signature" {
		t.Fatalf("tampered query: %d %s", rec.Code, rec.Body.String())
	}
	// A media client is not an admin client: BioHop cannot read the moderation API.
	if rec, out := hopCall(t, h, "biohop", "GET", "/hop/health", nil); rec.Code != 401 || out.Error != "bad_signature" {
		t.Fatalf("biohop on /hop: %d %s", rec.Code, rec.Body.String())
	}
	// A suspended member's files are not handed out.
	if err := a.setUserStatus(context.Background(), me, "suspended", ""); err != nil {
		t.Fatal(err)
	}
	if rec, out := signedMedia(h, "biohop", testSecret, "101", path); rec.Code != 403 || out.Error != "account_suspended" {
		t.Fatalf("suspended: %d %s", rec.Code, rec.Body.String())
	}
}

func TestMyMediaPerAppSecret(t *testing.T) {
	// A family with one secret per app: PixelHop holds BioHop's under HOP_SSO_SECRET_BIOHOP and
	// then accepts BioHop's calls with that secret only.
	bioSecret := "biohop-secret-that-is-long-enough-00000000"
	t.Setenv("HOP_SSO_SECRET_BIOHOP", bioSecret)
	a, h := ssoApp(t)
	linked(t, a, "101", "me@example.com")
	path := "/api/v1/me/media"
	if rec, _ := signedMedia(h, "biohop", bioSecret, "101", path); rec.Code != 200 {
		t.Fatalf("own secret: %d %s", rec.Code, rec.Body.String())
	}
	if rec, _ := signedMedia(h, "biohop", testSecret, "101", path); rec.Code != 401 {
		t.Fatalf("shared secret must not work once the app has its own: %d", rec.Code)
	}
}

func TestMyMediaNeedsSSO(t *testing.T) {
	// Without HEL.ink accounts there is no HOP ID to act for; sessions and keys still work.
	h := newTestApp(t)
	path := "/api/v1/me/media"
	if rec, out := signedMedia(h, "biohop", testSecret, "101", path); rec.Code != 404 || out.Error != "not_configured" {
		t.Fatalf("no sso: %d %s", rec.Code, rec.Body.String())
	}
}

func TestMyMediaPickerSecret(t *testing.T) {
	// HOP_PICKER_SECRET: BioHop signs its picker calls with a secret the hub never uses. PixelHop accepts it next
	// to the older secret while the family moves over, and without HOP_SSO_SECRET_BIOHOP it no longer needs
	// BioHop's sign-in secret at all.
	picker := "picker-secret-that-is-long-enough-0000000"
	t.Setenv("HOP_PICKER_SECRET", picker)
	a, h := ssoApp(t)
	linked(t, a, "101", "me@example.com")
	path := "/api/v1/me/media"
	if rec, _ := signedMedia(h, "biohop", picker, "101", path); rec.Code != 200 {
		t.Fatalf("picker secret: %d %s", rec.Code, rec.Body.String())
	}
	if rec, _ := signedMedia(h, "biohop", testSecret, "101", path); rec.Code != 200 {
		t.Fatalf("the older secret keeps working during the move: %d", rec.Code)
	}
	if rec, _ := signedMedia(h, "biohop", "another-secret-that-is-long-enough-000000", "101", path); rec.Code != 401 {
		t.Fatalf("an unknown secret: %d", rec.Code)
	}
}

func TestMyMediaShortPickerSecretIgnored(t *testing.T) {
	t.Setenv("HOP_PICKER_SECRET", "short")
	a, h := ssoApp(t)
	linked(t, a, "101", "me@example.com")
	if rec, _ := signedMedia(h, "biohop", "short", "101", "/api/v1/me/media"); rec.Code != 401 {
		t.Fatalf("a short picker secret must not verify anything: %d", rec.Code)
	}
}

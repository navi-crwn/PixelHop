package app

import (
	"bytes"
	"context"
	"database/sql"
	"image"
	"image/jpeg"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/navi-crwn/pixelhop/internal/db"
	"github.com/navi-crwn/pixelhop/internal/imaging"
)

func TestUsernameNeverFromEmail(t *testing.T) {
	a, _ := ssoApp(t)
	u := &db.User{ID: 0, Email: "carawin.secret@example.com"}
	if got := a.suggestUsername(context.Background(), u); strings.Contains(got, "carawin") || !strings.HasPrefix(got, "user") {
		t.Fatalf("username %q leaks the email", got)
	}
	u.Name = "Rina Putri"
	if got := a.suggestUsername(context.Background(), u); got != "rinaputri" {
		t.Fatalf("got %q", got)
	}
}

func TestUsernameRules(t *testing.T) {
	for _, ok := range []string{"ivan", "ivan.n", "pixel_hop", "a1-b2"} {
		if p := usernameProblem(ok); p != "" {
			t.Errorf("%q rejected: %s", ok, p)
		}
	}
	for _, bad := range []string{"ab", "Ivan", "-ivan", "ivan.", "iv..an", "admin", "api", "with space", "émile", strings.Repeat("a", 31)} {
		if usernameProblem(bad) == "" {
			t.Errorf("%q accepted", bad)
		}
	}
}

// seedOwner creates a user with one listed and one unlisted image.
func seedOwner(t *testing.T, a *App) (*db.User, *db.Image, *db.Image) {
	t.Helper()
	ctx := context.Background()
	u := &db.User{Email: "maya@example.com", Name: "Maya", QuotaBytes: 1 << 30}
	if err := a.db.InsertUser(ctx, u); err != nil {
		t.Fatal(err)
	}
	u.Username = sql.NullString{String: "maya", Valid: true}
	a.db.UpdateUser(ctx, u)
	mk := func(id string, listed bool) *db.Image {
		im := &db.Image{ID: id, UserID: sql.NullInt64{Int64: u.ID, Valid: true}, Title: id, Mime: "image/jpeg", Ext: "jpg",
			Size: 10, Width: 10, Height: 10, SHA256: id, Backend: "local", KeyFull: id + ".jpg", KeyThumb: id + "_t.jpg", Listed: listed}
		if err := a.db.InsertImage(ctx, im); err != nil {
			t.Fatal(err)
		}
		return im
	}
	return u, mk("public-shot", true), mk("private-shot", false)
}

func get(h http.Handler, path string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", path, nil))
	return rec
}

func TestProfileShowsOnlyChosenImages(t *testing.T) {
	a, h := ssoApp(t)
	u, _, _ := seedOwner(t, a)

	rec := get(h, "/u/maya")
	body := rec.Body.String()
	if rec.Code != 200 || !strings.Contains(body, "/public-shot") || strings.Contains(body, "private-shot") {
		t.Fatalf("profile: %d, public shown=%v private shown=%v", rec.Code, strings.Contains(body, "/public-shot"), strings.Contains(body, "private-shot"))
	}
	if rec := get(h, "/u/MAYA"); rec.Code != http.StatusMovedPermanently {
		t.Fatalf("case variant should redirect to the canonical handle, got %d", rec.Code)
	}

	// The image page names its owner and links the profile...
	if body := get(h, "/private-shot").Body.String(); !strings.Contains(body, "Shared by") || !strings.Contains(body, `href="/u/maya"`) {
		t.Fatal("owner missing on the image page")
	}
	// ...until the owner turns that off: then no owner and no profile page.
	u.ShowOwner = false
	a.db.UpdateUser(context.Background(), u)
	if body := get(h, "/private-shot").Body.String(); strings.Contains(body, "Shared by") {
		t.Fatal("owner shown although turned off")
	}
	if rec := get(h, "/u/maya"); rec.Code != 404 {
		t.Fatalf("hidden profile answered %d", rec.Code)
	}
}

func TestHelinkLoginKeepsEditedName(t *testing.T) {
	a, h := ssoApp(t)
	ctx := context.Background()
	u := &db.User{Email: "ivan@example.com", Name: "Ivan the Editor", QuotaBytes: 1 << 20, HelinkID: sql.NullString{String: "42", Valid: true}}
	a.db.InsertUser(ctx, u)

	c, state := start(t, h)
	tk := ticketFor(state)
	tk.Name, tk.Avatar = "Ivan From HELink", "https://hel.example.com/a.png"
	if rec := callback(h, c, signTicket(tk, testSecret)); rec.Code != http.StatusSeeOther {
		t.Fatalf("callback %d", rec.Code)
	}
	got, _ := a.db.UserByID(ctx, u.ID)
	if got.Name != "Ivan the Editor" || got.AvatarURL != "https://hel.example.com/a.png" || !got.Username.Valid {
		t.Fatalf("got name=%q avatar=%q username=%v", got.Name, got.AvatarURL, got.Username)
	}
}

func TestAvatarIsSquareAndClean(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, 300, 120))
	var buf bytes.Buffer
	jpeg.Encode(&buf, src, nil)
	tagged := append(append([]byte{0xFF, 0xD8}, []byte("\xFF\xE1\x00\x10Exif\x00\x00GPS-SECR")...), buf.Bytes()[2:]...)
	out, err := imaging.Avatar(tagged, 1e7, 400)
	if err != nil {
		t.Fatal(err)
	}
	cfg, _, _ := image.DecodeConfig(bytes.NewReader(out))
	if cfg.Width != 400 || cfg.Height != 400 || bytes.Contains(out, []byte("GPS")) {
		t.Fatalf("avatar %dx%d, metadata left=%v", cfg.Width, cfg.Height, bytes.Contains(out, []byte("GPS")))
	}
}

func TestSaveProfileValidates(t *testing.T) {
	a, h := ssoApp(t)
	ctx := context.Background()
	u, _, _ := seedOwner(t, a)
	other := &db.User{Email: "x@example.com", QuotaBytes: 1, Username: sql.NullString{String: "taken", Valid: true}}
	a.db.InsertUser(ctx, other)
	other.Username = sql.NullString{String: "taken", Valid: true}
	a.db.UpdateUser(ctx, other)

	tok := randomToken(32)
	a.db.InsertSession(ctx, hashToken(tok), u.ID, time.Hour, "", "")
	post := func(form string) *httptest.ResponseRecorder {
		req := httptest.NewRequest("POST", "/settings/profile", strings.NewReader(form))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.AddCookie(&http.Cookie{Name: sessionCookie, Value: tok})
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec
	}
	if rec := post("name=M&username=taken&show_owner=on"); rec.Code != 400 {
		t.Fatalf("duplicate handle accepted: %d", rec.Code)
	}
	if rec := post("name=M&username=maya2&website=javascript:alert(1)"); rec.Code != 400 {
		t.Fatalf("bad website accepted: %d", rec.Code)
	}
	if rec := post("name=Maya+R&username=maya.r&bio=Hi&website=hel.ink/b/maya&show_owner=on"); rec.Code != http.StatusSeeOther {
		t.Fatalf("valid profile rejected: %d %s", rec.Code, rec.Body.String())
	}
	got, _ := a.db.UserByID(ctx, u.ID)
	if got.Username.String != "maya.r" || got.Website != "https://hel.ink/b/maya" || !got.ShowOwner {
		t.Fatalf("saved %+v", got)
	}

	// avatar upload through the form
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	fw, _ := mw.CreateFormFile("avatar", "me.jpg")
	jpeg.Encode(fw, image.NewRGBA(image.Rect(0, 0, 50, 80)), nil)
	mw.Close()
	req := httptest.NewRequest("POST", "/settings/avatar", &body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.AddCookie(&http.Cookie{Name: sessionCookie, Value: tok})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	got, _ = a.db.UserByID(ctx, u.ID)
	if rec.Code != http.StatusSeeOther || got.AvatarKey == "" {
		t.Fatalf("avatar upload: %d key=%q", rec.Code, got.AvatarKey)
	}
	if r := get(h, avatarURL(got)); r.Code != 200 || r.Header().Get("Content-Type") != "image/jpeg" {
		t.Fatalf("avatar serve: %d %s", r.Code, r.Header().Get("Content-Type"))
	}
}

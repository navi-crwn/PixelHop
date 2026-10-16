package app

import (
	"bytes"
	"context"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/navi-crwn/pixelhop/internal/db"
)

// member makes a user with an API key and returns the key.
func member(t *testing.T, a *App, email string) (*db.User, string) {
	t.Helper()
	ctx := context.Background()
	u := &db.User{Email: email, Name: "Tester", Role: "user", Status: "active", QuotaBytes: 1 << 30}
	if err := a.db.InsertUser(ctx, u); err != nil {
		t.Fatal(err)
	}
	key := "ph_" + randomToken(24)
	if err := a.db.InsertAPIKey(ctx, u.ID, "test", key[:9], hashToken(key)); err != nil {
		t.Fatal(err)
	}
	return u, key
}

func testPNG(w, h int) []byte {
	m := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			m.Set(x, y, color.RGBA{uint8(x * 6), uint8(y * 8), 120, 255})
		}
	}
	var b bytes.Buffer
	png.Encode(&b, m)
	return b.Bytes()
}

type apiResp struct {
	Success bool            `json:"success"`
	Error   string          `json:"error"`
	Data    json.RawMessage `json:"data"`
}

func call(t *testing.T, h http.Handler, method, path, key string, body any) (*httptest.ResponseRecorder, apiResp) {
	t.Helper()
	var rd *bytes.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		rd = bytes.NewReader(b)
	} else {
		rd = bytes.NewReader(nil)
	}
	req := httptest.NewRequest(method, path, rd)
	req.Header.Set("Content-Type", "application/json")
	if key != "" {
		req.Header.Set("Authorization", "Bearer "+key)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	var r apiResp
	json.Unmarshal(rec.Body.Bytes(), &r)
	return rec, r
}

func upload(t *testing.T, h http.Handler, key string, n int) []string {
	t.Helper()
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	for i := 0; i < n; i++ {
		fw, _ := mw.CreateFormFile("file", "pic.png")
		fw.Write(testPNG(40+i, 30))
	}
	mw.Close()
	req := httptest.NewRequest("POST", "/api/v1/upload", &body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+key)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	var r struct {
		Success bool
		Error   string
		Data    struct{ Images []struct{ ID string } }
	}
	json.Unmarshal(rec.Body.Bytes(), &r)
	if !r.Success {
		t.Fatalf("upload: %d %s", rec.Code, rec.Body.String())
	}
	var ids []string
	for _, im := range r.Data.Images {
		ids = append(ids, im.ID)
	}
	return ids
}

func fetch(h http.Handler, path string, cookies ...*http.Cookie) *httptest.ResponseRecorder {
	req := httptest.NewRequest("GET", path, nil)
	for _, c := range cookies {
		req.AddCookie(c)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestPasswordProtectedImage(t *testing.T) {
	a, h := ssoApp(t)
	_, key := member(t, a, "pw@example.com")
	id := upload(t, h, key, 1)[0]

	if rec := fetch(h, "/i/"+id+".png"); rec.Code != 200 {
		t.Fatalf("public file: %d", rec.Code)
	}
	if rec, r := call(t, h, "PATCH", "/api/v1/images/"+id, key, map[string]any{"visibility": "password", "password": "open-sesame", "alt": "a test"}); !r.Success {
		t.Fatalf("patch: %d %s", rec.Code, r.Error)
	}
	if rec := fetch(h, "/i/"+id+".png"); rec.Code == 200 {
		t.Fatal("locked file served without unlocking")
	}
	if rec := fetch(h, "/"+id); !strings.Contains(rec.Body.String(), "/unlock/"+id) {
		t.Fatalf("locked page should ask for the password (%d)", rec.Code)
	}
	try := func(pw string) *httptest.ResponseRecorder {
		req := httptest.NewRequest("POST", "/unlock/"+id, strings.NewReader(url.Values{"password": {pw}}.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec
	}
	if rec := try("wrong"); rec.Code != http.StatusUnauthorized {
		t.Fatalf("wrong password: %d", rec.Code)
	}
	rec := try("open-sesame")
	if rec.Code != http.StatusSeeOther || len(rec.Result().Cookies()) == 0 {
		t.Fatalf("unlock: %d", rec.Code)
	}
	if rec := fetch(h, "/i/"+id+".png", rec.Result().Cookies()...); rec.Code != 200 {
		t.Fatalf("unlocked file: %d", rec.Code)
	}
	// oEmbed and Explore never show locked images
	if rec := fetch(h, "/oembed?url="+url.QueryEscape("https://p.example.com/"+id)); rec.Code != 404 {
		t.Fatalf("oembed of a locked image: %d", rec.Code)
	}
}

func TestTransformAndAlbums(t *testing.T) {
	a, h := ssoApp(t)
	_, key := member(t, a, "al@example.com")
	ids := upload(t, h, key, 3)

	rec := fetch(h, "/i/"+ids[0]+".png?w=20")
	if rec.Code != 200 {
		t.Fatalf("transform: %d %s", rec.Code, rec.Body.String())
	}
	if cfg, _, err := image.DecodeConfig(bytes.NewReader(rec.Body.Bytes())); err != nil || cfg.Width != 20 {
		t.Fatalf("transform size: %+v %v", cfg, err)
	}
	if rec := fetch(h, "/i/"+ids[0]+".png?w=99999"); rec.Code == 200 && rec.Body.Len() > 0 {
		if cfg, _, _ := image.DecodeConfig(bytes.NewReader(rec.Body.Bytes())); cfg.Width > 40 {
			t.Fatal("transform must never upscale past the original")
		}
	}

	_, r := call(t, h, "POST", "/api/v1/albums", key, map[string]any{"title": "Trip", "ids": ids[:2]})
	if !r.Success {
		t.Fatalf("create album: %s", r.Error)
	}
	var al struct {
		ID     string
		Images []struct{ ID string }
	}
	json.Unmarshal(r.Data, &al)
	if len(al.Images) != 2 {
		t.Fatalf("album has %d images", len(al.Images))
	}
	_, r = call(t, h, "PATCH", "/api/v1/albums/"+al.ID, key, map[string]any{"add": []string{ids[2]}, "order": []string{ids[2], ids[1], ids[0]}, "cover_id": ids[1], "description": "hello"})
	if !r.Success {
		t.Fatalf("update album: %s", r.Error)
	}
	json.Unmarshal(r.Data, &al)
	if len(al.Images) != 3 || al.Images[0].ID != ids[2] {
		t.Fatalf("order not kept: %+v", al.Images)
	}
	// someone else can't touch it
	_, other := member(t, a, "other@example.com")
	if rec, _ := call(t, h, "PATCH", "/api/v1/albums/"+al.ID, other, map[string]any{"title": "mine now"}); rec.Code != 404 {
		t.Fatalf("other user edited the album: %d", rec.Code)
	}
	if rec := fetch(h, "/a/"+al.ID); rec.Code != 200 || !strings.Contains(rec.Body.String(), "hello") {
		t.Fatalf("album page: %d", rec.Code)
	}
	if _, r := call(t, h, "DELETE", "/api/v1/albums/"+al.ID, key, nil); !r.Success {
		t.Fatalf("delete album: %s", r.Error)
	}
	if rec := fetch(h, "/i/"+ids[0]+".png"); rec.Code != 200 {
		t.Fatal("deleting an album must keep its images")
	}
}

func TestPublicPages(t *testing.T) {
	a, h := ssoApp(t)
	u, key := member(t, a, "pub@example.com")
	u.Username.String, u.Username.Valid, u.ShowOwner = "tester", true, true
	a.db.UpdateUser(context.Background(), u)
	id := upload(t, h, key, 1)[0]
	call(t, h, "PATCH", "/api/v1/images/"+id, key, map[string]any{"listed": true})

	for path, want := range map[string]string{
		"/explore":                    id,
		"/sitemap.xml":                "/" + id,
		"/manifest.webmanifest":       "share_target",
		"/sw.js":                      "caches",
		"/api/v1/users/tester/images": id,
		"/oembed?url=https://p.example.com/" + id: `"type":"photo"`,
	} {
		rec := fetch(h, path)
		if rec.Code != 200 || !strings.Contains(rec.Body.String(), want) {
			t.Errorf("%s: %d, want %q in body", path, rec.Code, want)
		}
	}
	if rec := fetch(h, "/api/v1/users/nobody/images"); rec.Code != 404 {
		t.Errorf("unknown profile: %d", rec.Code)
	}
}

func TestWebhookRules(t *testing.T) {
	a, _ := ssoApp(t)
	u, _ := member(t, a, "wh@example.com")
	// private addresses are refused before anything is sent
	for _, bad := range []string{"http://example.com/hook", "https://127.0.0.1/hook", "https://169.254.169.254/latest"} {
		if err := a.checkWebhookURL(bad); err == nil {
			t.Errorf("%s should be refused", bad)
		}
	}
	_ = u
}

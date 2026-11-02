package app

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
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
	"time"

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

func TestUploadedPage(t *testing.T) {
	a, h := ssoApp(t)
	_, key := member(t, a, "res@example.com")
	ids := upload(t, h, key, 3)
	rec := fetch(h, "/uploaded?id="+strings.Join(ids, ","))
	body := rec.Body.String()
	if rec.Code != 200 || strings.Count(body, `class="result-item"`) != 3 || !strings.Contains(body, "Copy all direct links") {
		t.Fatalf("uploaded page: %d", rec.Code)
	}
	call(t, h, "PATCH", "/api/v1/images/"+ids[0], key, map[string]any{"visibility": "private"})
	if rec := fetch(h, "/uploaded?id="+strings.Join(ids, ",")); strings.Count(rec.Body.String(), `class="result-item"`) != 2 {
		t.Fatal("private files must not be listed to others")
	}
	if rec := fetch(h, "/uploaded?id=nope"); rec.Code != 404 {
		t.Fatalf("no files: %d", rec.Code)
	}
}

func TestUploadOnlyKeyCannotReadOrDelete(t *testing.T) {
	a, h := ssoApp(t)
	u, full := member(t, a, "scope@example.com")
	key := "ph_" + randomToken(24)
	if err := a.db.InsertAPIKeyScoped(context.Background(), u.ID, "uploader", key[:9], hashToken(key), "upload"); err != nil {
		t.Fatal(err)
	}
	id := upload(t, h, key, 1)[0] // uploading works
	if rec, _ := call(t, h, "GET", "/api/v1/images", key, nil); rec.Code != http.StatusForbidden {
		t.Fatalf("listing with an upload-only key: %d", rec.Code)
	}
	if rec, _ := call(t, h, "DELETE", "/api/v1/images/"+id, key, nil); rec.Code != http.StatusForbidden {
		t.Fatalf("deleting with an upload-only key: %d", rec.Code)
	}
	if rec, r := call(t, h, "GET", "/api/v1/images/"+id, full, nil); !r.Success {
		t.Fatalf("full key still reads: %d", rec.Code)
	}
}

func TestOwnerCanBlockHotlinking(t *testing.T) {
	a, h := ssoApp(t)
	u, key := member(t, a, "hot@example.com")
	id := upload(t, h, key, 1)[0]
	get := func(referer string) int {
		req := httptest.NewRequest("GET", "/i/"+id+".png", nil)
		req.Host = "p.example.com"
		if referer != "" {
			req.Header.Set("Referer", referer)
		}
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec.Code
	}
	if get("https://forum.example.org/thread/1") != 200 {
		t.Fatal("embedding is allowed until the owner turns it off")
	}
	if err := a.db.SetBlockHotlinks(context.Background(), u.ID, true); err != nil {
		t.Fatal(err)
	}
	a.hotlinks.set(u.ID, true)
	if get("https://forum.example.org/thread/1") != http.StatusForbidden {
		t.Fatal("another website must not show the image")
	}
	if get("") != 200 {
		t.Fatal("a direct visit (no Referer) must still work")
	}
	if get("https://p.example.com/"+id) != 200 {
		t.Fatal("this site's own pages must still show it")
	}
}

// signedIn returns a session cookie for u.
func signedIn(t *testing.T, a *App, u *db.User) *http.Cookie {
	t.Helper()
	tok := randomToken(32)
	if err := a.db.InsertSession(context.Background(), hashToken(tok), u.ID, 3600e9, "203.0.113.9", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 Chrome/120.0 Safari/537.36"); err != nil {
		t.Fatal(err)
	}
	return &http.Cookie{Name: sessionCookie, Value: tok}
}

func post(h http.Handler, path string, form url.Values, c *http.Cookie) *httptest.ResponseRecorder {
	req := httptest.NewRequest("POST", path, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Sec-Fetch-Site", "same-origin")
	req.AddCookie(c)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestExportAndSessions(t *testing.T) {
	a, h := ssoApp(t)
	u, key := member(t, a, "export@example.com")
	id := upload(t, h, key, 2)[0]
	cookie := signedIn(t, a, u)
	other := signedIn(t, a, u) // a second browser

	rec := post(h, "/settings/export", url.Values{}, cookie)
	if rec.Code != 200 || rec.Header().Get("Content-Type") != "application/zip" {
		t.Fatalf("export: %d %s", rec.Code, rec.Header().Get("Content-Type"))
	}
	zr, err := zip.NewReader(bytes.NewReader(rec.Body.Bytes()), int64(rec.Body.Len()))
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, f := range zr.File {
		names = append(names, f.Name)
	}
	if len(names) != 3 || !strings.Contains(strings.Join(names, ","), "account.json") {
		t.Fatalf("zip holds %v, want 2 files and account.json", names)
	}
	if !strings.Contains(rec.Body.String(), id) && len(names) == 0 {
		t.Fatal("no image in the export")
	}

	page := fetch(h, "/settings", cookie).Body.String()
	if !strings.Contains(page, "Chrome · macOS") || !strings.Contains(page, "203.0.113.9") {
		t.Fatal("settings should list this browser")
	}
	if rec := post(h, "/settings/sessions/revoke", url.Values{}, cookie); rec.Code != http.StatusSeeOther {
		t.Fatalf("revoke: %d", rec.Code)
	}
	if rec := fetch(h, "/dashboard", other); rec.Code == 200 {
		t.Fatal("the other browser should be signed out")
	}
	if rec := fetch(h, "/dashboard", cookie); rec.Code != 200 {
		t.Fatalf("this browser stays signed in: %d", rec.Code)
	}
}

func TestTakenDownFileCannotBeUploadedAgain(t *testing.T) {
	a, h := ssoApp(t)
	_, key := member(t, a, "block@example.com")
	sum := sha256.Sum256(testPNG(77, 30))
	a.db.BlockHash(context.Background(), hex.EncodeToString(sum[:]), "test")
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	fw, _ := mw.CreateFormFile("file", "pic.png")
	fw.Write(testPNG(77, 30))
	mw.Close()
	req := httptest.NewRequest("POST", "/api/v1/upload", &body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+key)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if !strings.Contains(rec.Body.String(), "not allowed on this server") {
		t.Fatalf("blocked file: %d %s", rec.Code, rec.Body.String())
	}
	if ids := upload(t, h, key, 1); len(ids) != 1 { // a different file is fine
		t.Fatal("other files still upload")
	}
}

func TestMaintenanceModePausesUploadsOnly(t *testing.T) {
	a, h := ssoApp(t)
	_, key := member(t, a, "maint@example.com")
	id := upload(t, h, key, 1)[0]
	if err := a.db.SetSetting(context.Background(), "read_only", "true"); err != nil {
		t.Fatal(err)
	}
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	fw, _ := mw.CreateFormFile("file", "pic.png")
	fw.Write(testPNG(55, 30))
	mw.Close()
	req := httptest.NewRequest("POST", "/api/v1/upload", &body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+key)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusServiceUnavailable || !strings.Contains(rec.Body.String(), "maintenance") {
		t.Fatalf("upload during maintenance: %d %s", rec.Code, rec.Body.String())
	}
	if fetch(h, "/i/"+id+".png").Code != 200 || fetch(h, "/"+id).Code != 200 {
		t.Fatal("files already online must keep working")
	}
	if st := fetch(h, "/api/v1/upload-status").Body.String(); !strings.Contains(st, `"available":false`) {
		t.Fatalf("upload-status: %s", st)
	}
}

func TestAlbumEmbed(t *testing.T) {
	a, h := ssoApp(t)
	_, key := member(t, a, "embed@example.com")
	ids := upload(t, h, key, 3)
	_, r := call(t, h, "POST", "/api/v1/albums", key, map[string]any{"title": "Trip", "ids": ids})
	var al struct{ ID string }
	json.Unmarshal(r.Data, &al)
	if al.ID == "" {
		t.Fatalf("no album: %s", r.Error)
	}
	if rec, r := call(t, h, "PATCH", "/api/v1/images/"+ids[2], key, map[string]any{"visibility": "private"}); !r.Success {
		t.Fatalf("make private: %d", rec.Code)
	}
	rec := fetch(h, "/a/"+al.ID+"/embed")
	body := rec.Body.String()
	if rec.Code != 200 || !strings.Contains(rec.Header().Get("Content-Security-Policy"), "frame-ancestors *") {
		t.Fatalf("embed: %d csp=%q", rec.Code, rec.Header().Get("Content-Security-Policy"))
	}
	if !strings.Contains(body, ids[0]) || !strings.Contains(body, ids[1]) || strings.Contains(body, ids[2]) {
		t.Fatalf("embed lists what anyone may see, not private files:\n%s", body)
	}
	page := fetch(h, "/a/"+al.ID).Body.String()
	if !strings.Contains(page, "/embed") || !strings.Contains(page, "iframe") {
		t.Fatal("album page should offer the embed code")
	}
	if fetch(h, "/a/nonexistent1/embed").Code != 404 {
		t.Fatal("unknown album")
	}
}

func TestDeletedFilesWaitInTheTrash(t *testing.T) {
	a, h := ssoApp(t)
	u, key := member(t, a, "trash@example.com")
	id := upload(t, h, key, 1)[0]
	cookie := signedIn(t, a, u)

	if rec, r := call(t, h, "DELETE", "/api/v1/images/"+id, key, nil); !r.Success || !strings.Contains(string(r.Data), "trashed") {
		t.Fatalf("delete: %d %s", rec.Code, rec.Body.String())
	}
	if fetch(h, "/i/"+id+".png").Code == 200 || fetch(h, "/"+id).Code == 200 {
		t.Fatal("a trashed file must stop working at once")
	}
	if page := fetch(h, "/dashboard", cookie).Body.String(); strings.Contains(page, `data-id="`+id+`"`) {
		t.Fatal("the dashboard lists only files that are not in the trash")
	}
	if page := fetch(h, "/dashboard/trash", cookie).Body.String(); !strings.Contains(page, "/dashboard/trash/"+id+"/restore") {
		t.Fatal("the trash lists it with a restore button")
	}
	if rec := post(h, "/dashboard/trash/"+id+"/restore", url.Values{}, cookie); rec.Code != http.StatusSeeOther {
		t.Fatalf("restore: %d", rec.Code)
	}
	if fetch(h, "/i/"+id+".png").Code != 200 {
		t.Fatal("a restored file works again")
	}
	call(t, h, "DELETE", "/api/v1/images/"+id, key, nil)
	if rec, _ := call(t, h, "DELETE", "/api/v1/images/"+id, key, nil); rec.Code != 200 { // a second delete removes for good
		t.Fatalf("permanent delete: %d", rec.Code)
	}
	if _, err := a.db.Image(context.Background(), id); err == nil {
		t.Fatal("deleting from the trash removes the file for good")
	}
	// A file left in the trash for more than a week is removed by the janitor.
	id2 := upload(t, h, key, 2)[1]
	call(t, h, "DELETE", "/api/v1/images/"+id2, key, nil)
	a.db.ExecContext(context.Background(), `UPDATE images SET deleted_at=? WHERE id=?`, time.Now().Unix()-8*86400, id2)
	a.purgeTrash(context.Background())
	if _, err := a.db.Image(context.Background(), id2); err == nil {
		t.Fatal("the janitor removes files after a week in the trash")
	}
}

func TestTagsSearchAndFilter(t *testing.T) {
	a, h := ssoApp(t)
	u, key := member(t, a, "tags@example.com")
	ids := upload(t, h, key, 2)
	cookie := signedIn(t, a, u)
	rec, r := call(t, h, "PATCH", "/api/v1/images/"+ids[0], key, map[string]any{"tags": []string{"Beach", " Sun set ", "beach", "../x"}})
	if !r.Success || !strings.Contains(string(r.Data), `"beach"`) || !strings.Contains(string(r.Data), `"sun-set"`) {
		t.Fatalf("tags: %d %s", rec.Code, rec.Body.String())
	}
	page := fetch(h, "/dashboard?tag=beach", cookie).Body.String()
	if !strings.Contains(page, `data-id="`+ids[0]+`"`) || strings.Contains(page, `data-id="`+ids[1]+`"`) {
		t.Fatal("the tag filter shows only files with that tag")
	}
	if page := fetch(h, "/dashboard?q=sun", cookie).Body.String(); !strings.Contains(page, `data-id="`+ids[0]+`"`) {
		t.Fatal("search also looks in tags")
	}
	if page := fetch(h, "/dashboard", cookie).Body.String(); !strings.Contains(page, "?tag=sun-set") {
		t.Fatal("the dashboard lists the user's tags")
	}
	if _, r := call(t, h, "GET", "/api/v1/images/"+ids[0], key, nil); !strings.Contains(string(r.Data), `"tags"`) {
		t.Fatal("the image JSON carries its tags")
	}
}

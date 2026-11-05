package app

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/navi-crwn/pixelhop/internal/config"
	"github.com/navi-crwn/pixelhop/internal/db"
)

// guestApp is a plain server (no HEL.ink) on a fresh data folder, with the
// per-minute limiter out of the way so these tests can send many files.
func guestApp(t *testing.T) (*App, http.Handler) {
	t.Helper()
	t.Setenv("PIXELHOP_DATA", t.TempDir())
	t.Setenv("PIXELHOP_UPLOADS_PER_MINUTE", "1000")
	return openApp(t)
}

func openApp(t *testing.T) (*App, http.Handler) {
	t.Helper()
	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	d, err := db.Open(cfg.DataDir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { d.Close() })
	a, err := New(cfg, d, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	return a, a.Handler()
}

type batchResult struct {
	code   int
	images int
	errs   []string
	retry  string
	err    string
}

// batch sends n small pictures in one request, as a guest from ip (or as the
// member with key when it is set).
func batch(t *testing.T, h http.Handler, ip, key string, n int, form map[string]string) batchResult {
	t.Helper()
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	for i := 0; i < n; i++ {
		fw, _ := mw.CreateFormFile("file", "pic.png")
		fw.Write(testPNG(40+i, 30+n))
	}
	for k, v := range form {
		mw.WriteField(k, v)
	}
	mw.Close()
	req := httptest.NewRequest("POST", "/api/v1/upload", &body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.RemoteAddr = ip + ":4321"
	if key != "" {
		req.Header.Set("Authorization", "Bearer "+key)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	var r struct {
		Error string
		Data  struct {
			Images []struct{ ID string }
			Errors []struct{ Error string }
		}
	}
	json.Unmarshal(rec.Body.Bytes(), &r)
	out := batchResult{code: rec.Code, images: len(r.Data.Images), retry: rec.Header().Get("Retry-After"), err: r.Error}
	for _, e := range r.Data.Errors {
		out.errs = append(out.errs, e.Error)
	}
	return out
}

// An album made by a guest takes ten images; the eleventh is refused with a
// message that names the limit and the sign-up page.
func TestGuestAlbumCap(t *testing.T) {
	a, h := guestApp(t)
	res := batch(t, h, "198.51.100.7", "", 11, nil)
	if res.code != 200 || res.images != 10 || len(res.errs) != 1 {
		t.Fatalf("guest batch of 11: %d, %d images, errors %q", res.code, res.images, res.errs)
	}
	if msg := res.errs[0]; !strings.Contains(msg, "10 images") || !strings.Contains(msg, "free account") || !strings.Contains(msg, "/register") {
		t.Errorf("11th image: %q", msg)
	}
	var n int
	a.db.QueryRow(`SELECT COUNT(*) FROM images WHERE album_id IS NOT NULL`).Scan(&n)
	if n != 10 {
		t.Errorf("album holds %d images, want 10", n)
	}

	// The limit follows the control center setting.
	a.db.SetSetting(context.Background(), "guest_album_max", "3")
	if res := batch(t, h, "198.51.100.8", "", 4, nil); res.images != 3 || len(res.errs) != 1 || !strings.Contains(res.errs[0], "3 images") {
		t.Errorf("guest batch of 4 with the limit at 3: %d images, errors %q", res.images, res.errs)
	}
	// Translated like every other message.
	a.db.SetSetting(context.Background(), "guest_album_max", "2")
	req := httptest.NewRequest("POST", "/api/v1/upload?lang=id", nil)
	if msg := a.guestAlbumFull(req, 2).msg; !strings.Contains(msg, "Album tamu") || !strings.Contains(msg, "2 gambar") {
		t.Errorf("Indonesian message: %q", msg)
	}
}

// One guest address stores 25 files a day; the 26th gets 429 with
// Retry-After, also after a restart, and other addresses are not affected.
func TestGuestDailyCap(t *testing.T) {
	a, h := guestApp(t)
	ip := "203.0.113.9"
	for i := 0; i < 5; i++ {
		if res := batch(t, h, ip, "", 5, nil); res.code != 200 || res.images != 5 || len(res.errs) != 0 {
			t.Fatalf("batch %d: %d, %d images, errors %q", i, res.code, res.images, res.errs)
		}
	}
	res := batch(t, h, ip, "", 1, nil)
	if res.code != http.StatusTooManyRequests || res.retry == "" || !strings.Contains(res.err, "25 files") || !strings.Contains(res.err, "/register") {
		t.Fatalf("26th file: %d, Retry-After %q, %q", res.code, res.retry, res.err)
	}
	if res := batch(t, h, "203.0.113.10", "", 1, nil); res.code != 200 || res.images != 1 {
		t.Errorf("another address: %d, %d images, %q", res.code, res.images, res.err)
	}

	// Sent in pieces: refused when the upload starts.
	req := httptest.NewRequest("POST", "/api/v1/uploads", strings.NewReader(`{"name":"rec.mp4","size":1000}`))
	req.Header.Set("Content-Type", "application/json")
	req.RemoteAddr = ip + ":4321"
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusTooManyRequests || rec.Header().Get("Retry-After") == "" {
		t.Errorf("chunked start on a used-up day: %d, Retry-After %q", rec.Code, rec.Header().Get("Retry-After"))
	}

	// The count lives in the database: a new process on the same data still says no.
	_, h2 := openApp(t)
	if res := batch(t, h2, ip, "", 1, nil); res.code != http.StatusTooManyRequests {
		t.Errorf("after restart: %d %q", res.code, res.err)
	}

	// A batch that crosses the line keeps what fits and refuses the rest.
	a.db.SetSetting(context.Background(), "guest_daily_max", "3")
	if res := batch(t, h, "203.0.113.11", "", 5, map[string]string{"album": "0"}); res.code != 200 || res.images != 3 || len(res.errs) != 2 || !strings.Contains(res.errs[0], "3 files") {
		t.Errorf("batch of 5 with 3 a day: %d, %d images, errors %q", res.code, res.images, res.errs)
	}
	// 0 turns the limit off.
	a.db.SetSetting(context.Background(), "guest_daily_max", "0")
	if res := batch(t, h, ip, "", 1, nil); res.code != 200 || res.images != 1 {
		t.Errorf("limit off: %d %q", res.code, res.err)
	}
}

// Members are not guests: their albums and days are not capped this way.
func TestGuestLimitsSkipMembers(t *testing.T) {
	a, h := guestApp(t)
	_, key := member(t, a, "member@example.com")
	if res := batch(t, h, "203.0.113.20", key, 12, nil); res.code != 200 || res.images != 12 || len(res.errs) != 0 {
		t.Fatalf("member album of 12: %d, %d images, errors %q", res.code, res.images, res.errs)
	}
	for i := 0; i < 3; i++ {
		if res := batch(t, h, "203.0.113.20", key, 5, nil); res.code != 200 || res.images != 5 {
			t.Fatalf("member batch %d: %d, %d images", i, res.code, res.images)
		}
	}
	if res := batch(t, h, "203.0.113.20", key, 1, nil); res.code != 200 || res.images != 1 {
		t.Errorf("member's 28th file: %d %q", res.code, res.err)
	}
	// The member's address stored nothing as a guest: a guest there starts fresh.
	if res := batch(t, h, "203.0.113.20", "", 1, nil); res.code != 200 || res.images != 1 {
		t.Errorf("guest on the member's address: %d %q", res.code, res.err)
	}
}

// Guest upload rows older than two days are pruned by the janitor.
func TestGuestUploadsPruned(t *testing.T) {
	a, _ := guestApp(t)
	ctx := context.Background()
	a.db.Exec(`INSERT INTO guest_uploads (ip_hash, at) VALUES ('old', strftime('%s','now') - 3*86400), ('new', strftime('%s','now'))`)
	a.sweep(ctx)
	var n int
	a.db.QueryRow(`SELECT COUNT(*) FROM guest_uploads`).Scan(&n)
	if n != 1 {
		t.Errorf("%d rows after the sweep, want 1", n)
	}
}

// New accounts start with 128 MB unless the owner sets PIXELHOP_DEFAULT_QUOTA_MB.
func TestDefaultQuota(t *testing.T) {
	t.Setenv("PIXELHOP_DEFAULT_QUOTA_MB", "")
	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.DefaultQuota != 128<<20 || cfg.GuestAlbumMax != 10 || cfg.GuestDailyMax != 25 {
		t.Errorf("defaults: quota %d, album %d, daily %d", cfg.DefaultQuota>>20, cfg.GuestAlbumMax, cfg.GuestDailyMax)
	}
	t.Setenv("PIXELHOP_DEFAULT_QUOTA_MB", "1024")
	if cfg, _ := config.Load(); cfg.DefaultQuota != 1024<<20 {
		t.Errorf("quota from the environment: %d MB", cfg.DefaultQuota>>20)
	}
}

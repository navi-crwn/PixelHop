package app

import (
	"bytes"
	"context"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"math/rand"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/navi-crwn/pixelhop/internal/db"
)

// noisePNG is a small picture nobody else has: every seed gives a different
// file (so none is taken for a duplicate) of almost the same size.
func noisePNG(seed int64) []byte {
	rnd := rand.New(rand.NewSource(seed))
	m := image.NewRGBA(image.Rect(0, 0, 40, 40))
	for y := 0; y < 40; y++ {
		for x := 0; x < 40; x++ {
			m.Set(x, y, color.RGBA{uint8(rnd.Intn(256)), uint8(rnd.Intn(256)), uint8(rnd.Intn(256)), 255})
		}
	}
	var b bytes.Buffer
	png.Encode(&b, m)
	return b.Bytes()
}

func uploadRequest(t *testing.T, path string, data []byte) *http.Request {
	t.Helper()
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	fw, _ := mw.CreateFormFile("file", "noise.png")
	fw.Write(data)
	mw.Close()
	req := httptest.NewRequest("POST", path, &body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	return req
}

// roomFor sets the member's quota to hold exactly n of the given files.
func roomFor(t *testing.T, a *App, id int64, n int, files [][]byte) int64 {
	t.Helper()
	var biggest int
	for _, f := range files {
		biggest = max(biggest, len(f))
	}
	quota := int64(n*biggest) + int64(biggest)/4
	if _, err := a.db.ExecContext(context.Background(), `UPDATE users SET quota_bytes=? WHERE id=?`, quota, id); err != nil {
		t.Fatal(err)
	}
	return quota
}

// Many uploads arriving together against a nearly full quota: only as many as
// fit may be stored, however the requests interleave.
func TestQuotaHoldsUnderParallelUploads(t *testing.T) {
	const parallel, fit = 12, 3
	var files [][]byte
	for i := 0; i < parallel; i++ {
		files = append(files, noisePNG(int64(i+1)))
	}

	run := func(t *testing.T, name string, send func(a *App, h http.Handler, u *db.User, key string, data []byte) int) {
		t.Run(name, func(t *testing.T) {
			a, h := ssoApp(t)
			u, key := member(t, a, "quota-"+name+"@example.com")
			quota := roomFor(t, a, u.ID, fit, files)
			u, _ = a.db.UserByID(context.Background(), u.ID) // with the new quota
			a.cfg.UploadsPerMinute = 1000

			var wg sync.WaitGroup
			var mu sync.Mutex
			stored := 0
			start := make(chan struct{})
			for _, f := range files {
				wg.Add(1)
				go func() {
					defer wg.Done()
					<-start
					if send(a, h, u, key, f) == 200 {
						mu.Lock()
						stored++
						mu.Unlock()
					}
				}()
			}
			close(start)
			wg.Wait()

			used, count := a.db.UserStorage(context.Background(), u.ID)
			if used > quota {
				t.Errorf("stored %d bytes in %d files, quota is %d", used, count, quota)
			}
			if count != fit || stored != fit {
				t.Errorf("want %d files stored, got %d rows and %d accepted answers", fit, count, stored)
			}
			if left := len(a.quota.m); left != 0 {
				t.Errorf("%d quota locks still held after every upload ended", left)
			}
		})
	}

	// Through the whole server, as a client would.
	run(t, "http", func(a *App, h http.Handler, _ *db.User, key string, data []byte) int {
		req := uploadRequest(t, "/api/v1/upload", data)
		req.Header.Set("Authorization", "Bearer "+key)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec.Code
	})

	// Straight into the upload handler: the storage guard in front of it must
	// not be what keeps the quota safe.
	run(t, "handler", func(a *App, h http.Handler, u *db.User, _ string, data []byte) int {
		req := uploadRequest(t, "/api/v1/upload", data)
		req = req.WithContext(context.WithValue(req.Context(), keyUser, u))
		results, _, err := a.handleUpload(req)
		if err != nil || len(results) != 1 || results[0].Err != nil {
			return 0
		}
		return 200
	})
}

// The chunked path re-checks the quota when the file is complete: several big
// uploads started while there was room must not all be kept.
func TestQuotaCheckedWhenChunkedUploadFinishes(t *testing.T) {
	a, h := ssoApp(t)
	u, key := member(t, a, "chunks@example.com")
	files := [][]byte{noisePNG(1), noisePNG(2), noisePNG(3)}
	roomFor(t, a, u.ID, 1, files)

	call := func(method, path string, body []byte) (int, map[string]any) {
		req := httptest.NewRequest(method, path, bytes.NewReader(body))
		req.Header.Set("Authorization", "Bearer "+key)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		var r struct {
			Data map[string]any `json:"data"`
		}
		json.Unmarshal(rec.Body.Bytes(), &r)
		return rec.Code, r.Data
	}
	var ids []string
	for i, f := range files {
		b, _ := json.Marshal(map[string]any{"name": "big.png", "size": len(f)})
		code, data := call("POST", "/api/v1/uploads", b)
		if code != 200 {
			t.Fatalf("start %d: %d", i, code)
		}
		id := data["id"].(string)
		if code, _ := call("PUT", "/api/v1/uploads/"+id+"?offset=0", f); code != 200 {
			t.Fatalf("put %d: %d", i, code)
		}
		ids = append(ids, id)
	}
	ok := 0
	for _, id := range ids {
		if code, _ := call("POST", "/api/v1/uploads/"+id+"/finish", nil); code == 200 {
			ok++
		}
	}
	if used, count := a.db.UserStorage(context.Background(), u.ID); ok != 1 || count != 1 {
		t.Errorf("room for one file: %d finished, %d rows, %d bytes used", ok, count, used)
	}
}

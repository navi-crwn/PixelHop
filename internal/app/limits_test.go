package app

import (
	"bytes"
	"context"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// Guests may upload any file up to 20 MB; members get 100 MB for videos and
// PDFs, while pictures stay at 20 MB for everyone.
func TestUploadLimits(t *testing.T) {
	a, h := ssoApp(t)
	ctx := context.Background()
	u, key := member(t, a, "limits@example.com")

	start := func(size int64, auth string) (int, string) {
		b, _ := json.Marshal(map[string]any{"name": "rec.mp4", "size": size})
		req := httptest.NewRequest("POST", "/api/v1/uploads", bytes.NewReader(b))
		req.Header.Set("Content-Type", "application/json")
		if auth != "" {
			req.Header.Set("Authorization", "Bearer "+auth)
		}
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		var r apiResp
		json.Unmarshal(rec.Body.Bytes(), &r)
		return rec.Code, r.Error
	}
	if code, msg := start(30<<20, ""); code != http.StatusBadRequest || !strings.Contains(msg, "20 MB") || !strings.Contains(msg, "Sign in") {
		t.Errorf("guest 30 MB video: %d %q", code, msg)
	}
	if code, msg := start(15<<20, ""); code != 200 {
		t.Errorf("guest 15 MB video: %d %q", code, msg)
	}
	if code, msg := start(90<<20, key); code != 200 {
		t.Errorf("member 90 MB video: %d %q", code, msg)
	}
	if code, msg := start(150<<20, key); code != http.StatusBadRequest || !strings.Contains(msg, "100 MB") || strings.Contains(msg, "Sign in") {
		t.Errorf("member 150 MB video: %d %q", code, msg)
	}

	// A 25 MB PDF sent the ordinary way: refused for a guest, kept for a member.
	pdf := append([]byte("%PDF-1.4\n"), bytes.Repeat([]byte("0"), 25<<20)...)
	send := func(auth string) string {
		var body bytes.Buffer
		mw := multipart.NewWriter(&body)
		fw, _ := mw.CreateFormFile("file", "statement.pdf")
		fw.Write(pdf)
		mw.Close()
		req := httptest.NewRequest("POST", "/api/v1/upload", &body)
		req.Header.Set("Content-Type", mw.FormDataContentType())
		if auth != "" {
			req.Header.Set("Authorization", "Bearer "+auth)
		}
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec.Body.String()
	}
	if out := send(""); !strings.Contains(out, "larger than 20 MB") {
		t.Errorf("guest 25 MB PDF: %s", trim(out, 300))
	}
	if out := send(key); strings.Contains(out, "larger than") {
		t.Errorf("member 25 MB PDF refused: %s", trim(out, 300))
	}

	// The upload box tells each one their own limits.
	if body := fetch(h, "/").Body.String(); !strings.Contains(body, "Images, videos &amp; PDFs up to 20 MB each") {
		t.Error("guest upload box doesn't say 20 MB for everything")
	}
	tok := randomToken(32)
	a.db.InsertSession(ctx, hashToken(tok), u.ID, time.Hour, "", "")
	if body := fetch(h, "/", &http.Cookie{Name: sessionCookie, Value: tok}).Body.String(); !strings.Contains(body, "videos &amp; PDFs up to 100 MB") {
		t.Error("member upload box doesn't say 100 MB for videos")
	}
}

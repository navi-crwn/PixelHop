package app

import (
	"bytes"
	"context"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// A tiny but valid one-page PDF.
const testPDF = "%PDF-1.4\n1 0 obj<</Type/Catalog/Pages 2 0 R>>endobj\n2 0 obj<</Type/Pages/Kids[3 0 R]/Count 1>>endobj\n" +
	"3 0 obj<</Type/Page/Parent 2 0 R/MediaBox[0 0 200 120]/Contents 4 0 R/Resources<<>>>>endobj\n" +
	"4 0 obj<</Length 35>>stream\n0 0 1 rg 20 20 160 80 re f\nendstream endobj\n" +
	"trailer<</Root 1 0 R>>\n%%EOF\n"

func testVideo(t *testing.T) []byte {
	t.Helper()
	if !have("ffmpeg") || !have("ffprobe") {
		t.Skip("ffmpeg not installed")
	}
	out := filepath.Join(t.TempDir(), "v.mov")
	cmd := exec.Command("ffmpeg", "-v", "error", "-f", "lavfi", "-i", "testsrc=size=320x240:rate=25:duration=2",
		"-c:v", "libx264", "-pix_fmt", "yuv420p", "-metadata", "location=+48.8577+002.2950/", out)
	if b, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("ffmpeg: %v %s", err, b)
	}
	b, _ := os.ReadFile(out)
	return b
}

func uploadFile(t *testing.T, h http.Handler, key, name string, data []byte) map[string]any {
	t.Helper()
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	fw, _ := mw.CreateFormFile("file", name)
	fw.Write(data)
	mw.Close()
	req := httptest.NewRequest("POST", "/api/v1/upload", &body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+key)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	var r struct {
		Success bool
		Error   string
		Data    map[string]any
	}
	json.Unmarshal(rec.Body.Bytes(), &r)
	if !r.Success {
		t.Fatalf("upload %s: %d %s", name, rec.Code, rec.Body.String())
	}
	return r.Data
}

func TestVideoUploadAndOptimise(t *testing.T) {
	a, h := ssoApp(t)
	_, key := member(t, a, "vid@example.com")
	d := uploadFile(t, h, key, "screen recording.mov", testVideo(t))
	id := d["id"].(string)
	if d["mime"] != "video/mp4" {
		t.Fatalf("mime %v", d["mime"])
	}
	if rm, _ := d["removed"].([]any); len(rm) == 0 {
		t.Errorf("the location tag should be reported as removed: %v", d["removed"])
	}
	page := fetch(h, "/"+id)
	if page.Code != 200 || !strings.Contains(page.Body.String(), "<video") {
		t.Fatalf("video page: %d", page.Code)
	}
	if rec := fetch(h, "/t/"+id+".jpg"); rec.Code != 200 || !strings.HasPrefix(rec.Header().Get("Content-Type"), "image/") {
		t.Fatalf("poster: %d", rec.Code)
	}
	req := httptest.NewRequest("GET", "/i/"+id+".mp4", nil)
	req.Header.Set("Range", "bytes=0-99")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusPartialContent || rec.Body.Len() != 100 {
		t.Fatalf("range request: %d, %d bytes", rec.Code, rec.Body.Len())
	}
	if !a.db.MediaJobPending(context.Background(), id) {
		t.Fatal("video should be queued for optimising")
	}
	if err := a.optimizeVideo(context.Background(), id); err != nil {
		t.Fatal(err)
	}
	im, _ := a.db.Image(context.Background(), id)
	if im.Mime != "video/mp4" || im.Size <= 0 {
		t.Fatalf("after optimising: %+v", im)
	}
	if rec := fetch(h, "/i/"+id+".mov"); rec.Code != 200 { // the old extension keeps working
		t.Fatalf("old link: %d", rec.Code)
	}
}

func TestPDFUpload(t *testing.T) {
	a, h := ssoApp(t)
	_, key := member(t, a, "pdf@example.com")
	d := uploadFile(t, h, key, "statement.pdf", []byte(testPDF))
	id := d["id"].(string)
	if d["mime"] != "application/pdf" {
		t.Fatalf("mime %v", d["mime"])
	}
	rec := fetch(h, "/i/"+id+".pdf")
	if rec.Code != 200 || !bytes.HasPrefix(rec.Body.Bytes(), []byte("%PDF")) || strings.Contains(rec.Header().Get("Content-Security-Policy"), "sandbox") {
		t.Fatalf("pdf file: %d %q", rec.Code, rec.Header().Get("Content-Security-Policy"))
	}
	if rec := fetch(h, "/"+id); !strings.Contains(rec.Body.String(), "Open PDF") {
		t.Fatal("pdf page has no open button")
	}
	if rec := fetch(h, "/i/"+id+".pdf?w=100"); !bytes.HasPrefix(rec.Body.Bytes(), []byte("%PDF")) {
		t.Fatal("transforms must not apply to PDFs")
	}
}

func TestChunkedUpload(t *testing.T) {
	a, h := ssoApp(t)
	_, key := member(t, a, "chunk@example.com")
	data := testVideo(t)
	_, r := call(t, h, "POST", "/api/v1/uploads", key, map[string]any{"name": "big.mp4", "size": len(data)})
	if !r.Success {
		t.Fatalf("start: %s", r.Error)
	}
	var st struct {
		ID string `json:"id"`
	}
	json.Unmarshal(r.Data, &st)
	put := func(off int, b []byte) *httptest.ResponseRecorder {
		req := httptest.NewRequest("PUT", "/api/v1/uploads/"+st.ID+"?offset="+strconv.Itoa(off), bytes.NewReader(b))
		req.Header.Set("Authorization", "Bearer "+key)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec
	}
	half := len(data) / 2
	if rec := put(0, data[:half]); rec.Code != 200 {
		t.Fatalf("piece 1: %d %s", rec.Code, rec.Body.String())
	}
	if rec := put(0, data[:half]); rec.Code != http.StatusConflict { // a resend tells where to continue
		t.Fatalf("resend: %d", rec.Code)
	}
	// someone else can't add to it
	_, other := member(t, a, "x@example.com")
	req := httptest.NewRequest("PUT", "/api/v1/uploads/"+st.ID+"?offset="+strconv.Itoa(half), bytes.NewReader(data[half:]))
	req.Header.Set("Authorization", "Bearer "+other)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != 404 {
		t.Fatalf("other user's piece: %d", rec.Code)
	}
	if rec := put(half, data[half:]); rec.Code != 200 {
		t.Fatalf("piece 2: %d", rec.Code)
	}
	_, r = call(t, h, "POST", "/api/v1/uploads/"+st.ID+"/finish", key, nil)
	if !r.Success {
		t.Fatalf("finish: %s", r.Error)
	}
	var im struct{ Mime string }
	json.Unmarshal(r.Data, &im)
	if im.Mime != "video/mp4" {
		t.Fatalf("mime %q", im.Mime)
	}
}

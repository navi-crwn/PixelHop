package app

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func TestUploadStoragePauseAndRecovery(t *testing.T) {
	a, h := ssoApp(t)
	free := int64(1 << 30)
	a.space = func(string) (int64, int64) { return free, 20 << 30 }
	for _, path := range []string{"/api/v1/upload", "/api/upload.php", "/api/v1/uploads", "/api/v1/uploads/test/finish", "/share"} {
		r := httptest.NewRequest("POST", path, strings.NewReader("{}"))
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != 507 || !strings.Contains(w.Body.String(), storagePaused) || w.Header().Get("Retry-After") == "" {
			t.Fatalf("%s: %d %s", path, w.Code, w.Body)
		}
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("PUT", "/api/v1/uploads/test", strings.NewReader("chunk")))
	if w.Code != 507 {
		t.Fatalf("chunk bypass: %d", w.Code)
	}
	free = 4 << 30
	w = httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("GET", "/api/v1/upload-status", nil))
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"available":true`) {
		t.Fatal(w.Body.String())
	}
	w = httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("POST", "/api/v1/uploads", strings.NewReader(`{"name":"test.png","size":100}`)))
	if w.Code != 200 {
		t.Fatalf("recovery: %d %s", w.Code, w.Body)
	}
}

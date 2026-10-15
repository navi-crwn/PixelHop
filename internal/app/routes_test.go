package app

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/navi-crwn/pixelhop/internal/config"
	"github.com/navi-crwn/pixelhop/internal/db"
)

// newTestApp builds a full App on a temporary data folder. Building the
// handler also proves no two routes conflict (net/http panics if they do).
func newTestApp(t *testing.T) http.Handler {
	t.Helper()
	t.Setenv("PIXELHOP_DATA", t.TempDir())
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
	return a.Handler()
}

func TestLegacyLinks(t *testing.T) {
	h := newTestApp(t)
	cases := []struct {
		path string
		code int
		to   string
	}{
		{"/", 200, ""},
		{"/abc123/", 301, "/abc123"},
		{"/result?id=nope", 404, ""},
		{"/static/css/app.css", 200, ""},
		{"/i/2026/08/27/missing_original.webp", 404, ""},
		{"/healthz", 200, ""},
	}
	for _, c := range cases {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest("GET", c.path, nil))
		if rec.Code != c.code {
			t.Errorf("%s: status %d, want %d", c.path, rec.Code, c.code)
		}
		if c.to != "" && rec.Header().Get("Location") != c.to {
			t.Errorf("%s: redirect to %q, want %q", c.path, rec.Header().Get("Location"), c.to)
		}
	}
}

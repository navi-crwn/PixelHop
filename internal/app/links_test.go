package app

import (
	"context"
	"database/sql"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/navi-crwn/pixelhop/internal/config"
	"github.com/navi-crwn/pixelhop/internal/db"
)

// The app lives on pixel.example; shared links stay on p.example.
func linkApp(t *testing.T) (*App, http.Handler) {
	t.Helper()
	t.Setenv("PIXELHOP_DATA", t.TempDir())
	t.Setenv("PIXELHOP_BASE_URL", "https://pixel.example")
	t.Setenv("PIXELHOP_LINK_URL", "https://p.example")
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

func hostGet(h http.Handler, host, path string, cookies ...*http.Cookie) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, "https://"+host+path, nil)
	for _, c := range cookies {
		req.AddCookie(c)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestLinkURLMustBeASite(t *testing.T) {
	t.Setenv("PIXELHOP_BASE_URL", "https://pixel.example")
	for _, bad := range []string{"https://p.example/p", "p.example", "https://pixel.example/x"} {
		t.Setenv("PIXELHOP_LINK_URL", bad)
		if _, err := config.Load(); err == nil {
			t.Errorf("PIXELHOP_LINK_URL=%q was accepted", bad)
		}
	}
}

func TestSharedLinksStayOnLinkHost(t *testing.T) {
	a, h := linkApp(t)
	ctx := context.Background()
	owner := &db.User{Email: "cs@example.com", Name: "CS", QuotaBytes: 1 << 30}
	if err := a.db.InsertUser(ctx, owner); err != nil {
		t.Fatal(err)
	}
	pub := addImage(t, a, owner, "struk-AbC123")
	priv := &db.Image{ID: "rahasia-XyZ789", Title: "secret", Mime: "image/jpeg", Ext: "jpg", Size: 1, Width: 1, Height: 1,
		SHA256: "x", Backend: "local", KeyFull: "x.jpg", KeyThumb: "x_t.jpg", Private: true,
		UserID: sql.NullInt64{Int64: owner.ID, Valid: true}}
	if err := a.db.InsertImage(ctx, priv); err != nil {
		t.Fatal(err)
	}
	session := sessionFor(t, a, owner.ID)

	// Links handed out point at the link host.
	if l := a.linksFor(pub); l.Page != "https://p.example/struk-AbC123" || l.Direct != "https://p.example/i/struk-AbC123.jpg" {
		t.Fatalf("links = %+v", l)
	}

	// The shared page opens on the link host, with the link host as canonical.
	rec := hostGet(h, "p.example", "/struk-AbC123")
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), `<link rel="canonical" href="https://p.example/struk-AbC123">`) {
		t.Fatalf("link host page: %d", rec.Code)
	}
	if strings.Contains(rec.Body.String(), "manifest.webmanifest") {
		t.Error("the link host offers the app for install")
	}
	// ... even when the app's session cookie comes along: no sign-in there.
	if rec := hostGet(h, "p.example", "/struk-AbC123", session); strings.Contains(rec.Body.String(), "Sign out") {
		t.Error("signed in on the link host")
	}

	cases := []struct {
		host, path string
		cookie     *http.Cookie
		code       int
		to         string
	}{
		// A visitor on the old app address of a shared page goes to the link host.
		{"pixel.example", "/struk-AbC123", nil, 302, "https://p.example/struk-AbC123"},
		// The signed-in owner stays on the app.
		{"pixel.example", "/struk-AbC123", session, 200, ""},
		// The app's own pages on the link host go to the app.
		{"p.example", "/", nil, 301, "https://pixel.example/"},
		{"p.example", "/login?next=/dashboard", nil, 301, "https://pixel.example/login?next=/dashboard"},
		{"p.example", "/dashboard", session, 301, "https://pixel.example/dashboard"},
		{"p.example", "/auth/google-callback.php?code=x&state=y", nil, 301, "https://pixel.example/auth/google-callback.php?code=x&state=y"},
		// A private file's page on the link host goes to the app, where its owner is signed in.
		{"p.example", "/rahasia-XyZ789", nil, 302, "https://pixel.example/rahasia-XyZ789"},
		{"pixel.example", "/rahasia-XyZ789", nil, 404, ""},
		{"pixel.example", "/rahasia-XyZ789", session, 200, ""},
		// What a shared page needs keeps working on the link host.
		{"p.example", "/static/css/app.css", nil, 200, ""},
		{"p.example", "/qr/struk-AbC123.png", nil, 200, ""},
		{"p.example", "/api/v1/stats", nil, 200, ""},
		{"pixel.example", "/api/v1/stats", nil, 200, ""},
		// Dedicated pages live on the app host only.
		{"p.example", "/about", nil, 301, "https://pixel.example/about"},
		{"p.example", "/nope-AAAAAA", nil, 404, ""},
		// The app itself is unchanged.
		{"pixel.example", "/", nil, 200, ""},
	}
	for _, c := range cases {
		var cs []*http.Cookie
		if c.cookie != nil {
			cs = append(cs, c.cookie)
		}
		rec := hostGet(h, c.host, c.path, cs...)
		if rec.Code != c.code || rec.Header().Get("Location") != c.to {
			t.Errorf("%s%s (cookie %v): %d → %q, want %d → %q", c.host, c.path, c.cookie != nil, rec.Code, rec.Header().Get("Location"), c.code, c.to)
		}
	}

	// Forms posted on the link host keep their method when sent on.
	req := httptest.NewRequest(http.MethodPost, "https://p.example/api/upload.php", nil)
	if rec := httptest.NewRecorder(); func() bool { h.ServeHTTP(rec, req); return rec.Code == 307 || rec.Code == 308 }() {
		t.Error("the API was sent away from the link host")
	}
	req = httptest.NewRequest(http.MethodPost, "https://p.example/share", nil)
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != 308 || rec.Header().Get("Location") != "https://pixel.example/share" {
		t.Errorf("POST /share on the link host: %d → %q", rec.Code, rec.Header().Get("Location"))
	}

	// Phones that installed the app from the link host let go of it.
	if rec := hostGet(h, "p.example", "/sw.js"); !strings.Contains(rec.Body.String(), "unregister") {
		t.Error("the link host still serves the app's service worker")
	}
}

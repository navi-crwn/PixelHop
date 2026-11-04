package app

import (
	"context"
	"database/sql"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/navi-crwn/pixelhop/internal/config"
	"github.com/navi-crwn/pixelhop/internal/db"
	"github.com/navi-crwn/pixelhop/internal/storage"
)

// s3App registers an S3 backend with a CDN address, backed by a fake server that answers every GET.
func s3App(t *testing.T) *App {
	t.Helper()
	a, _ := ssoApp(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/jpeg")
		w.Write([]byte("synthetic"))
	}))
	t.Cleanup(srv.Close)
	a.backends["s3"] = storage.NewS3(config.S3Config{Endpoint: srv.URL, Region: "auto", Bucket: "b", AccessKey: "k", SecretKey: "s", PathStyle: true, PublicURL: "https://cdn.example.test"})
	return a
}

// Private, password-protected and expiring files must never be sent to the
// public CDN address, which has no access checks.
func TestLockedS3FilesDoNotRedirectToPublicURL(t *testing.T) {
	a := s3App(t)
	h := a.Handler()
	ctx := context.Background()
	u := linked(t, a, "private-s3-user", "private-s3@example.com")
	owner := sessionFor(t, a, u.ID)
	uid := sql.NullInt64{Int64: u.ID, Valid: true}
	soon := time.Now().Add(time.Hour).Unix()
	for _, tc := range []struct {
		im       *db.Image
		redirect bool
	}{
		{&db.Image{ID: "private-s3", UserID: uid, Private: true}, false},
		{&db.Image{ID: "password-s3", UserID: uid, PassHash: "hash"}, false},
		{&db.Image{ID: "expiring-s3", UserID: uid, ExpiresAt: soon}, false},
		{&db.Image{ID: "public-s3", UserID: uid}, true},
	} {
		im := tc.im
		im.Mime, im.Ext, im.Backend, im.KeyFull, im.KeyThumb = "image/jpeg", "jpg", "s3", im.ID+".jpg", im.ID+"_t.jpg"
		if err := a.db.InsertImage(ctx, im); err != nil {
			t.Fatal(err)
		}
		for _, path := range []string{"/i/" + im.ID + ".jpg", "/t/" + im.ID + ".jpg"} {
			req := httptest.NewRequest("GET", path, nil)
			req.AddCookie(owner)
			if im.PassHash != "" {
				req.AddCookie(&http.Cookie{Name: unlockCookieName(im.ID), Value: a.unlockToken(im)})
			}
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)
			loc := rec.Header().Get("Location")
			if tc.redirect {
				if rec.Code != http.StatusFound || !strings.HasPrefix(loc, "https://cdn.example.test/") {
					t.Errorf("%s: public file should redirect to the CDN: %d %q", path, rec.Code, loc)
				}
				continue
			}
			if strings.Contains(loc, "cdn.example.test") {
				t.Errorf("%s: redirected to the public address %q", path, loc)
			}
			if rec.Code != http.StatusOK || rec.Body.String() != "synthetic" {
				t.Errorf("%s: want the app to serve the file, got %d %q", path, rec.Code, rec.Body.String())
			}
		}
	}
}

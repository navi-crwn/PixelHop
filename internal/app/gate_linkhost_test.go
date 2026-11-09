package app

import (
	"context"
	"database/sql"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/navi-crwn/pixelhop/internal/db"
)

// The content warning on the link host (p.hel.ink) posts to /gate/... there. That route used to be missing from
// linkPatterns, so the post got a 308 to the app host: another origin, which CSP form-action blocks, and the gate
// cookie would have been set on the wrong host anyway. The confirmation must stay on the host it was posted to.
func TestGateConfirmStaysOnTheHostItWasPostedTo(t *testing.T) {
	a, h := linkApp(t)
	ctx := context.Background()
	owner := &db.User{Email: "gate@example.com", Name: "Gate", QuotaBytes: 1 << 30}
	if err := a.db.InsertUser(ctx, owner); err != nil {
		t.Fatal(err)
	}
	im := addImage(t, a, owner, "adult-AbC123")
	im.ContentWarning = "adult"
	if err := a.db.UpdateImageMeta(ctx, im); err != nil {
		t.Fatal(err)
	}
	al := &db.Album{ID: "album-XyZ789", Title: "Sensitive album", UserID: im.UserID, ContentWarning: "sensitive"}
	if err := a.db.InsertAlbum(ctx, al); err != nil {
		t.Fatal(err)
	}
	other := addImage(t, a, owner, "inside-QwE456")
	other.AlbumID = sql.NullString{String: al.ID, Valid: true}
	if err := a.db.UpdateImageMeta(ctx, other); err != nil {
		t.Fatal(err)
	}

	cases := []struct{ host, gate, page, cookie string }{
		{"p.example", "/gate/" + im.ID, "/" + im.ID, gateCookieName(im.ID)},
		{"p.example", "/gate/a/" + al.ID, "/a/" + al.ID, albumGateCookieName(al.ID)},
		// A signed-in owner looks at the page on the app host; the gate there must stay there too.
		{"pixel.example", "/gate/" + im.ID, "/" + im.ID, gateCookieName(im.ID)},
		{"pixel.example", "/gate/a/" + al.ID, "/a/" + al.ID, albumGateCookieName(al.ID)},
	}
	for _, c := range cases {
		form := url.Values{"remember": {"1"}}
		req := httptest.NewRequest(http.MethodPost, "https://"+c.host+c.gate, strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != c.page {
			t.Errorf("POST %s%s: %d → %q, want 303 → %q (same host)", c.host, c.gate, rec.Code, rec.Header().Get("Location"), c.page)
			continue
		}
		var got *http.Cookie
		for _, ck := range rec.Result().Cookies() {
			if ck.Name == c.cookie {
				got = ck
			}
		}
		if got == nil || got.Value != "1" || got.Domain != "" {
			t.Errorf("POST %s%s: gate cookie %+v, want a host-only %s=1", c.host, c.gate, got, c.cookie)
			continue
		}
		if c.host != "p.example" {
			continue
		}
		// The page on the link host, with the cookie, no longer shows the gate.
		page := hostGet(h, c.host, c.page, got)
		if page.Code != http.StatusOK || strings.Contains(page.Body.String(), `action="`+c.gate+`"`) {
			t.Errorf("GET %s%s after the gate: %d, gate form still shown: %v", c.host, c.page, page.Code, strings.Contains(page.Body.String(), `action="`+c.gate+`"`))
		}
	}

	// Without the cookie the link host still shows the gate.
	if page := hostGet(h, "p.example", "/"+im.ID); !strings.Contains(page.Body.String(), `action="/gate/`+im.ID+`"`) {
		t.Errorf("the gate is missing on the link host without the cookie (status %d)", page.Code)
	}
}

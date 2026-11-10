package app

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// expiries returns the expires_at of every stored image, keyed by whether the
// row has an owner.
func storedExpiries(t *testing.T, a *App) (guest, owned []int64) {
	t.Helper()
	rows, err := a.db.Query(`SELECT expires_at, user_id IS NULL FROM images`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	for rows.Next() {
		var e int64
		var isGuest bool
		if err := rows.Scan(&e, &isGuest); err != nil {
			t.Fatal(err)
		}
		if isGuest {
			guest = append(guest, e)
		} else {
			owned = append(owned, e)
		}
	}
	return
}

func near(got int64, days int) bool {
	want := time.Now().Add(time.Duration(days) * 24 * time.Hour).Unix()
	return got > want-120 && got < want+120
}

// New guest uploads expire after the default lifetime, a shorter choice is
// kept, a longer one is capped, members are untouched, 0 turns it off.
func TestGuestUploadsExpireByDefault(t *testing.T) {
	a, h := guestApp(t)
	ctx := context.Background()
	if a.guestTTLDays(ctx) != 30 {
		t.Fatalf("default ttl %d", a.guestTTLDays(ctx))
	}
	if res := batch(t, h, "203.0.113.40", "", 1, nil); res.code != 200 {
		t.Fatalf("upload: %d %q", res.code, res.err)
	}
	if res := batch(t, h, "203.0.113.41", "", 1, map[string]string{"expire": "1d"}); res.code != 200 {
		t.Fatalf("upload 1d: %d %q", res.code, res.err)
	}
	if res := batch(t, h, "203.0.113.42", "", 1, map[string]string{"expire": "90d"}); res.code != 200 {
		t.Fatalf("upload 90d: %d %q", res.code, res.err)
	}
	guest, _ := storedExpiries(t, a)
	if len(guest) != 3 || !near(guest[0], 30) || !near(guest[1], 1) || !near(guest[2], 30) {
		t.Fatalf("guest expiries (want 30d, 1d, 30d cap): %v", guest)
	}

	_, key := member(t, a, "ttl@example.com")
	if res := batch(t, h, "203.0.113.43", key, 1, nil); res.code != 200 {
		t.Fatalf("member upload: %d %q", res.code, res.err)
	}
	if _, owned := storedExpiries(t, a); len(owned) != 1 || owned[0] != 0 {
		t.Fatalf("a member's upload must not expire by default: %v", owned)
	}

	a.db.SetSetting(ctx, "guest_ttl_days", "0")
	if res := batch(t, h, "203.0.113.44", "", 1, nil); res.code != 200 {
		t.Fatalf("upload ttl off: %d %q", res.code, res.err)
	}
	if guest, _ = storedExpiries(t, a); len(guest) != 4 || guest[3] != 0 {
		t.Fatalf("with the setting at 0 nothing expires: %v", guest)
	}
}

// The upload form tells guests (only) how long files stay.
func TestGuestTTLNotice(t *testing.T) {
	a, h := guestApp(t)
	get := func(cookie string) string {
		req := httptest.NewRequest("GET", "/", nil)
		if cookie != "" {
			req.Header.Set("Cookie", cookie)
		}
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec.Body.String()
	}
	if body := get(""); !strings.Contains(body, `id="guest-ttl"`) || !strings.Contains(body, "30 days") {
		t.Error("guests are not told their files expire")
	}
	a.db.SetSetting(context.Background(), "guest_ttl_days", "0")
	if strings.Contains(get(""), `id="guest-ttl"`) {
		t.Error("notice shown although nothing expires")
	}
}

// The API opts in to being embedded by pages under COEP: require-corp.
func TestAPICrossOriginResourcePolicy(t *testing.T) {
	_, h := guestApp(t)
	for _, path := range []string{"/api/v1/me", "/api/v1/images"} {
		for _, method := range []string{"GET", "OPTIONS"} {
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, httptest.NewRequest(method, path, nil))
			if got := rec.Header().Get("Cross-Origin-Resource-Policy"); got != "cross-origin" {
				t.Errorf("%s %s (%d): CORP %q", method, path, rec.Code, got)
			}
		}
	}
	_ = http.StatusOK
}

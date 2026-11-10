package app

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// expiries returns the expires_at of every stored image, keyed by whether the
// row has an owner.
func storedExpiries(t *testing.T, a *App) (guest, owned []int64) {
	t.Helper()
	rows, err := a.db.Query(`SELECT expires_at, user_id IS NULL FROM images ORDER BY rowid`)
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

// Guests are only offered lifetimes the server will honour; members see all.
func TestExpireOptionsMatchWhoIsUploading(t *testing.T) {
	a, h := guestApp(t)
	opts := func(body string) (all []string) {
		for _, v := range []string{`value="">Never`, `value="1d"`, `value="7d"`, `value="30d"`, `value="90d"`} {
			if strings.Contains(body, v) {
				all = append(all, v)
			}
		}
		return
	}
	guest := hostGet(h, "pixel.example", "/").Body.String()
	if got := strings.Join(opts(guest), " "); got != `value="1d" value="7d"` || !strings.Contains(guest, `value="">30 days`) {
		t.Errorf("guest options (30-day cap): %q", got)
	}
	u, _ := member(t, a, "opts@example.com")
	mem := hostGet(h, "pixel.example", "/", sessionFor(t, a, u.ID)).Body.String()
	if got := strings.Join(opts(mem), " "); got != `value="">Never value="1d" value="7d" value="30d" value="90d"` {
		t.Errorf("member options: %q", got)
	}
	a.db.SetSetting(context.Background(), "guest_ttl_days", "0")
	if got := strings.Join(opts(hostGet(h, "pixel.example", "/").Body.String()), " "); !strings.Contains(got, `Never`) || !strings.Contains(got, `90d`) {
		t.Errorf("guest options with no cap: %q", got)
	}
}

package app

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestParseQuotaGB(t *testing.T) {
	for in, want := range map[string]int64{"10": 10 << 30, " 2.5 ": 2560 << 20, "2,5": 2560 << 20, "10 GB": 10 << 30, "10gb": 10 << 30, "0": 0} {
		if got, ok := parseQuotaGB(in); !ok || got != want {
			t.Errorf("parseQuotaGB(%q) = %d, %v; want %d", in, got, ok, want)
		}
	}
	for _, in := range []string{"", "ten", "-1", "10.240,5", "1e9", "NaN"} {
		if _, ok := parseQuotaGB(in); ok {
			t.Errorf("parseQuotaGB(%q) should be refused", in)
		}
	}
}

// An admin changing a quota sees the result, and a bad value says so
// instead of being skipped without a word (the 3.5.1 bug).
func TestAdminQuotaSaysWhatHappened(t *testing.T) {
	a, h := ssoApp(t)
	ctx := context.Background()
	admin, _ := member(t, a, "boss@example.com")
	admin.Role = "admin"
	a.db.UpdateUser(ctx, admin)
	tim, _ := member(t, a, "tim@example.com")

	tok := randomToken(32)
	a.db.InsertSession(ctx, hashToken(tok), admin.ID, time.Hour, "", "")
	post := func(form url.Values) *httptest.ResponseRecorder {
		req := httptest.NewRequest("POST", "/admin/users/"+strconv.FormatInt(tim.ID, 10), strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("Sec-Fetch-Site", "same-origin")
		req.AddCookie(&http.Cookie{Name: sessionCookie, Value: tok})
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec
	}
	cookie := &http.Cookie{Name: sessionCookie, Value: tok}

	rec := post(url.Values{"action": {"quota"}, "quota_gb": {"10"}, "back": {"/admin/users?q=tim&page=1"}})
	loc := rec.Header().Get("Location")
	if rec.Code != http.StatusSeeOther || !strings.Contains(loc, "done=quota") || !strings.Contains(loc, "q=tim") {
		t.Fatalf("save: %d %s", rec.Code, loc)
	}
	if u, _ := a.db.UserByID(ctx, tim.ID); u.QuotaBytes != 10<<30 {
		t.Fatalf("quota is %d", u.QuotaBytes)
	}
	if body := fetch(h, loc, cookie).Body.String(); !strings.Contains(body, "Storage quota for tim@example.com is now 10.0 GB") {
		t.Errorf("no success message on %s", loc)
	}

	rec = post(url.Values{"action": {"quota"}, "quota_gb": {"10.240,5"}, "back": {"/admin/users"}})
	loc = rec.Header().Get("Location")
	if !strings.Contains(loc, "error=bad-quota") {
		t.Fatalf("bad value not reported: %s", loc)
	}
	if u, _ := a.db.UserByID(ctx, tim.ID); u.QuotaBytes != 10<<30 {
		t.Fatalf("bad value changed the quota to %d", u.QuotaBytes)
	}
	if body := fetch(h, loc, cookie).Body.String(); !strings.Contains(body, "not changed") {
		t.Error("no error message for a bad quota")
	}

	// Old forms still send megabytes.
	post(url.Values{"action": {"quota"}, "quota_mb": {"2048"}, "back": {"/admin/users"}})
	if u, _ := a.db.UserByID(ctx, tim.ID); u.QuotaBytes != 2<<30 {
		t.Fatalf("quota_mb: %d", u.QuotaBytes)
	}

	// Changing your own role is refused, with a reason.
	req := httptest.NewRequest("POST", "/admin/users/"+strconv.FormatInt(admin.ID, 10), strings.NewReader("action=role&role=user&back=/admin/users"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Sec-Fetch-Site", "same-origin")
	req.AddCookie(cookie)
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if !strings.Contains(rec.Header().Get("Location"), "error=self") {
		t.Fatalf("self change: %s", rec.Header().Get("Location"))
	}
}

package app

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"
)

// Each window lives for its own length: the cleaner frees a minute's window
// after a few minutes, but keeps a day's window for the whole day.
func TestLimiterKeepsEachWindowForItsOwnLength(t *testing.T) {
	l := &limiter{m: map[string]*window{}} // no background cleaner: the test calls sweep itself
	t0 := time.Now()
	for i := 0; i < 10; i++ {
		if !l.allowAt("ai-day:upscale:1", 10, 24*time.Hour, t0) {
			t.Fatalf("use %d of 10 refused", i+1)
		}
	}
	if l.allowAt("ai-day:upscale:1", 10, 24*time.Hour, t0) {
		t.Fatal("the 11th use of the day was allowed")
	}
	l.allowAt("upload:192.0.2.1", 30, time.Minute, t0)

	// Two hours later the cleaner has run: the minute is freed, the day is not.
	l.sweep(t0.Add(2 * time.Hour))
	if _, ok := l.m["upload:192.0.2.1"]; ok {
		t.Error("a finished one-minute window was kept: memory would grow")
	}
	if l.allowAt("ai-day:upscale:1", 10, 24*time.Hour, t0.Add(2*time.Hour)) {
		t.Fatal("two hours later the daily limit was full again")
	}
	l.sweep(t0.Add(23 * time.Hour))
	if l.allowAt("ai-day:upscale:1", 10, 24*time.Hour, t0.Add(23*time.Hour)) {
		t.Fatal("23 hours later the daily limit was full again")
	}

	// After a day the window has run out: the cleaner frees it, and a new day starts.
	l.sweep(t0.Add(24*time.Hour + time.Second))
	if len(l.m) != 0 {
		t.Errorf("windows left after a day: %d", len(l.m))
	}
	if !l.allowAt("ai-day:upscale:1", 10, 24*time.Hour, t0.Add(24*time.Hour+time.Second)) {
		t.Fatal("a new day did not allow the tool again")
	}
}

// A window that runs out is replaced on the next call even if the cleaner
// has not run yet.
func TestLimiterStartsAgainAfterTheWindow(t *testing.T) {
	l := &limiter{m: map[string]*window{}}
	t0 := time.Now()
	if !l.allowAt("k", 1, time.Minute, t0) || l.allowAt("k", 1, time.Minute, t0.Add(30*time.Second)) {
		t.Fatal("one per minute: first allowed, second refused")
	}
	if !l.allowAt("k", 1, time.Minute, t0.Add(61*time.Second)) {
		t.Fatal("the next minute was refused")
	}
	if !l.allowAt("off", 0, time.Minute, t0) || !l.allowAt("off", 0, time.Minute, t0) {
		t.Fatal("max 0 means no limit")
	}
}

// Through the real handler: a member who used up a tool's daily share is
// still refused two hours later (the cleaner has run), in their own language,
// and allowed again once the day is over.
func TestAIToolDailyLimitLastsADay(t *testing.T) {
	a, h := ssoApp(t)
	a.cfg.RembgURL = "http://127.0.0.1:1" // never reached: every call below stops before the AI service
	u, key := member(t, a, "daily@example.com")
	n := 0
	post := func(lang string) (int, string) {
		n++ // a new address each time, so the per-minute limit (8 per address) stays out of the way
		path := "/api/v1/tools/upscale"
		if lang != "" {
			path += "?lang=" + lang
		}
		req := httptest.NewRequest("POST", path, nil)
		req.RemoteAddr = fmt.Sprintf("198.51.100.%d:1234", n)
		req.Header.Set("Authorization", "Bearer "+key)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		var r apiResp
		json.Unmarshal(rec.Body.Bytes(), &r)
		return rec.Code, r.Error
	}
	perDay := aiTools["upscale"].perDay
	for i := 0; i < perDay; i++ {
		if code, msg := post(""); code != http.StatusBadRequest { // counted, then refused for having no image
			t.Fatalf("use %d: %d %q", i+1, code, msg)
		}
	}
	if code, msg := post(""); code != http.StatusTooManyRequests || !strings.Contains(msg, "today's "+strconv.Itoa(perDay)) {
		t.Fatalf("over the daily share: %d %q", code, msg)
	}
	a.limits.sweep(time.Now().Add(2 * time.Hour))
	if code, msg := post("id"); code != http.StatusTooManyRequests || !strings.Contains(msg, "Coba lagi besok") {
		t.Fatalf("two hours later, in Indonesian: %d %q", code, msg)
	}
	// The day is over: move the window's start back a day.
	a.limits.mu.Lock()
	a.limits.m["ai-day:upscale:"+strconv.FormatInt(u.ID, 10)].start = time.Now().Add(-24*time.Hour - time.Minute)
	a.limits.mu.Unlock()
	if code, msg := post(""); code != http.StatusBadRequest {
		t.Fatalf("the next day: %d %q", code, msg)
	}
}

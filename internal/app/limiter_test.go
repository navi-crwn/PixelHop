package app

import (
	"bytes"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
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

// give takes back one use of the window it was counted in, never below zero, never in a later window,
// and racing allow/give pairs never let more than max through at once.
func TestLimiterGive(t *testing.T) {
	l := &limiter{m: map[string]*window{}}
	t0 := time.Now()
	l.give("none", t0) // no window: nothing happens
	l.allowAt("k", 2, time.Hour, t0)
	l.give("k", t0)
	l.give("k", t0)
	if l.m["k"].n != 0 {
		t.Fatalf("n=%d after giving back more than was taken", l.m["k"].n)
	}
	// A window that started after the use was taken is not touched.
	l.allowAt("k", 2, time.Hour, t0.Add(2*time.Hour))
	l.give("k", t0)
	if l.m["k"].n != 1 {
		t.Fatalf("a later window lost a use: n=%d", l.m["k"].n)
	}
	// Racing: 50 callers take, half give back; the count never passes max at any moment.
	r := &limiter{m: map[string]*window{}}
	var mu sync.Mutex
	inUse, most := 0, 0
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			at := time.Now()
			if !r.allowAt("race", 5, time.Hour, at) {
				return
			}
			mu.Lock()
			inUse++
			most = max(most, inUse)
			mu.Unlock()
			if i%2 == 0 {
				mu.Lock()
				inUse--
				mu.Unlock()
				r.give("race", at)
			}
		}(i)
	}
	wg.Wait()
	if most > 5 || r.m["race"].n > 5 || r.m["race"].n < 0 {
		t.Fatalf("over the limit: most=%d n=%d", most, r.m["race"].n)
	}
}

// Through the real handler, with a stand-in AI service: a call counts against the day only when the
// member got a result. Calls refused before the service (no image, too large) or failed by it (down,
// error, busy) cost nothing; ten results use up the day, which stays used two hours later (the cleaner
// has run) and comes back after 24 hours. The refusal is in the member's language.
func TestAIToolDailyLimitLastsADay(t *testing.T) {
	a, h := ssoApp(t)
	failing := true
	ai := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if failing {
			http.Error(w, "model crashed", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "image/png")
		w.Write(testPNG(4, 4))
	}))
	defer ai.Close()
	a.cfg.RembgURL = ai.URL
	u, key := member(t, a, "daily@example.com")
	n := 0
	post := func(lang string, img []byte) (int, string) {
		n++ // a new address each time, so the per-minute limit (8 per address) stays out of the way
		path := "/api/v1/tools/upscale"
		if lang != "" {
			path += "?lang=" + lang
		}
		var body bytes.Buffer
		mw := multipart.NewWriter(&body)
		if img != nil {
			fw, _ := mw.CreateFormFile("file", "a.png")
			fw.Write(img)
		}
		mw.Close()
		req := httptest.NewRequest("POST", path, &body)
		req.Header.Set("Content-Type", mw.FormDataContentType())
		req.RemoteAddr = fmt.Sprintf("198.51.100.%d:1234", n%250+1)
		req.Header.Set("Authorization", "Bearer "+key)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		var r apiResp
		json.Unmarshal(rec.Body.Bytes(), &r)
		return rec.Code, r.Error
	}
	good := testPNG(40, 30)
	perDay := aiTools["upscale"].perDay
	for i := 0; i < perDay; i++ {
		if code, msg := post("", good); code != http.StatusBadGateway {
			t.Fatalf("failing service, call %d: %d %q", i+1, code, msg)
		}
		if code, _ := post("", nil); code != http.StatusBadRequest {
			t.Fatalf("no image: %d", code)
		}
	}
	if code, msg := post("", testPNG(1100, 1000)); code != http.StatusRequestEntityTooLarge {
		t.Fatalf("too large: %d %q", code, msg)
	}
	ai.Close() // the service goes away entirely
	if code, _ := post("", good); code != http.StatusBadGateway {
		t.Fatalf("service down: %d", code)
	}
	ai = httptest.NewServer(ai.Config.Handler)
	a.cfg.RembgURL = ai.URL
	failing = false
	for i := 0; i < perDay; i++ { // none of the failures cost anything: all ten results go through
		if code, msg := post("", good); code != http.StatusOK {
			t.Fatalf("result %d after failures: %d %q", i+1, code, msg)
		}
	}
	if code, msg := post("", good); code != http.StatusTooManyRequests || !strings.Contains(msg, "today's "+strconv.Itoa(perDay)) {
		t.Fatalf("over the daily share: %d %q", code, msg)
	}
	a.limits.sweep(time.Now().Add(2 * time.Hour))
	if code, msg := post("id", good); code != http.StatusTooManyRequests || !strings.Contains(msg, "Coba lagi besok") {
		t.Fatalf("two hours later, in Indonesian: %d %q", code, msg)
	}
	// The day is over: move the window's start back a day.
	a.limits.mu.Lock()
	a.limits.m["ai-day:upscale:"+strconv.FormatInt(u.ID, 10)].start = time.Now().Add(-24*time.Hour - time.Minute)
	a.limits.mu.Unlock()
	if code, msg := post("", good); code != http.StatusOK {
		t.Fatalf("the next day: %d %q", code, msg)
	}
}

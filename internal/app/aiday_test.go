package app

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/navi-crwn/pixelhop/internal/db"
)

// aiPost sends one image to an AI tool as the member with this API key, from its own address so the
// per-minute limit (8 per address) stays out of the way.
func aiPost(h http.Handler, tool, key string, ip int) *httptest.ResponseRecorder {
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	fw, _ := mw.CreateFormFile("file", "a.png")
	fw.Write(testPNG(40, 30))
	mw.Close()
	req := httptest.NewRequest("POST", "/api/v1/tools/"+tool, &body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.RemoteAddr = fmt.Sprintf("198.51.%d.%d:1234", ip/250, ip%250+1)
	req.Header.Set("Authorization", "Bearer "+key)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func aiStandIn(t *testing.T) *httptest.Server {
	t.Helper()
	ai := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		w.Write(testPNG(4, 4))
	}))
	t.Cleanup(ai.Close)
	return ai
}

// restarted is the same app after the process was stopped and started again: a new App on the database
// reopened from the same folder.
func restarted(t *testing.T, old *App) (*App, http.Handler) {
	t.Helper()
	old.db.Close()
	d, err := db.Open(old.cfg.DataDir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { d.Close() })
	a, err := New(old.cfg, d, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	return a, a.Handler()
}

// The day's share is in the database: results used before the process was restarted still count after it.
// A member who used 3 is on the 4th next, the answer says what is left, the refusal at the limit keeps the
// old shape (success false, error text) with the numbers added, and the share does not come back by restarting.
func TestAIDayShareSurvivesARestart(t *testing.T) {
	a, h := ssoApp(t)
	a.cfg.RembgURL = aiStandIn(t).URL
	_, key := member(t, a, "restart@example.com")
	perDay := aiTools["upscale"].perDay

	n := 0
	use := func(h http.Handler) *httptest.ResponseRecorder { n++; return aiPost(h, "upscale", key, n) }
	for i := 1; i <= 3; i++ {
		rec := use(h)
		if rec.Code != http.StatusOK {
			t.Fatalf("use %d: %d", i, rec.Code)
		}
		if got, want := rec.Header().Get("X-Day-Remaining"), strconv.Itoa(perDay-i); got != want {
			t.Fatalf("after use %d: X-Day-Remaining %q, want %q", i, got, want)
		}
		if rec.Header().Get("X-Day-Limit") != strconv.Itoa(perDay) {
			t.Fatalf("X-Day-Limit %q", rec.Header().Get("X-Day-Limit"))
		}
	}

	a2, h2 := restarted(t, a)
	rec := use(h2)
	if rec.Code != http.StatusOK || rec.Header().Get("X-Day-Remaining") != strconv.Itoa(perDay-4) {
		t.Fatalf("the 4th use after a restart: %d, remaining %q, want %d", rec.Code, rec.Header().Get("X-Day-Remaining"), perDay-4)
	}
	for i := 5; i <= perDay; i++ {
		if rec = use(h2); rec.Code != http.StatusOK {
			t.Fatalf("use %d: %d", i, rec.Code)
		}
	}
	if rec.Header().Get("X-Day-Remaining") != "0" {
		t.Fatalf("last use: remaining %q", rec.Header().Get("X-Day-Remaining"))
	}

	// Restart again at the limit: still used up, and the refusal tells how much is left (none).
	_, h3 := restarted(t, a2)
	rec = use(h3)
	var body map[string]any
	json.Unmarshal(rec.Body.Bytes(), &body)
	if rec.Code != http.StatusTooManyRequests || body["success"] != false || body["error"] == "" {
		t.Fatalf("over the share after a restart: %d %s", rec.Code, rec.Body.String())
	}
	if body["remaining"] != float64(0) || body["limit"] != float64(perDay) || body["resets_at"] == nil {
		t.Fatalf("refusal lacks the share numbers: %s", rec.Body.String())
	}

	// GET /api/v1/me carries the same numbers for every tool.
	me := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/v1/me", nil)
	req.Header.Set("Authorization", "Bearer "+key)
	h3.ServeHTTP(me, req)
	var got struct {
		Data struct {
			AIDay map[string]struct{ Limit, Used, Remaining int } `json:"ai_day"`
		} `json:"data"`
	}
	json.Unmarshal(me.Body.Bytes(), &got)
	if up := got.Data.AIDay["upscale"]; up.Limit != perDay || up.Used != perDay || up.Remaining != 0 {
		t.Fatalf("/me ai_day upscale: %+v (%s)", up, me.Body.String())
	}
	if f := got.Data.AIDay["faces"]; f.Used != 0 || f.Remaining != aiTools["faces"].perDay {
		t.Fatalf("/me ai_day faces: %+v", f)
	}
}

// Racing requests cannot go over the share: 40 at the same moment against a share of 10 give exactly 10 results.
func TestAIDayShareHoldsUnderRacingRequests(t *testing.T) {
	a, h := ssoApp(t)
	a.cfg.RembgURL = aiStandIn(t).URL
	_, key := member(t, a, "race@example.com")
	perDay := aiTools["upscale"].perDay
	var ok, refused atomic.Int32
	var wg sync.WaitGroup
	for i := 0; i < 4*perDay; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			switch aiPost(h, "upscale", key, i).Code {
			case http.StatusOK:
				ok.Add(1)
			case http.StatusTooManyRequests:
				refused.Add(1)
			}
		}(i)
	}
	wg.Wait()
	if int(ok.Load()) != perDay || int(refused.Load()) != 3*perDay {
		t.Fatalf("results %d (want %d), refused %d (want %d)", ok.Load(), perDay, refused.Load(), 3*perDay)
	}
}

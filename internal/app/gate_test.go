package app

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/navi-crwn/pixelhop/internal/db"
)

func TestContentWarningGateImage(t *testing.T) {
	a, h := ssoApp(t)

	im := &db.Image{
		ID:             "nsfw123",
		Title:          "Adult Media",
		Mime:           "image/jpeg",
		Ext:            "jpg",
		Size:           1024,
		Width:          800,
		Height:         600,
		SHA256:         "nsfw123hash",
		Backend:        "local",
		KeyFull:        "nsfw123.jpg",
		KeyThumb:       "nsfw123_t.jpg",
		IP:             "192.0.2.1",
		ContentWarning: "adult",
		WarningReason:  "Contains adult themes",
		Listed:         true,
	}
	if err := a.db.InsertImage(context.Background(), im); err != nil {
		t.Fatal(err)
	}

	// 1. Visit view page without cookie -> gate is present
	req := httptest.NewRequest("GET", "/nsfw123", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "18+ Adult Content") {
		t.Fatalf("expected gate badge in body")
	}
	if !strings.Contains(body, "Contains adult themes") {
		t.Fatalf("expected creator warning note in body")
	}

	// 2. Direct hotlink with external referer redirects to view page
	reqImg := httptest.NewRequest("GET", "/i/nsfw123.jpg", nil)
	reqImg.Header.Set("Referer", "https://external.example.com/blog")
	recImg := httptest.NewRecorder()
	h.ServeHTTP(recImg, reqImg)
	if recImg.Code != http.StatusSeeOther {
		t.Fatalf("expected 303 redirect for external referer, got %d", recImg.Code)
	}
	if loc := recImg.Header().Get("Location"); !strings.Contains(loc, "/nsfw123") {
		t.Fatalf("expected redirect to /nsfw123, got %q", loc)
	}

	// 3. Confirm gate via POST /gate/nsfw123
	form := url.Values{"remember": {"1"}}
	reqGate := httptest.NewRequest("POST", "/gate/nsfw123", strings.NewReader(form.Encode()))
	reqGate.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	recGate := httptest.NewRecorder()
	h.ServeHTTP(recGate, reqGate)
	if recGate.Code != http.StatusSeeOther {
		t.Fatalf("expected 303 redirect after gate confirmation, got %d", recGate.Code)
	}
	cookie := recGate.Result().Cookies()
	var gateCookie *http.Cookie
	for _, c := range cookie {
		if c.Name == "hop_gate_ph_nsfw123" {
			gateCookie = c
			break
		}
	}
	if gateCookie == nil || gateCookie.Value != "1" {
		t.Fatalf("expected hop_gate_ph_nsfw123 cookie to be set")
	}

	// 4. Visit view page with cookie -> gate overlay not shown
	reqPassed := httptest.NewRequest("GET", "/nsfw123", nil)
	reqPassed.AddCookie(gateCookie)
	recPassed := httptest.NewRecorder()
	h.ServeHTTP(recPassed, reqPassed)
	if recPassed.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", recPassed.Code)
	}
	bodyPassed := recPassed.Body.String()
	if strings.Contains(bodyPassed, "Age Verification Required") {
		t.Fatalf("expected gate overlay to not appear after cookie verification")
	}

	// 5. Explore page excludes adult content
	reqExplore := httptest.NewRequest("GET", "/explore", nil)
	recExplore := httptest.NewRecorder()
	h.ServeHTTP(recExplore, reqExplore)
	if strings.Contains(recExplore.Body.String(), "/nsfw123") {
		t.Fatalf("adult content should not be listed on explore")
	}
}

func TestContentWarningModerationActions(t *testing.T) {
	a, h := ssoApp(t)

	im := &db.Image{
		ID:             "modpic",
		Title:          "Mod Pic",
		Mime:           "image/jpeg",
		Ext:            "jpg",
		Size:           100,
		Width:          10,
		Height:         10,
		SHA256:         "modpicsha",
		Backend:        "local",
		KeyFull:        "modpic.jpg",
		KeyThumb:       "modpic_t.jpg",
		IP:             "127.0.0.1",
		ContentWarning: "none",
	}
	if err := a.db.InsertImage(context.Background(), im); err != nil {
		t.Fatal(err)
	}

	// Mark NSFW
	res := hopPost(t, h, "/hop/content/image/modpic/actions", map[string]any{
		"action": "mark_nsfw",
		"reason": "Explicit nudity",
	})
	if !res.OK {
		t.Fatalf("mark_nsfw failed: %s", res.Message)
	}

	updated, err := a.db.Image(context.Background(), "modpic")
	if err != nil {
		t.Fatal(err)
	}
	if updated.ContentWarning != "adult" || updated.WarningReason != "Explicit nudity" {
		t.Fatalf("expected adult warning, got %q / %q", updated.ContentWarning, updated.WarningReason)
	}

	// Mark Sensitive
	res = hopPost(t, h, "/hop/content/image/modpic/actions", map[string]any{
		"action": "mark_sensitive",
		"reason": "Contains spoilers",
	})
	if !res.OK {
		t.Fatalf("mark_sensitive failed: %s", res.Message)
	}

	updated, err = a.db.Image(context.Background(), "modpic")
	if err != nil {
		t.Fatal(err)
	}
	if updated.ContentWarning != "sensitive" || updated.WarningReason != "Contains spoilers" {
		t.Fatalf("expected sensitive warning, got %q / %q", updated.ContentWarning, updated.WarningReason)
	}

	// Unmark NSFW
	res = hopPost(t, h, "/hop/content/image/modpic/actions", map[string]any{
		"action": "unmark_nsfw",
	})
	if !res.OK {
		t.Fatalf("unmark_nsfw failed: %s", res.Message)
	}

	updated, err = a.db.Image(context.Background(), "modpic")
	if err != nil {
		t.Fatal(err)
	}
	if updated.ContentWarning != "none" {
		t.Fatalf("expected none warning, got %q", updated.ContentWarning)
	}
}

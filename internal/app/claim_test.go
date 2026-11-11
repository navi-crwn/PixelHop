package app

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type guestPic struct{ ID, Token, DeleteURL string }

// guestUpload stores n pictures as a signed-out visitor and returns what the
// browser would keep: the id and the secret delete link.
func guestUpload(t *testing.T, h http.Handler, n int) []guestPic {
	t.Helper()
	var out []guestPic
	for i := 0; i < n; i++ {
		d := guestFile(t, h, testPNG(30+i, 20+i))
		del, _ := d["delete_url"].(string)
		parts := strings.Split(del, "/")
		if del == "" || len(parts) < 3 {
			t.Fatalf("a guest upload hands out a delete link: %v", d)
		}
		out = append(out, guestPic{ID: d["id"].(string), Token: parts[len(parts)-1], DeleteURL: del})
	}
	return out
}

func guestFile(t *testing.T, h http.Handler, data []byte) map[string]any {
	t.Helper()
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	fw, _ := mw.CreateFormFile("file", "guest.png")
	fw.Write(data)
	mw.Close()
	req := httptest.NewRequest("POST", "/api/v1/upload", &body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	var r struct {
		Success bool
		Data    map[string]any
	}
	json.Unmarshal(rec.Body.Bytes(), &r)
	if !r.Success {
		t.Fatalf("guest upload: %d %s", rec.Code, rec.Body.String())
	}
	return r.Data
}

type claimAnswer struct {
	Success bool `json:"success"`
	Error   string
	Data    struct {
		Claimed []string
		Already []string
		Skipped []struct{ ID, Reason string }
	}
}

func postClaim(t *testing.T, h http.Handler, c *http.Cookie, items ...map[string]string) (*httptest.ResponseRecorder, claimAnswer) {
	t.Helper()
	body, _ := json.Marshal(map[string]any{"items": items})
	req := httptest.NewRequest("POST", "/dashboard/claim", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if c != nil {
		req.AddCookie(c)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	var out claimAnswer
	json.Unmarshal(rec.Body.Bytes(), &out)
	return rec, out
}

func item(p guestPic) map[string]string { return map[string]string{"id": p.ID, "token": p.Token} }

func ownerOf(t *testing.T, a *App, id string) (int64, bool) {
	t.Helper()
	im, err := a.db.Image(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	return im.UserID.Int64, im.UserID.Valid
}

func TestClaimHappyPath(t *testing.T) {
	a, h := ssoApp(t)
	u, _ := member(t, a, "claim@example.com")
	pics := guestUpload(t, h, 2)
	cookie := sessionFor(t, a, u.ID)

	rec, out := postClaim(t, h, cookie, item(pics[0]), item(pics[1]))
	if rec.Code != 200 || !out.Success || len(out.Data.Claimed) != 2 || len(out.Data.Skipped) != 0 {
		t.Fatalf("claim: %d %s", rec.Code, rec.Body.String())
	}
	for _, p := range pics {
		if id, ok := ownerOf(t, a, p.ID); !ok || id != u.ID {
			t.Errorf("%s was not moved into the library", p.ID)
		}
		// The secret link is spent: it no longer deletes anything.
		del := httptest.NewRecorder()
		h.ServeHTTP(del, httptest.NewRequest("POST", "/delete/"+p.ID+"/"+p.Token, nil))
		if del.Code != http.StatusForbidden {
			t.Errorf("the old delete link of %s still answers %d", p.ID, del.Code)
		}
	}
	if used, n := a.db.UserStorage(context.Background(), u.ID); n != 2 || used == 0 {
		t.Errorf("library holds %d files, %d bytes", n, used)
	}
}

func TestClaimNeedsTheToken(t *testing.T) {
	a, h := ssoApp(t)
	u, _ := member(t, a, "thief@example.com")
	pics := guestUpload(t, h, 1)
	cookie := sessionFor(t, a, u.ID)

	// No token at all, and an empty one: refused outright.
	for _, it := range []map[string]string{{"id": pics[0].ID}, {"id": pics[0].ID, "token": ""}} {
		if rec, _ := postClaim(t, h, cookie, it); rec.Code != http.StatusBadRequest {
			t.Errorf("claim without a token: %d", rec.Code)
		}
	}
	// A stolen id with a wrong token claims nothing, and looks the same as an id that does not exist.
	rec, out := postClaim(t, h, cookie, map[string]string{"id": pics[0].ID, "token": "guess"}, map[string]string{"id": "nope-aaaaaa", "token": "guess"})
	if rec.Code != 200 || len(out.Data.Claimed) != 0 || len(out.Data.Skipped) != 2 || out.Data.Skipped[0].Reason != "invalid" || out.Data.Skipped[1].Reason != "invalid" {
		t.Fatalf("wrong tokens: %s", rec.Body.String())
	}
	if _, ok := ownerOf(t, a, pics[0].ID); ok {
		t.Fatal("an image was claimed with a wrong token")
	}
	// A body that is not the expected shape, and too many items.
	req := httptest.NewRequest("POST", "/dashboard/claim", strings.NewReader("nonsense"))
	req.AddCookie(cookie)
	bad := httptest.NewRecorder()
	h.ServeHTTP(bad, req)
	if bad.Code != http.StatusBadRequest {
		t.Errorf("garbage body: %d", bad.Code)
	}
	many := make([]map[string]string, claimMaxItems+1)
	for i := range many {
		many[i] = map[string]string{"id": "x-aaaaaa", "token": "t"}
	}
	if rec, _ := postClaim(t, h, cookie, many...); rec.Code != http.StatusBadRequest {
		t.Errorf("too many items: %d", rec.Code)
	}
}

func TestClaimTwiceAndAlreadyOwned(t *testing.T) {
	a, h := ssoApp(t)
	me, _ := member(t, a, "me@example.com")
	other, _ := member(t, a, "other@example.com")
	pics := guestUpload(t, h, 2)
	mine, theirs := sessionFor(t, a, me.ID), sessionFor(t, a, other.ID)

	if _, out := postClaim(t, h, mine, item(pics[0])); len(out.Data.Claimed) != 1 {
		t.Fatalf("first claim: %+v", out)
	}
	// Again: nothing moves, and it says it was already done.
	rec, out := postClaim(t, h, mine, item(pics[0]))
	if rec.Code != 200 || len(out.Data.Claimed) != 0 || len(out.Data.Already) != 1 || out.Data.Already[0] != pics[0].ID {
		t.Fatalf("second claim: %s", rec.Body.String())
	}
	// Someone else holding the old link cannot take it from me.
	_, out = postClaim(t, h, theirs, item(pics[0]))
	if len(out.Data.Claimed) != 0 || len(out.Data.Already) != 0 || len(out.Data.Skipped) != 1 || out.Data.Skipped[0].Reason != "invalid" {
		t.Fatalf("other member: %+v", out)
	}
	if id, _ := ownerOf(t, a, pics[0].ID); id != me.ID {
		t.Fatal("the image changed hands")
	}
	// An image that already has a member owner (a normal upload) is not claimable at all.
	_, key := member(t, a, "owner@example.com")
	own := uploadFile(t, h, key, "own.png", testPNG(31, 21))["id"].(string)
	_, out = postClaim(t, h, mine, map[string]string{"id": own, "token": pics[1].Token})
	if len(out.Data.Claimed) != 0 || out.Data.Skipped[0].Reason != "invalid" {
		t.Fatalf("claimed a member's image: %+v", out)
	}
}

func TestClaimRespectsQuotaRemovalAndReports(t *testing.T) {
	a, h := ssoApp(t)
	ctx := context.Background()
	u, _ := member(t, a, "quota@example.com")
	pics := guestUpload(t, h, 3)
	cookie := sessionFor(t, a, u.ID)

	a.db.ExecContext(ctx, `UPDATE images SET hidden=1, hidden_reason='moderation' WHERE id=?`, pics[0].ID)
	a.db.ExecContext(ctx, `INSERT INTO reports (image_id, reason, created_at) VALUES (?, 'spam', 1)`, pics[1].ID)
	_, out := postClaim(t, h, cookie, item(pics[0]), item(pics[1]))
	if len(out.Data.Claimed) != 0 || len(out.Data.Skipped) != 2 || out.Data.Skipped[0].Reason != "unavailable" || out.Data.Skipped[1].Reason != "unavailable" {
		t.Fatalf("removed or reported: %+v", out)
	}

	u.QuotaBytes = 1
	a.db.UpdateUser(ctx, u)
	cookie = sessionFor(t, a, u.ID)
	_, out = postClaim(t, h, cookie, item(pics[2]))
	if len(out.Data.Claimed) != 0 || len(out.Data.Skipped) != 1 || out.Data.Skipped[0].Reason != "quota" {
		t.Fatalf("over quota: %+v", out)
	}
	if _, ok := ownerOf(t, a, pics[2].ID); ok {
		t.Fatal("claimed past the quota")
	}
}

func TestClaimNeedsSignIn(t *testing.T) {
	_, h := ssoApp(t)
	pics := guestUpload(t, h, 1)
	rec, _ := postClaim(t, h, nil, item(pics[0]))
	if rec.Code != http.StatusSeeOther || !strings.HasPrefix(rec.Header().Get("Location"), "/login") {
		t.Fatalf("signed out: %d %s", rec.Code, rec.Header().Get("Location"))
	}
}

func TestClaimIsRateLimited(t *testing.T) {
	a, h := ssoApp(t)
	u, _ := member(t, a, "rate@example.com")
	cookie := sessionFor(t, a, u.ID)
	last := 0
	for i := 0; i < 12; i++ {
		rec, _ := postClaim(t, h, cookie, map[string]string{"id": "x-aaaaaa", "token": "t"})
		last = rec.Code
		if i < 10 && rec.Code == http.StatusTooManyRequests {
			t.Fatalf("limited too early, at call %d", i+1)
		}
	}
	if last != http.StatusTooManyRequests {
		t.Fatalf("the 12th claim in an hour was answered %d", last)
	}
}

// A token only ever travels in the request body: it is in no URL and no log line.
func TestClaimNeverLogsOrRoutesATokenInTheURL(t *testing.T) {
	a, h := ssoApp(t)
	var logs bytes.Buffer
	a.log = slog.New(slog.NewTextHandler(&logs, nil))
	u, _ := member(t, a, "log@example.com")
	pics := guestUpload(t, h, 1)
	logs.Reset()
	rec, out := postClaim(t, h, sessionFor(t, a, u.ID), item(pics[0]))
	if rec.Code != 200 || len(out.Data.Claimed) != 1 {
		t.Fatalf("claim: %s", rec.Body.String())
	}
	if !strings.Contains(logs.String(), "/dashboard/claim") {
		t.Fatalf("expected the request in the log: %q", logs.String())
	}
	if strings.Contains(logs.String(), pics[0].Token) || strings.Contains(rec.Body.String(), pics[0].Token) {
		t.Fatal("the token leaked into the log or the answer")
	}
	// GET is not a way to claim.
	if g := fetch(h, "/dashboard/claim?id="+pics[0].ID+"&token="+pics[0].Token, sessionFor(t, a, u.ID)); g.Code == 200 {
		t.Errorf("claim over GET answered %d", g.Code)
	}
}

// The notice is on the pages a member lands on, hidden until the browser has something to claim.
func TestClaimNoticeIsOnMemberPages(t *testing.T) {
	a, h := ssoApp(t)
	u, _ := member(t, a, "notice@example.com")
	cookie := sessionFor(t, a, u.ID)
	for _, path := range []string{"/dashboard", "/"} {
		body := fetch(h, path, cookie).Body.String()
		if !strings.Contains(body, `id="claim-notice"`) || !strings.Contains(body, `data-claim-url="/dashboard/claim"`) || !strings.Contains(body, "hidden") {
			t.Errorf("%s has no claim notice", path)
		}
	}
	if strings.Contains(fetch(h, "/").Body.String(), `id="claim-notice"`) {
		t.Error("a signed-out visitor sees the claim notice")
	}
}

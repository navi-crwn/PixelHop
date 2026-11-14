package app

import (
	"context"
	"database/sql"
	"net/http"

	"github.com/navi-crwn/pixelhop/internal/db"
	"testing"
)

// When the question "who else points at this file?" cannot be answered, the file is kept; a dropped
// request does not cut a removal short.
func TestDeleteKeepsASharedFileWhenTheCountFails(t *testing.T) {
	a, _ := ssoApp(t)
	ua, _ := member(t, a, "count-a@example.com")
	ub, _ := member(t, a, "count-b@example.com")
	first, _ := sharedObject(t, a, ua, ub, "cf-1", "cf-2")

	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := a.db.CountKeyRefs(cancelled, first.KeyFull); err == nil {
		t.Fatal("a failed count was reported as a number")
	}
	if err := a.deleteImageObjects(cancelled, first); err == nil {
		t.Fatal("objects deleted although the count failed")
	}
	if !hasObject(a, first.KeyFull) || !hasObject(a, first.KeyThumb) {
		t.Fatal("the shared file was deleted when the count failed")
	}
	// removeImage ignores the cancellation and finishes: row gone, the other owner's file kept.
	if err := a.removeImage(cancelled, first); err != nil {
		t.Fatalf("removal cut short by a cancelled request: %v", err)
	}
	if _, err := a.db.Image(context.Background(), first.ID); err == nil {
		t.Fatal("row left behind")
	}
	if !hasObject(a, first.KeyFull) {
		t.Fatal("removing one row took the other's file")
	}
}

// The first link by e-mail also ends the API keys and webhooks of that row (nobody proved the e-mail), but a
// later sign-in of the linked member leaves the keys they make afterwards alone.
func TestLinkingByEmailRevokesKeysAndWebhooks(t *testing.T) {
	a, h := ssoApp(t)
	ctx := t.Context()
	old, key := member(t, a, "ivan@example.com")
	if err := a.db.InsertWebhook(ctx, &db.Webhook{UserID: old.ID, URL: "https://example.com/hook", Secret: "s", Events: "image.uploaded"}); err != nil {
		t.Fatal(err)
	}
	c, state := start(t, h)
	if rec := callback(h, c, signTicket(ticketFor(state), testSecret)); rec.Code != http.StatusSeeOther {
		t.Fatalf("callback %d", rec.Code)
	}
	if keys, _ := a.db.APIKeys(ctx, old.ID); len(keys) != 0 {
		t.Fatalf("API keys survived the link: %d", len(keys))
	}
	if hooks, _ := a.db.Webhooks(ctx, old.ID); len(hooks) != 0 {
		t.Fatalf("webhooks survived the link: %d", len(hooks))
	}
	if r, _ := call(t, h, "GET", "/api/v1/me", key, nil); r.Code != http.StatusUnauthorized {
		t.Fatalf("the old key still works: %d", r.Code)
	}
	// Keys made after the link are the member's own and stay across later sign-ins.
	k2 := "ph_" + randomToken(24)
	a.db.InsertAPIKey(ctx, old.ID, "new", k2[:9], hashToken(k2))
	c2, st2 := start(t, h)
	callback(h, c2, signTicket(ticketFor(st2), testSecret))
	if keys, _ := a.db.APIKeys(ctx, old.ID); len(keys) != 1 {
		t.Fatalf("a later sign-in revoked the member's new key: %d", len(keys))
	}
}

// On S3 with a public address, a file shared by several rows is served by the app: the redirect would carry
// the first uploader's link id in its address.
func TestSharedS3FileIsNotRedirectedToAnotherUploadersAddress(t *testing.T) {
	a := s3App(t)
	h := a.Handler()
	ctx := t.Context()
	ua, _ := member(t, a, "s3share-a@example.com")
	ub, _ := member(t, a, "s3share-b@example.com")
	mk := func(id string, uid int64, key string) {
		im := &db.Image{ID: id, UserID: sql.NullInt64{Int64: uid, Valid: true}, Mime: "image/jpeg", Ext: "jpg", Backend: "s3", KeyFull: key, KeyThumb: key + "_t"}
		if err := a.db.InsertImage(ctx, im); err != nil {
			t.Fatal(err)
		}
	}
	mk("alone", ua.ID, "2026/11/14/alone.jpg")
	if rec := fetch(h, "/i/alone.jpg"); rec.Code != http.StatusFound {
		t.Fatalf("a file only one row stores should still redirect: %d", rec.Code)
	}
	mk("first", ua.ID, "2026/11/14/first.jpg")
	mk("second", ub.ID, "2026/11/14/first.jpg")
	for _, id := range []string{"first", "second"} {
		rec := fetch(h, "/i/"+id+".jpg")
		if rec.Code != http.StatusOK || rec.Header().Get("Location") != "" || rec.Body.String() != "synthetic" {
			t.Fatalf("/i/%s: %d %q location=%q, want the app to serve it", id, rec.Code, rec.Body.String(), rec.Header().Get("Location"))
		}
	}
}

// A row whose file is gone is not a source to share: the new upload stores its own file and works.
func TestUploadDoesNotShareAFileThatIsGone(t *testing.T) {
	a, h := ssoApp(t)
	ctx := t.Context()
	_, ka := member(t, a, "gone-a@example.com")
	_, kb := member(t, a, "gone-b@example.com")
	png := testPNG(77, 55)
	first, _, _ := uploadAs(t, h, ka, png)
	im, _ := a.db.Image(ctx, first)
	if err := a.store.Delete(ctx, im.KeyFull); err != nil {
		t.Fatal(err)
	}
	second, _, _ := uploadAs(t, h, kb, png)
	if keyOf(t, a, second) == im.KeyFull {
		t.Fatal("the upload was pointed at a file that no longer exists")
	}
	if rec := fetch(h, "/i/"+second+".png"); rec.Code != 200 {
		t.Fatalf("the new upload does not open: %d", rec.Code)
	}
}

package app

import (
	"context"
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

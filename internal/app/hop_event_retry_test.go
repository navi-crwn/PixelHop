package app

import (
	"context"
	"testing"
)

// A transiently invalid signed event must remain retryable with the same id.
func TestHopEventFailureDoesNotConsumeID(t *testing.T) {
	a, h := ssoApp(t)
	u := linked(t, a, "retry-event-user", "retry-event@example.com")
	ctx := context.Background()

	rec, out := hopCall(t, h, "helink", "POST", "/hop/events", map[string]any{
		"id": "event-retry-1", "type": "account.status", "data": "not-an-object",
	})
	if rec.Code != 400 || out.OK {
		t.Fatalf("malformed event: status=%d body=%s", rec.Code, rec.Body.String())
	}
	hopPost(t, h, "/hop/events", map[string]any{
		"id": "event-retry-1", "type": "account.status", "data": map[string]any{"hop_id": u.HelinkID.String, "status": "locked"},
	})
	got, err := a.db.UserByID(ctx, u.ID)
	if err != nil || got.Status != "locked" {
		t.Fatalf("same-id retry did not apply status: got=%+v err=%v", got, err)
	}
	// A delivery that succeeded is still handled only once.
	_, out = hopCall(t, h, "helink", "POST", "/hop/events", map[string]any{
		"id": "event-retry-1", "type": "account.status", "data": map[string]any{"hop_id": u.HelinkID.String, "status": "locked"},
	})
	if !out.OK {
		t.Fatalf("duplicate delivery: %+v", out)
	}
}

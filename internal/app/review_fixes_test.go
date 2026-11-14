package app

import (
	"context"
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

package app

import (
	"context"
	"net/http"
	"strings"
	"testing"
)

// The note a member writes "for the admin" when uploading with a ticket used to be saved as the album's
// description, so everyone with the link read it on the album page, and "Copy for chat" carried it too.
// It belongs to the owner and the moderators only (dashboard test 2026-11-08, finding #9).
func TestTicketNoteIsNotOnTheSharedPage(t *testing.T) {
	a, h := linkApp(t)
	ctx := context.Background()
	owner, key := member(t, a, "ticket@example.com")
	const note = "paid twice, order 4471, please refund"
	if res := batch(t, h, "", key, 1, map[string]string{"album": "force", "title": "TRX-20261108-0042", "note": note}); res.code != 200 || res.images != 1 {
		t.Fatalf("upload: %d, %d images, %q", res.code, res.images, res.err)
	}
	rows, err := a.db.Albums(ctx, owner.ID)
	if err != nil || len(rows) != 1 {
		t.Fatalf("albums: %v, %d", err, len(rows))
	}
	al, err := a.db.Album(ctx, rows[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	if al.AdminNote != note || al.Description != "" || al.Title != "TRX-20261108-0042" {
		t.Fatalf("album saved as note=%q description=%q title=%q", al.AdminNote, al.Description, al.Title)
	}

	// A visitor on the link host: the ticket is the title, the note is nowhere, not even in the copy-for-chat data.
	if page := hostGet(h, "p.example", "/a/"+al.ID); page.Code != http.StatusOK || strings.Contains(page.Body.String(), "refund") {
		t.Errorf("visitor sees the admin note on the album page (status %d)", page.Code)
	}
	// Nor does another member.
	other, _ := member(t, a, "other@example.com")
	if page := fetch(h, "/a/"+al.ID, sessionFor(t, a, other.ID)); page.Code != http.StatusOK || strings.Contains(page.Body.String(), "refund") {
		t.Errorf("another member sees the admin note (status %d)", page.Code)
	}
	// The owner does, labelled as such, and the editor lets them change it.
	me := sessionFor(t, a, owner.ID)
	if page := fetch(h, "/a/"+al.ID, me); !strings.Contains(page.Body.String(), "refund") || !strings.Contains(page.Body.String(), "only you and the moderators") {
		t.Errorf("owner does not see the admin note (status %d)", page.Code)
	}
	if page := fetch(h, "/a/"+al.ID+"/edit", me); !strings.Contains(page.Body.String(), `id="ae-note"`) || !strings.Contains(page.Body.String(), "refund") {
		t.Errorf("album editor has no admin note field (status %d)", page.Code)
	}
	// So does an admin.
	admin, _ := member(t, a, "boss@example.com")
	admin.Role = "admin"
	a.db.UpdateUser(ctx, admin)
	if page := fetch(h, "/a/"+al.ID, sessionFor(t, a, admin.ID)); !strings.Contains(page.Body.String(), "refund") {
		t.Errorf("admin does not see the admin note (status %d)", page.Code)
	}

	// The album API keeps the two apart: the note is admin_note, the public description stays empty.
	rec, out := call(t, h, "GET", "/api/v1/albums/"+al.ID, key, nil)
	body := string(out.Data)
	if rec.Code != 200 || !strings.Contains(body, `"admin_note":"`+note+`"`) || !strings.Contains(body, `"description":""`) {
		t.Errorf("album JSON: %d %s", rec.Code, body)
	}
	// Made up front by the uploader script, with admin_note instead of description.
	rec, out = call(t, h, "POST", "/api/v1/albums", key, map[string]any{"title": "TRX-2", "admin_note": "second note"})
	if rec.Code != 200 || !strings.Contains(string(out.Data), `"admin_note":"second note"`) || !strings.Contains(string(out.Data), `"description":""`) {
		t.Errorf("create album: %d %s", rec.Code, out.Data)
	}
}

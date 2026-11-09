package app

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/navi-crwn/pixelhop/internal/db"
)

// A ticket upload of more than three files ends on /uploaded?id=…, where the uploader copies the ticket,
// the note and the links for the support chat. When the note moved out of the public description it
// vanished from that page for its owner as well. The page opens for anyone who has the ids, so the note is
// shown there to the album's owner and to admins only.
func TestTicketNoteOnTheUploadResultPage(t *testing.T) {
	a, h := ssoApp(t)
	ctx := context.Background()
	owner, key := member(t, a, "ticket-result@example.com")
	const note = "paid twice, order 4471, please refund"
	if res := batch(t, h, "", key, 4, map[string]string{"album": "force", "title": "TRX-20261109-0007", "note": note}); res.code != 200 || res.images != 4 {
		t.Fatalf("upload: %d, %d images, %q", res.code, res.images, res.err)
	}
	ims, _, err := a.db.Images(ctx, db.ImageQuery{UserID: owner.ID, Limit: 10})
	if err != nil || len(ims) != 4 {
		t.Fatalf("images: %v, %d", err, len(ims))
	}
	var ids []string
	for _, im := range ims {
		ids = append(ids, im.ID)
	}
	path := "/uploaded?id=" + strings.Join(ids, ",")

	// The uploader: the ticket as the heading, the note labelled, and the note in the text "Copy for chat" copies.
	page := fetch(h, path, sessionFor(t, a, owner.ID))
	body := page.Body.String()
	if page.Code != http.StatusOK || !strings.Contains(body, "TRX-20261109-0007") {
		t.Fatalf("owner's result page: %d, ticket shown: %v", page.Code, strings.Contains(body, "TRX-20261109-0007"))
	}
	if !strings.Contains(body, "only you and the moderators") || !strings.Contains(body, `data-report-note="`+note+`"`) {
		t.Errorf("owner's result page lacks the note (labelled: %v, in the copy-for-chat data: %v)", strings.Contains(body, "only you and the moderators"), strings.Contains(body, `data-report-note="`+note+`"`))
	}
	// An admin sees it too.
	admin, _ := member(t, a, "boss-result@example.com")
	admin.Role = "admin"
	a.db.UpdateUser(ctx, admin)
	if body := fetch(h, path, sessionFor(t, a, admin.ID)).Body.String(); !strings.Contains(body, "refund") {
		t.Error("an admin does not see the note on the result page")
	}

	// Anyone else who has the ids gets the files and the ticket, never the note.
	other, _ := member(t, a, "other-result@example.com")
	for who, page := range map[string]string{"visitor": fetch(h, path).Body.String(), "another member": fetch(h, path, sessionFor(t, a, other.ID)).Body.String()} {
		if !strings.Contains(page, "TRX-20261109-0007") {
			t.Errorf("%s: the result page did not render (no ticket heading), so this check proves nothing", who)
		}
		if strings.Contains(page, "refund") {
			t.Errorf("%s reads the note for the admin on the result page", who)
		}
	}
}

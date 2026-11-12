package db

import (
	"context"
	"database/sql"
	"testing"
	"time"
)

// Migration 17 adds images.claimed_at to a database made before it existed;
// the rows already there count as never claimed.
func TestClaimedAtMigration(t *testing.T) {
	dir := t.TempDir()
	// Make a complete database, then take it back to schema 16 without the column (later
	// migrations run again on reopen; they are written to allow that).
	d, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := d.Exec(`ALTER TABLE images DROP COLUMN claimed_at`); err != nil {
		t.Fatal(err)
	}
	if _, err := d.Exec(`DELETE FROM schema_version WHERE v>=17`); err != nil {
		t.Fatal(err)
	}
	if _, err := d.Exec(`INSERT INTO images (id, owner_token, mime, ext, size, width, height, sha256, backend, key_full, key_thumb, created_at)
		VALUES ('old-aaaaaa','h','image/png','png',1,1,1,'s','local','a','b',1)`); err != nil {
		t.Fatal(err)
	}
	d.Close()

	d, err = Open(dir)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer d.Close()
	var claimed, version int64
	if err := d.QueryRow(`SELECT claimed_at, (SELECT MAX(v) FROM schema_version) FROM images WHERE id='old-aaaaaa'`).Scan(&claimed, &version); err != nil {
		t.Fatal(err)
	}
	if claimed != 0 || version != int64(len(migrations)) || version < 17 {
		t.Fatalf("claimed_at=%d schema=%d", claimed, version)
	}
}

func guestImage(t *testing.T, d *DB, id, token string, mod func(*Image)) {
	t.Helper()
	im := &Image{ID: id, OwnerToken: token, Mime: "image/png", Ext: "png", Size: 100, Width: 1, Height: 1, SHA256: id,
		Backend: "local", KeyFull: id + ".png", KeyThumb: id + "_t.png", CreatedAt: time.Now().Unix() - 100}
	if mod != nil {
		mod(im)
	}
	if err := d.InsertImage(context.Background(), im); err != nil {
		t.Fatal(err)
	}
}

func TestClaimImages(t *testing.T) {
	ctx := context.Background()
	d, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	u := &User{Email: "a@example.com", Name: "A", Role: "user", Status: "active", QuotaBytes: 1 << 20}
	other := &User{Email: "b@example.com", Name: "B", Role: "user", Status: "active", QuotaBytes: 1 << 20}
	if err := d.InsertUser(ctx, u); err != nil {
		t.Fatal(err)
	}
	if err := d.InsertUser(ctx, other); err != nil {
		t.Fatal(err)
	}
	guestImage(t, d, "ok-aaaaaa", "h-ok", nil)
	guestImage(t, d, "ttl-aaaaaa", "h-ttl", func(im *Image) { im.ExpiresAt = im.CreatedAt + 30*86400 }) // from the guest limit
	guestImage(t, d, "mine-aaaaaa", "h-mine", func(im *Image) { im.ExpiresAt = im.CreatedAt + 3600 })   // chosen by the guest
	guestImage(t, d, "hid-aaaaaa", "h-hid", func(im *Image) { im.Hidden = true; im.HiddenReason = "moderation" })
	guestImage(t, d, "trs-aaaaaa", "h-trs", nil)
	d.TrashImage(ctx, "trs-aaaaaa")
	guestImage(t, d, "rep-aaaaaa", "h-rep", nil)
	d.Exec(`INSERT INTO reports (image_id, reason, created_at) VALUES ('rep-aaaaaa','spam',1)`)
	guestImage(t, d, "big-aaaaaa", "h-big", func(im *Image) { im.Size = 5000 })
	guestImage(t, d, "own-aaaaaa", "h-own", func(im *Image) { im.UserID = sql.NullInt64{Int64: other.ID, Valid: true} })

	items := []ClaimItem{
		{"ok-aaaaaa", "h-ok"}, {"ttl-aaaaaa", "h-ttl"}, {"mine-aaaaaa", "h-mine"},
		{"hid-aaaaaa", "h-hid"}, {"trs-aaaaaa", "h-trs"}, {"rep-aaaaaa", "h-rep"},
		{"big-aaaaaa", "h-big"}, {"own-aaaaaa", "h-own"},
		{"ok-aaaaaa", "wrong"}, {"nope-aaaaaa", "x"},
	}
	res, err := d.ClaimImages(ctx, u.ID, items[:8], 1000, 30*86400)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Claimed) != 3 || res.Claimed[0] != "ok-aaaaaa" || res.Claimed[1] != "ttl-aaaaaa" || res.Claimed[2] != "mine-aaaaaa" {
		t.Fatalf("claimed %v", res.Claimed)
	}
	reasons := map[string]string{}
	for _, s := range res.Skipped {
		reasons[s.ID] = s.Reason
	}
	want := map[string]string{"hid-aaaaaa": "unavailable", "trs-aaaaaa": "unavailable", "rep-aaaaaa": "unavailable", "big-aaaaaa": "quota", "own-aaaaaa": "invalid"}
	for id, r := range want {
		if reasons[id] != r {
			t.Errorf("%s: %q, want %q (all: %v)", id, reasons[id], r, reasons)
		}
	}
	var uid sql.NullInt64
	var tok string
	var exp, at int64
	for id, wantExp := range map[string]int64{"ok-aaaaaa": 0, "ttl-aaaaaa": 0, "mine-aaaaaa": 0} {
		var created int64
		if err := d.QueryRow(`SELECT user_id, owner_token, expires_at, claimed_at, created_at FROM images WHERE id=?`, id).Scan(&uid, &tok, &exp, &at, &created); err != nil {
			t.Fatal(err)
		}
		if id == "mine-aaaaaa" {
			wantExp = created + 3600 // the guest chose this expiry: it stays
		}
		if uid.Int64 != u.ID || tok != "" || exp != wantExp || at == 0 {
			t.Errorf("%s: user %v token %q expires %d claimed_at %d", id, uid, tok, exp, at)
		}
	}
	// Untouched: the unclaimable ones keep their token and have no owner.
	if err := d.QueryRow(`SELECT user_id, owner_token FROM images WHERE id='hid-aaaaaa'`).Scan(&uid, &tok); err != nil || uid.Valid || tok != "h-hid" {
		t.Errorf("hidden image changed: %v %q %v", uid, tok, err)
	}

	// A second claim: the first three are "already", a wrong token or a missing id is "invalid".
	res, err = d.ClaimImages(ctx, u.ID, []ClaimItem{items[0], items[8], items[9]}, 1000, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Claimed) != 0 || len(res.Already) != 2 || len(res.Skipped) != 1 || res.Skipped[0].Reason != "invalid" {
		t.Fatalf("second claim: %+v", res)
	}
	// Someone else cannot claim a claimed image with the old token.
	res, _ = d.ClaimImages(ctx, other.ID, []ClaimItem{{"ok-aaaaaa", "h-ok"}}, 1000, 0)
	if len(res.Claimed) != 0 || len(res.Skipped) != 1 || res.Skipped[0].Reason != "invalid" {
		t.Fatalf("other member: %+v", res)
	}
	// A stolen id with the wrong token claims nothing.
	guestImage(t, d, "str-aaaaaa", "h-str", nil)
	res, _ = d.ClaimImages(ctx, other.ID, []ClaimItem{{"str-aaaaaa", "guess"}, {"str-aaaaaa", ""}}, 1000, 0)
	if len(res.Claimed) != 0 {
		t.Fatalf("claimed with a wrong token: %+v", res)
	}
}

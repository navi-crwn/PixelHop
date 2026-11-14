package main

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/navi-crwn/pixelhop/internal/db"
)

// mergeFixture makes an old account that holds one of everything and a new (HOP ID) account with its own data.
func mergeFixture(t *testing.T) (dir string, old, neu *db.User) {
	t.Helper()
	dir = t.TempDir()
	d, err := db.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	ctx := context.Background()
	old = &db.User{Email: "old@example.com", Name: "Old", QuotaBytes: 500 << 20, PassHash: "hash", Role: "admin"}
	neu = &db.User{Email: "new@example.com", Name: "New", QuotaBytes: 128 << 20, HelinkID: sql.NullString{String: "9", Valid: true}}
	for _, u := range []*db.User{old, neu} {
		if err := d.InsertUser(ctx, u); err != nil {
			t.Fatal(err)
		}
	}
	old.GoogleID = sql.NullString{String: "g-old", Valid: true}
	old.AvatarKey, old.Warning = "avatars/1-abc.jpg", "Please keep it friendly."
	if err := d.UpdateUser(ctx, old); err != nil {
		t.Fatal(err)
	}
	neu.HelinkID = sql.NullString{String: "9", Valid: true}
	d.UpdateUser(ctx, neu)
	if err := d.InsertAlbum(ctx, &db.Album{ID: "alb1", UserID: sql.NullInt64{Int64: old.ID, Valid: true}, Title: "Trip"}); err != nil {
		t.Fatal(err)
	}
	for i, id := range []string{"img-one", "img-two"} {
		im := &db.Image{ID: id, UserID: sql.NullInt64{Int64: old.ID, Valid: true}, Mime: "image/png", Ext: "png", Size: 1 << 20, Backend: "local", KeyFull: id + ".png", KeyThumb: id + "_t.png", SHA256: id}
		if i == 0 {
			im.AlbumID = sql.NullString{String: "alb1", Valid: true}
		}
		if err := d.InsertImage(ctx, im); err != nil {
			t.Fatal(err)
		}
	}
	if err := d.InsertAPIKey(ctx, old.ID, "sharex", "ph_abcdef", "hashed"); err != nil {
		t.Fatal(err)
	}
	if err := d.InsertWebhook(ctx, &db.Webhook{UserID: old.ID, URL: "https://example.com/hook", Secret: "s", Events: "image.uploaded"}); err != nil {
		t.Fatal(err)
	}
	if _, err := d.Exec(`INSERT INTO reminders (user_id,key,dismissed_at) VALUES (?,?,1)`, old.ID, "tips"); err != nil {
		t.Fatal(err)
	}
	if _, err := d.Exec(`UPDATE users SET block_hotlinks=1 WHERE id=?`, old.ID); err != nil {
		t.Fatal(err)
	}
	if err := d.InsertSession(ctx, "old-tok", old.ID, time.Hour, "", ""); err != nil {
		t.Fatal(err)
	}
	return dir, old, neu
}

func runMerge(t *testing.T, dir string, args ...string) string {
	t.Helper()
	t.Setenv("PIXELHOP_DATA", dir)
	r, w, _ := os.Pipe()
	old := os.Stdout
	os.Stdout = w
	err := run(append([]string{"user", "merge"}, args...))
	w.Close()
	os.Stdout = old
	out, _ := io.ReadAll(r)
	if err != nil {
		return "ERROR: " + err.Error()
	}
	return string(out)
}

// --dry-run prints the numbers and changes nothing; the real run moves every kind of data and closes the old account.
func TestUserMergeMovesEverythingAndDryRunChangesNothing(t *testing.T) {
	dir, old, neu := mergeFixture(t)
	dry := runMerge(t, dir, "old@example.com", "new@example.com", "--dry-run")
	t.Log("\n" + dry)
	for _, want := range []string{"Would move", "images 2", "albums 1", "API keys 1", "webhooks 1", "reminder cards 1", "profile photo: moved", "content warning moves: true", "WARNING: 1 API key(s) and 1 webhook(s)", "is an admin", "Nothing was changed"} {
		if !strings.Contains(dry, want) {
			t.Errorf("dry run does not say %q:\n%s", want, dry)
		}
	}
	d, _ := db.Open(dir)
	ctx := context.Background()
	if _, err := d.UserByID(ctx, old.ID); err != nil {
		t.Fatal("the dry run removed the old account")
	}
	if n := d.CountWebhooks(ctx, old.ID); n != 1 {
		t.Fatalf("the dry run moved a webhook: %d", n)
	}
	d.Close()

	out := runMerge(t, dir, "old@example.com", "new@example.com")
	t.Log("\n" + out)
	if !strings.Contains(out, "Moved") || strings.HasPrefix(out, "ERROR") {
		t.Fatalf("merge: %s", out)
	}
	d, _ = db.Open(dir)
	defer d.Close()
	if _, err := d.UserByID(ctx, old.ID); err == nil {
		t.Error("the old account was left open")
	}
	if _, err := d.SessionUser(ctx, "old-tok"); err == nil {
		t.Error("an old session survived")
	}
	if _, err := d.UserByEmail(ctx, "old@example.com"); err == nil {
		t.Error("the old e-mail still signs in")
	}
	if _, err := d.UserByGoogle(ctx, "g-old"); err == nil {
		t.Error("the old Google link survived")
	}
	got, _ := d.UserByID(ctx, neu.ID)
	if used, n := d.UserStorage(ctx, neu.ID); n != 2 || used != 2<<20 {
		t.Errorf("images: %d files, %d bytes", n, used)
	}
	if a, err := d.Album(ctx, "alb1"); err != nil || a.UserID.Int64 != neu.ID {
		t.Errorf("album not moved: %v", err)
	}
	if keys, _ := d.APIKeys(ctx, neu.ID); len(keys) != 1 {
		t.Errorf("API keys: %d", len(keys))
	}
	if hooks, _ := d.Webhooks(ctx, neu.ID); len(hooks) != 1 {
		t.Errorf("webhooks: %d", len(hooks))
	}
	if n := countRows(t, d, `SELECT COUNT(*) FROM reminders WHERE user_id=?`, neu.ID); n != 1 {
		t.Errorf("reminders: %d", n)
	}
	if got.AvatarKey != "avatars/1-abc.jpg" || got.Warning != "Please keep it friendly." {
		t.Errorf("photo/warning: %q %q", got.AvatarKey, got.Warning)
	}
	if got.QuotaBytes != 500<<20 {
		t.Errorf("quota %d, want the larger 500 MB", got.QuotaBytes)
	}
	if got.Role == "admin" {
		t.Error("the old account's admin role moved to the HOP ID account")
	}
	if n := countRows(t, d, `SELECT block_hotlinks FROM users WHERE id=?`, neu.ID); n != 1 {
		t.Error("the embedding block of the old account was lost")
	}
	if !got.HelinkID.Valid || got.HelinkID.String != "9" {
		t.Error("the HOP ID link of the new account changed")
	}
}

func countRows(t *testing.T, d *db.DB, q string, args ...any) (n int) {
	t.Helper()
	d.QueryRowContext(context.Background(), q, args...).Scan(&n)
	return
}

// An old account that has a HOP ID of its own is never merged away.
func TestUserMergeRefusesAnAccountLinkedToAnotherHopID(t *testing.T) {
	dir, old, _ := mergeFixture(t)
	d, _ := db.Open(dir)
	old.HelinkID = sql.NullString{String: "77", Valid: true}
	d.UpdateUser(context.Background(), old)
	d.Close()
	for _, args := range [][]string{{"old@example.com", "new@example.com"}, {"old@example.com", "new@example.com", "--dry-run"}} {
		if out := runMerge(t, dir, args...); !strings.Contains(out, "linked to a HOP ID of its own") {
			t.Errorf("%v: %s", args, out)
		}
	}
	d, _ = db.Open(dir)
	defer d.Close()
	if _, err := d.UserByID(context.Background(), old.ID); err != nil {
		t.Fatal("the account was removed")
	}
}

func fileHash(t *testing.T, dir string) string {
	t.Helper()
	h := sha256.New()
	for _, n := range []string{"pixelhop.db", "pixelhop.db-wal"} {
		b, err := os.ReadFile(filepath.Join(dir, n))
		if err == nil && len(b) > 0 { // an empty -wal file is not data
			h.Write([]byte(n))
			h.Write(b)
		}
	}
	return hex.EncodeToString(h.Sum(nil))
}

// A dry run writes nothing: the database files are byte for byte the same afterwards, and a database that
// has not been migrated to this release is not migrated by it (the run says so and stops).
func TestUserMergeDryRunWritesNothing(t *testing.T) {
	dir, _, _ := mergeFixture(t)
	before := fileHash(t, dir)
	out := runMerge(t, dir, "old@example.com", "new@example.com", "--dry-run")
	if !strings.Contains(out, "Would move") || fileHash(t, dir) != before {
		t.Fatalf("a dry run changed the database or failed: %s", out)
	}

	d, _ := db.Open(dir)
	if _, err := d.Exec(`DELETE FROM schema_version WHERE v=(SELECT MAX(v) FROM schema_version)`); err != nil {
		t.Fatal(err)
	}
	d.Close()
	before = fileHash(t, dir)
	out = runMerge(t, dir, "old@example.com", "new@example.com", "--dry-run")
	if !strings.Contains(out, "has not been updated") || fileHash(t, dir) != before {
		t.Fatalf("a dry run on an old schema must stop without writing: %s", out)
	}
}

// Into an account that has its own photo, a warning and a clashing reminder key: nothing of the new
// account is overwritten; the old photo is reported as kept-by-new.
func TestUserMergeKeepsWhatTheNewAccountHas(t *testing.T) {
	dir, old, neu := mergeFixture(t)
	d, _ := db.Open(dir)
	ctx := context.Background()
	neu.AvatarKey, neu.Warning = "avatars/2-own.jpg", "Own warning."
	d.UpdateUser(ctx, neu)
	d.Exec(`INSERT INTO reminders (user_id,key,dismissed_at) VALUES (?,?,5)`, neu.ID, "tips")
	d.Close()
	out := runMerge(t, dir, "old@example.com", "new@example.com")
	if !strings.Contains(out, "kept: the new account has its own") {
		t.Fatalf("output: %s", out)
	}
	d, _ = db.Open(dir)
	defer d.Close()
	got, _ := d.UserByID(ctx, neu.ID)
	if got.AvatarKey != "avatars/2-own.jpg" || got.Warning != "Own warning." {
		t.Errorf("the new account's own photo or warning was overwritten: %+v", got)
	}
	if n := countRows(t, d, `SELECT COUNT(*) FROM reminders WHERE user_id=?`, neu.ID); n != 1 {
		t.Errorf("reminders after a clash: %d, want 1", n)
	}
	if _, err := d.UserByID(ctx, old.ID); err == nil {
		t.Error("old account left")
	}
}

// MergeUsers itself refuses to merge an account into itself (which would delete it).
func TestMergeUsersRefusesTheSameAccount(t *testing.T) {
	dir, old, _ := mergeFixture(t)
	d, _ := db.Open(dir)
	defer d.Close()
	if _, err := d.MergeUsers(context.Background(), old.ID, old.ID); err == nil {
		t.Fatal("merged an account into itself")
	}
	if n := countRows(t, d, `SELECT COUNT(*) FROM images WHERE user_id=?`, old.ID); n != 2 {
		t.Fatalf("images after the refusal: %d", n)
	}
}

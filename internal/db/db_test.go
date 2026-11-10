package db

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"
)

func TestOpenSetsFilePermissions0600(t *testing.T) {
	dir := t.TempDir()
	d, err := Open(dir)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer d.Close()

	dbPath := filepath.Join(dir, "pixelhop.db")
	info, err := os.Stat(dbPath)
	if err != nil {
		t.Fatalf("Stat(%s): %v", dbPath, err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Fatalf("expected file mode 0600, got %04o", perm)
	}
}

func TestAdminNoteMigrationPreservesLegacyDescriptions(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "pixelhop.db")
	sqlDB, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	for _, stmt := range []string{
		`CREATE TABLE schema_version (v INTEGER NOT NULL)`,
		`INSERT INTO schema_version (v) VALUES (14)`,
		`CREATE TABLE albums (id TEXT PRIMARY KEY, description TEXT NOT NULL DEFAULT '')`,
		`CREATE TABLE sessions (token_hash TEXT PRIMARY KEY, user_id INTEGER NOT NULL, created_at INTEGER NOT NULL, expires_at INTEGER NOT NULL, ip TEXT NOT NULL DEFAULT '', user_agent TEXT NOT NULL DEFAULT '')`,
		`INSERT INTO sessions (token_hash, user_id, created_at, expires_at) VALUES ('old', 1, 1, 4102444800)`,
		`INSERT INTO albums (id, description) VALUES ('legacy-ticket', 'ambiguous legacy text')`,
	} {
		if _, err := sqlDB.Exec(stmt); err != nil {
			sqlDB.Close()
			t.Fatal(err)
		}
	}
	if err := sqlDB.Close(); err != nil {
		t.Fatal(err)
	}
	d, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	var description, note string
	var version int
	if err := d.QueryRow(`SELECT description, admin_note, (SELECT MAX(v) FROM schema_version) FROM albums WHERE id='legacy-ticket'`).Scan(&description, &note, &version); err != nil {
		t.Fatal(err)
	}
	// A session from before verified_at existed counts as never verified (0), so its first page load re-checks it.
	if at, err := d.SessionVerifiedAt(t.Context(), "old"); err != nil || at != 0 {
		t.Fatalf("old session verified_at = %d, %v", at, err)
	}
	// There is no provenance that distinguishes an old ticket note from a public description.
	if description != "ambiguous legacy text" || note != "" || version != len(migrations) {
		t.Fatalf("description=%q admin_note=%q schema=%d; legacy text must remain unchanged", description, note, version)
	}
}

func TestViewsAreCountedInMemoryAndFlushedTogether(t *testing.T) {
	ctx := context.Background()
	d, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	im := &Image{ID: "v1", OwnerToken: "t", Mime: "image/png", Ext: "png", Backend: "local", KeyFull: "a", KeyThumb: "b"}
	if err := d.InsertImage(ctx, im); err != nil {
		t.Fatalf("insert: %v", err)
	}
	for i := 0; i < 5; i++ {
		d.CountView(ctx, "v1", "example.org")
	}
	d.CountView(ctx, "gone", "x.test") // an image that no longer exists matches nothing
	got, _ := d.Image(ctx, "v1")
	if got.Views != 0 {
		t.Fatalf("views are written on flush, not per visit: %d", got.Views)
	}
	d.FlushViews(ctx)
	got, _ = d.Image(ctx, "v1")
	if got.Views != 5 {
		t.Fatalf("views after flush = %d, want 5", got.Views)
	}
	days, hosts := d.ImageStats(ctx, "v1")
	if len(days) != 1 || days[0].Views != 5 || len(hosts) != 1 || hosts[0].Views != 5 {
		t.Fatalf("stats = %+v %+v", days, hosts)
	}
	d.CountView(ctx, "v1", "")
	d.FlushViews(ctx)
	got, _ = d.Image(ctx, "v1")
	if got.Views != 6 {
		t.Fatalf("second flush adds up: %d", got.Views)
	}
}

package db

import (
	"context"
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

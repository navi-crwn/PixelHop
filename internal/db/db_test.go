package db

import (
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

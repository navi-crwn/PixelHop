package main

import (
	"os"
	"path/filepath"
	"testing"
)

// A mistyped command ("servee") must say so and leave the disk alone: no data folder, no database.
func TestUnknownCommandTouchesNothing(t *testing.T) {
	data := filepath.Join(t.TempDir(), "data")
	t.Setenv("PIXELHOP_DATA", data)
	err := run([]string{"servee"})
	if err == nil || err.Error() != `unknown command "servee"` {
		t.Fatalf("run: %v", err)
	}
	if _, statErr := os.Stat(data); !os.IsNotExist(statErr) {
		t.Fatalf("the data folder was created (stat error %v)", statErr)
	}
}

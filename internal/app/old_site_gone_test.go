package app

import (
	"os"
	"strings"
	"testing"
)

// The old seven-language website builder is gone, and nothing points at it (running it would have
// overwritten the published pages with an outdated site).
func TestOldWebsiteBuilderIsGone(t *testing.T) {
	if _, err := os.Stat("../../site"); err == nil {
		t.Error("the site/ folder is back")
	}
	b, err := os.ReadFile("../../docs/assets/js/site.js")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(b), "site/build.py") {
		t.Error("docs/assets/js/site.js still names the old builder")
	}
}

package app

import (
	"os"
	"strings"
	"testing"
)

// The release workflow injects the version without the tag's "v" (3.8.0). The
// other ways to build must do the same, or the same release reports itself as
// "v3.8.0" on the status page, in /hop/health and in the User-Agent.
func TestBuildPathsInjectTheVersionWithoutTheTagPrefix(t *testing.T) {
	for file, want := range map[string]string{
		"../../Makefile":                      "$(patsubst v%,%,$(shell git describe",
		"../../install.sh":                    `VERSION="${VERSION#v}"`,
		"../../update.sh":                     `VERSION="${VERSION#v}"`,
		"../../.github/workflows/release.yml": "VERSION=${{ steps.ver.outputs.version }}",
	} {
		b, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(b), want) {
			t.Errorf("%s does not strip the leading v from the version (want %q)", file, want)
		}
	}
}

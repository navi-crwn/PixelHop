package app

import "testing"

func TestSafeNextRejectsControlCharacters(t *testing.T) {
	for next, want := range map[string]string{
		"/dashboard?x=1": "/dashboard?x=1",
		"/settings#api":  "/settings#api",
		"//evil.test":    "/dashboard",
		"/\\evil.test":   "/dashboard",
		"/\t/evil.test":  "/dashboard", // browsers drop the tab: it would become //evil.test
		"/\n/evil.test":  "/dashboard",
		"/\r/evil.test":  "/dashboard",
		"/a\x00b":        "/dashboard",
		"https://x.test": "/dashboard",
		"":               "/dashboard",
	} {
		if got := safeNext(next); got != want {
			t.Errorf("safeNext(%q) = %q, want %q", next, got, want)
		}
	}
}

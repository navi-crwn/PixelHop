package app

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestUICleanCookie(t *testing.T) {
	for v, want := range map[string]bool{"clean": true, "hop": false, "": false, "Clean": false, "x": false} {
		r := httptest.NewRequest("GET", "/", nil)
		if v != "" {
			r.AddCookie(&http.Cookie{Name: "hop_ui", Value: v})
		}
		if got := uiClean(r); got != want {
			t.Errorf("hop_ui=%q: got %v, want %v", v, got, want)
		}
	}
	if uiClean(httptest.NewRequest("GET", "/", nil)) {
		t.Error("no cookie must mean the default look")
	}
}

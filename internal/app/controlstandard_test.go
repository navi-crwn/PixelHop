package app

import (
	"regexp"
	"strings"
	"testing"

	"github.com/navi-crwn/pixelhop/web"
)

// The control standard (hop-ops docs/15): one radius, tokenised heights, no pill
// inputs or buttons. Chips and badges may stay pills.
func TestControlStandardTokens(t *testing.T) {
	b, err := web.Static.ReadFile("static/css/app.css")
	if err != nil {
		t.Fatal(err)
	}
	css := string(b)
	for _, tok := range []string{"--hop-ctl-radius: 8px", "--hop-ctl-h: 36px", "--hop-ctl-h-sm: 32px", "--hop-ctl-h-lg: 40px", "--hop-field-max:", "--hop-field-max-narrow:", "--hop-search-max:"} {
		if !strings.Contains(css, tok) {
			t.Errorf("app.css lacks token %q", tok)
		}
	}
	rule := regexp.MustCompile(`(?m)^([^{}@/]*)\{([^}]*)\}`)
	ctl := regexp.MustCompile(`input|select|textarea|\.btn\b|\.chip\b|\.lang-chip\b|\.seg button|\.kind-tabs a|\.tool-chips a`)
	pill := regexp.MustCompile(`border-radius:\s*(999\d*px|50%|9999px)`)
	for _, m := range rule.FindAllStringSubmatch(css, -1) {
		if ctl.MatchString(m[1]) && pill.MatchString(m[2]) {
			t.Errorf("pill radius on control selector %q", strings.TrimSpace(m[1]))
		}
	}
}

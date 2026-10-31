package i18n

import (
	"encoding/json"
	"io/fs"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/navi-crwn/pixelhop/web"
)

var (
	placeholderRe = regexp.MustCompile(`\{[a-z_]+\}`)
	tagRe         = regexp.MustCompile(`</?[a-z]+[^>]*>`)
)

func raw(t *testing.T, code string) map[string]string {
	b, err := fs.ReadFile(web.I18n, "i18n/"+code+".json")
	if err != nil {
		t.Fatal(err)
	}
	m := map[string]string{}
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("%s: %v", code, err)
	}
	return m
}

func sorted(re *regexp.Regexp, s string) []string {
	x := re.FindAllString(s, -1)
	slices.Sort(x)
	return x
}

func TestCatalogs(t *testing.T) {
	en := raw(t, Default)
	if len(en) == 0 {
		t.Fatal("empty default catalog")
	}
	for _, l := range Langs {
		if l.Code == Default {
			continue
		}
		c := raw(t, l.Code)
		for k := range c {
			if _, ok := en[k]; !ok {
				t.Errorf("%s: unknown key %q", l.Code, k)
			}
		}
		for k, e := range en {
			v, ok := c[k]
			if !ok {
				t.Errorf("%s: missing %q", l.Code, k)
				continue
			}
			if !slices.Equal(sorted(placeholderRe, e), sorted(placeholderRe, v)) {
				t.Errorf("%s: %q placeholders differ: %q vs %q", l.Code, k, e, v)
			}
			if !slices.Equal(sorted(tagRe, e), sorted(tagRe, v)) {
				t.Errorf("%s: %q markup differs: %q vs %q", l.Code, k, e, v)
			}
			if e != "" && strings.TrimSpace(v) == "" {
				t.Errorf("%s: %q is empty", l.Code, k)
			}
		}
	}
}

func TestT(t *testing.T) {
	if got := T("en", "about.title", "site", "PixelHop"); got != "About PixelHop" {
		t.Fatalf("T = %q", got)
	}
	if got := T("xx", "nav.upload"); got != "Upload" {
		t.Fatalf("unknown language should fall back to English, got %q", got)
	}
	if got := T("en", "no.such.key"); got != "no.such.key" {
		t.Fatalf("missing key = %q", got)
	}
	l := Locale{Get("en")}
	if got := string(l.H("about.title", "site", "<b>x</b>")); !strings.Contains(got, "&lt;b&gt;x&lt;/b&gt;") {
		t.Fatalf("H must escape values: %q", got)
	}
	if (Locale{Get("ar")}).Dir() != "rtl" || (Locale{Get("en")}).Dir() != "ltr" {
		t.Fatal("dir")
	}
}

func TestLangs(t *testing.T) {
	if len(Langs) != 8 {
		t.Fatalf("expected 8 languages, got %d", len(Langs))
	}
	expected := []string{"en", "id", "zh", "es", "ar", "pt", "fr", "ja"}
	for i, code := range expected {
		if Langs[i].Code != code {
			t.Errorf("Langs[%d] = %q, want %q", i, Langs[i].Code, code)
		}
	}
}

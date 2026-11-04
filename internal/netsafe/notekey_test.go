package netsafe

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

const key = "FAKEKEY0notehop0s2audit0000000000000000000"

// testdata/note-link-variants.json is hop-safety's fixture: addresses with a made-up key, and whether a WHATWG URL
// parser (Node's URL, the rules browsers use) makes each one open a NoteHop note. Everything that opens a note must
// be recognised. Everything else must be allowed, except the two spellings in stricter, which are refused on
// purpose: a second k parameter after an empty one still puts the key in the stored address (hop-safety does the
// same), and a host with a joiner character cannot be mapped without UTS 46 tables, so it is refused to be safe
// (hop-safety, which has ext-intl, lets it through).
func TestNoteKeyLinkMatchesBrowserReading(t *testing.T) {
	raw, err := os.ReadFile("testdata/note-link-variants.json")
	if err != nil {
		t.Fatal(err)
	}
	var variants []struct {
		Label     string `json:"label"`
		URL       string `json:"url"`
		OpensNote bool   `json:"opens_note"`
	}
	if err := json.Unmarshal(raw, &variants); err != nil {
		t.Fatal(err)
	}
	if len(variants) < 60 {
		t.Fatalf("fixture has %d variants", len(variants))
	}
	stricter := map[string]bool{"zero-width joiner U+200D": true, "fragment k=&k=KEY (empty then real)": true}
	var extra []string
	for _, v := range variants {
		got := NoteKeyLink(v.URL)
		switch {
		case v.OpensNote && !got:
			t.Errorf("%s: %q opens a note but was not recognised", v.Label, v.URL)
		case !v.OpensNote && got:
			extra = append(extra, v.Label)
		}
	}
	for _, label := range extra {
		if !stricter[label] {
			t.Errorf("%s: an ordinary address was refused", label)
		}
	}
	if len(extra) != len(stricter) {
		t.Errorf("refused on purpose: %v, want %d entries", extra, len(stricter))
	}
}

func TestNoteKeyLinkExtraSpellings(t *testing.T) {
	refused := []string{
		"https://note.hel.ink/x#k=" + key, "https://n.hel.ink/x#k=" + key, "HTTPS://NOTE.HEL.INK./x#k=" + key,
		"https://note.hel.ink/x#a=ADMIN0FAKE00", "https://note.hel.ink/x#new=1&k=" + key, "https://note.hel.ink/x#%6B=" + key,
		"https://note.hel.ink/x#?k=" + key, "https://note.hel.ink/x#k=" + key + "&a=ADMIN0FAKE00",
		"note.hel.ink/x#k=" + key, "note.hel.ink:443/x#k=" + key, "//note.hel.ink/x#k=" + key,
		"https:/note.hel.ink/x#k=" + key, "https:note.hel.ink/x#k=" + key, "https:////note.hel.ink/x#k=" + key,
		"https://https:/note.hel.ink/x#k=" + key, "https://note.hel.ink\\@example.com/x#k=" + key,
		"https://note.hel\t.ink/x#k=" + key, " \x01https://note.hel.ink/x#k=" + key,
		"https://ｎote.hel.ink/x#k=" + key, "https://note。hel。ink/x#k=" + key, "https://no­te.hel.ink/x#k=" + key,
		"https://note%2ehel%2eink/x#k=" + key, "https://%6Eote.hel.ink/x#k=" + key, "https://www.note.hel.ink/x#k=" + key,
		"intent://note.hel.ink/x#Intent;scheme=https;S.browser_fallback_url=https%3A%2F%2Fnote.hel.ink%2Fx%23k%3D" + key + ";end",
		"https://note.hel.ink/x#k=" + key, // plain
	}
	for _, u := range refused {
		if !NoteKeyLink(u) {
			t.Errorf("not recognised: %q", u)
		}
	}
	allowed := []string{
		"", "https://note.hel.ink/x", "https://note.hel.ink/x#k=", "https://note.hel.ink/x?k=" + key, "https://note.hel.ink/x#K=" + key,
		"https://note.hel.ink/x#section", "https://example.com/x#k=" + key, "https://note.hel.ink.example.com/x#k=" + key,
		"https://xnote.hel.ink/x#k=" + key, "javascript:alert(1)//#k=" + key, "mailto:a@note.hel.ink#k=" + key,
		"https://hel.ink/b/maya#k=1", "https://example.com/#a=1", "example.com:8080/x#k=1",
	}
	for _, u := range allowed {
		if NoteKeyLink(u) {
			t.Errorf("wrongly recognised: %q", u)
		}
	}
}

func TestScrubNoteKeys(t *testing.T) {
	in := "see https://note.hel.ink/x#k=" + key + "&a=ADMIN0FAKE00 and https://example.com/page#a=1 and n.hel.ink/y%23k=" + key + " end"
	want := "see https://note.hel.ink/x and https://example.com/page#a=1 and n.hel.ink/y end"
	if got := ScrubNoteKeys(in); got != want {
		t.Fatalf("got  %q\nwant %q", got, want)
	}
	for in, want := range map[string]string{
		"https://note.hel.ink/x#section":                                     "https://note.hel.ink/x#section",
		"https://note.hel.ink/x#new=1&k=" + key:                              "https://note.hel.ink/x#new=1", // the rest of the fragment stays
		"https://note.hel.ink/x#?k=" + key:                                   "https://note.hel.ink/x",
		"plain words, no address":                                            "plain words, no address",
		"https://example.com/?u=https%3A%2F%2Fnote.hel.ink%2Fx%23k%3D" + key: "https://example.com/?u=https%3A%2F%2Fnote.hel.ink%2Fx",
	} {
		got := ScrubNoteKeys(in)
		if got != want {
			t.Errorf("ScrubNoteKeys(%q) = %q, want %q", in, got, want)
		}
		if strings.Contains(got, key) {
			t.Errorf("key kept in %q", got)
		}
	}
}

func TestNoteKeyInText(t *testing.T) {
	for text, want := range map[string]bool{
		"look at https://note.hel.ink/x#k=" + key + " please":                              true,
		"n.hel.ink/y%23k=" + key:                                                           true,
		"nested https://x.test/?u=https%253A%252F%252Fnote.hel.ink%252Fx%2523k%253D" + key: true,
		"https://example.com/#a=1 and https://note.hel.ink/x":                              false,
		"nothing to see": false,
	} {
		if got := NoteKeyInText(text); got != want {
			t.Errorf("NoteKeyInText(%q) = %v, want %v", text, got, want)
		}
	}
}

func TestNoteHostsFromEnv(t *testing.T) {
	t.Setenv("PIXELHOP_NOTE_HOSTS", "notes.example.org, https://n.example.org/")
	if !NoteKeyLink("https://notes.example.org/x#k="+key) || !NoteKeyLink("https://n.example.org/x#k="+key) || NoteKeyLink("https://note.hel.ink/x#k="+key) {
		t.Fatal("PIXELHOP_NOTE_HOSTS not used")
	}
}

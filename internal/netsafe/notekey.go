package netsafe

import (
	"net/url"
	"os"
	"regexp"
	"strings"
	"unicode"
)

// An encrypted NoteHop link keeps the note key in its #fragment (https://note.hel.ink/TOKEN#k=KEY, and &a=ADMIN
// for the owner link). Browsers never send a fragment, which is why NoteHop cannot read the note; an app that
// stores the whole address, shows it or forwards it holds the key.
//
// This is the Go counterpart of hop-safety's EncryptedNoteLink (PHP) and follows its rules: the address is read
// the way a browser reads it, a NoteHop host with a k or a parameter that has a value in the fragment is a key
// link, and the fixture in testdata/ is the one hop-safety checks against a WHATWG URL parser. One difference:
// the PHP side maps hosts with ext-intl (UTS 46); here the mapping is stdlib only (full-width letters, ideographic
// full stops, soft hyphens and similar), and a host that still has non-ASCII characters after that is treated as
// "might be a NoteHop host" and refused when the fragment carries a secret, never guessed to be harmless.
//
// Hosts come from PIXELHOP_NOTE_HOSTS (comma separated), default note.hel.ink and n.hel.ink.

var defaultNoteHosts = []string{"note.hel.ink", "n.hel.ink"}

var (
	secretParams   = map[string]bool{"k": true, "a": true}
	hasScheme      = regexp.MustCompile(`(?i)^[a-z][a-z0-9+.\-]*:`)
	hostAndPort    = regexp.MustCompile(`^[^/:]+:\d+(/|$)`)
	schemeSlashes  = regexp.MustCompile(`(?i)^https?:[/\\]*[\x00-\x20]*`)
	httpAuthority  = regexp.MustCompile(`(?i)^https?:[/\\]*([^/\\?#]*)`)
	portSuffix     = regexp.MustCompile(`:\d*$`)
	intentFallback = regexp.MustCompile(`(?i)^intent://[^#]*#Intent;(?:[^;]*;)*?S\.browser_fallback_url=([^;]*)`)
	addressStart   = regexp.MustCompile(`(?i)^(?:https?:)?//[^/]`)
	textToken      = regexp.MustCompile(`[^\s"'<>()\[\]{}]+`)
	escapedHash    = regexp.MustCompile(`(?i)%23`)
)

// NoteHosts is the lower-case list of NoteHop hosts.
func NoteHosts() []string {
	raw := defaultNoteHosts
	if v := strings.TrimSpace(os.Getenv("PIXELHOP_NOTE_HOSTS")); v != "" {
		raw = strings.Split(v, ",")
	}
	var out []string
	for _, h := range raw {
		h = strings.TrimSpace(h)
		if i := strings.Index(h, "://"); i >= 0 {
			h = h[i+3:]
		}
		if i := strings.IndexAny(h, "/?#"); i >= 0 {
			h = h[:i]
		}
		if host, ok := normalizeHost(portSuffix.ReplaceAllString(h, "")); ok && host != "" {
			out = append(out, host)
		}
	}
	return out
}

// normalizeHost reads a host the way a browser does: percent escapes decoded, full-width ASCII mapped down,
// ideographic full stops turned into dots, characters UTS 46 ignores dropped, lower case, no trailing dot, no
// leading www. ok is false when non-ASCII characters remain that this stdlib-only mapping cannot read.
func normalizeHost(h string) (host string, ok bool) {
	h = percentDecode(strings.Trim(h, "[]"))
	h = strings.Map(func(r rune) rune {
		switch {
		case r >= 0xFF01 && r <= 0xFF5E:
			return r - 0xFEE0
		case r == 0x3002 || r == 0xFF0E || r == 0xFF61:
			return '.'
		case r == 0x00AD || r == 0x200B || r == 0x2060 || r == 0xFEFF:
			return -1
		case unicode.IsControl(r):
			return -1
		}
		return r
	}, h)
	for _, r := range h {
		if r > unicode.MaxASCII {
			return h, false
		}
	}
	h = strings.TrimRight(strings.ToLower(h), ".")
	return strings.TrimPrefix(h, "www."), true
}

// percentDecode is rawurldecode: %XX becomes a byte, anything else (including a bad escape) stays.
func percentDecode(s string) string {
	if !strings.Contains(s, "%") {
		return s
	}
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == '%' && i+2 < len(s) && isHex(s[i+1]) && isHex(s[i+2]) {
			b.WriteByte(unhex(s[i+1])<<4 | unhex(s[i+2]))
			i += 2
			continue
		}
		b.WriteByte(s[i])
	}
	return b.String()
}

func isHex(c byte) bool {
	return c >= '0' && c <= '9' || c >= 'a' && c <= 'f' || c >= 'A' && c <= 'F'
}

func unhex(c byte) byte {
	switch {
	case c >= 'a':
		return c - 'a' + 10
	case c >= 'A':
		return c - 'A' + 10
	}
	return c - '0'
}

// formDecode is urldecode: like percentDecode, and "+" is a space.
func formDecode(s string) string { return percentDecode(strings.ReplaceAll(s, "+", " ")) }

// dropLineBreaks removes what a browser ignores anywhere in an address.
func dropLineBreaks(s string) string {
	return strings.NewReplacer("\t", "", "\r", "", "\n", "").Replace(s)
}

// browserClean drops tabs and line breaks anywhere and control characters and spaces at both ends.
func browserClean(s string) string {
	return strings.TrimFunc(dropLineBreaks(s), func(r rune) bool { return r <= 0x20 })
}

// fragmentParts splits a fragment the way note.js reads it (new URLSearchParams(location.hash.slice(1))): tabs and
// line breaks dropped, one leading "?" dropped ("#?k=..." is the k parameter), pairs split on &.
func fragmentParts(fragment string) (pairs []string, mark string) {
	fragment = dropLineBreaks(strings.TrimPrefix(fragment, "#"))
	if strings.HasPrefix(fragment, "?") {
		fragment, mark = fragment[1:], "?"
	}
	return strings.Split(fragment, "&"), mark
}

func pairName(pair string) string {
	name, _, _ := strings.Cut(pair, "=")
	return formDecode(name)
}

// fragmentHasSecret reports whether the fragment has a k or a parameter with a value.
func fragmentHasSecret(fragment string) bool {
	pairs, _ := fragmentParts(fragment)
	for _, pair := range pairs {
		name, value, _ := strings.Cut(pair, "=")
		if secretParams[formDecode(name)] && formDecode(value) != "" {
			return true
		}
	}
	return false
}

// NoteKeyLink reports whether raw is the address of an encrypted NoteHop note: a NoteHop host and a fragment with
// a k or a parameter that has a value. It accepts an address without a scheme, the way people paste them, and the
// fallback address of an Android intent.
func NoteKeyLink(raw string) bool {
	clean := browserClean(raw)
	if m := intentFallback.FindStringSubmatch(clean); m != nil {
		return NoteKeyLink(percentDecode(m[1]))
	}
	if clean == "" || !strings.Contains(clean, "#") {
		return false
	}
	if !hasScheme.MatchString(clean) || hostAndPort.MatchString(clean) {
		clean = "https://" + strings.TrimLeft(clean, "/\\")
	}
	// "https://https:/note.hel.ink/..." (a form that puts https:// in front of an address that has one): the key is
	// stored all the same, so read through the repeated scheme.
	for {
		lead := schemeSlashes.FindString(clean)
		rest := strings.ToLower(clean[len(lead):])
		if lead == "" || !(strings.HasPrefix(rest, "http:") || strings.HasPrefix(rest, "https:")) {
			break
		}
		clean = clean[len(lead):]
	}
	m := httpAuthority.FindStringSubmatch(clean)
	if m == nil {
		return false
	}
	authority := m[1]
	if i := strings.LastIndex(authority, "@"); i >= 0 {
		authority = authority[i+1:]
	}
	host, readable := normalizeHost(portSuffix.ReplaceAllString(authority, ""))
	if !fragmentHasSecret(clean[strings.Index(clean, "#")+1:]) {
		return false
	}
	if !readable {
		return true // might be a NoteHop host written in a way this code cannot read; refuse
	}
	for _, h := range NoteHosts() {
		if host == h {
			return true
		}
	}
	return false
}

// NoteKeyInText reports whether free text (a report note) contains such an address, with or without a scheme.
// Only text with a "#" or "%23" in it is looked at; up to four rounds of percent-decoding catch an address inside
// another one's query.
func NoteKeyInText(text string) bool {
	for round := 0; round < 4; round++ {
		if strings.Contains(text, "#") || escapedHash.MatchString(text) {
			for _, address := range addressesIn(text) {
				if NoteKeyLink(address) {
					return true
				}
			}
		}
		decoded := percentDecode(text)
		if decoded == text {
			break
		}
		text = decoded
	}
	return false
}

// addressesIn returns every run without whitespace or quotes that has a # in it, plus the addresses that start
// inside such a run (intent://...;S.browser_fallback_url=https://note...#k=...).
func addressesIn(text string) []string {
	var found []string
	for _, token := range textToken.FindAllString(escapedHash.ReplaceAllString(text, "#"), -1) {
		if !strings.Contains(token, "#") {
			continue
		}
		found = append(found, token)
		for at := 1; at < len(token); at++ {
			if addressStart.MatchString(token[at:]) {
				found = append(found, token[at:])
			}
		}
	}
	return found
}

// withoutSecrets takes the secret parameters (k, a) out of an address's fragment; any other part of the fragment
// stays ("#new=1&k=KEY" becomes "#new=1", "#k=KEY" becomes nothing). A leading "?" in the fragment is kept. The
// fallback address of an Android intent is cleaned the same way.
func withoutSecrets(raw string) string {
	if loc := intentFallback.FindStringSubmatchIndex(raw); loc != nil {
		inner := raw[loc[2]:loc[3]]
		return raw[:loc[2]] + strings.ReplaceAll(url.QueryEscape(withoutSecrets(percentDecode(inner))), "+", "%20") + raw[loc[3]:]
	}
	at := strings.Index(raw, "#")
	if at < 0 {
		return raw
	}
	pairs, mark := fragmentParts(raw[at+1:])
	var kept []string
	for _, pair := range pairs {
		if pair != "" && !secretParams[pairName(pair)] {
			kept = append(kept, pair)
		}
	}
	if len(kept) == 0 {
		return raw[:at]
	}
	return raw[:at] + "#" + mark + strings.Join(kept, "&")
}

// ScrubNoteKeys takes the secret parameters out of every NoteHop address in text (a report note). Only an address
// on a NoteHop host is touched, so "https://example.com/#a=1" stays as it was, and so does every other part of the
// text. An address hidden inside another one (percent-encoded) loses its whole fragment.
func ScrubNoteKeys(text string) string {
	return textToken.ReplaceAllStringFunc(text, func(token string) string {
		if !strings.Contains(token, "#") && !escapedHash.MatchString(token) {
			return token
		}
		plain := token
		if loc := escapedHash.FindStringIndex(token); loc != nil {
			plain = token[:loc[0]] + "#" + token[loc[1]:] // the first %23 only
		}
		if NoteKeyLink(plain) {
			return withoutSecrets(plain)
		}
		if NoteKeyInText(token) {
			cut := len(token)
			if i := strings.Index(token, "#"); i >= 0 {
				cut = i
			}
			if loc := escapedHash.FindStringIndex(token); loc != nil && loc[0] < cut {
				cut = loc[0]
			}
			return token[:cut]
		}
		return token
	})
}

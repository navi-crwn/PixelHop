package app

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
	"time"
)

var (
	canonRe = regexp.MustCompile(`<link rel="canonical" href="([^"]*)">`)
	ogURLRe = regexp.MustCompile(`<meta property="og:url" content="([^"]*)">`)
	ogImgRe = regexp.MustCompile(`<meta property="og:image" content="([^"]*)">`)
	altRe   = regexp.MustCompile(`<link rel="alternate" hreflang="([^"]*)" href="([^"]*)">`)
	ldRe    = regexp.MustCompile(`(?s)<script type="application/ld\+json">(.*?)</script>`)
)

func first(re *regexp.Regexp, s string) string {
	if m := re.FindStringSubmatch(s); m != nil {
		return strings.ReplaceAll(m[1], "&amp;", "&")
	}
	return ""
}

func indexable(rec interface {
	Header() http.Header
}, body string) bool {
	return rec.Header().Get("X-Robots-Tag") == "" && !strings.Contains(body, `<meta name="robots" content="noindex">`)
}

// Every fixed public page names its absolute canonical address on the app host, without the query it was
// asked with (a valid ?lang= excepted: that is the page in that language, as on the hub), lists its nine
// languages and x-default, and carries og:url, an absolute og:image and twitter:card. On the link host the
// same paths answer 301 to the app host, so they are never served there.
func TestSEOFixedPages(t *testing.T) {
	_, h := linkApp(t)
	for _, p := range sitePages {
		rec := hostGet(h, "pixel.example", p+"?utm_source=x&page=2")
		body := rec.Body.String()
		if rec.Code != 200 || !indexable(rec, body) {
			t.Errorf("%s: %d, indexable %v", p, rec.Code, indexable(rec, body))
			continue
		}
		want := "https://pixel.example" + p
		if c := first(canonRe, body); c != want {
			t.Errorf("%s: canonical %q, want %q", p, c, want)
		}
		if u := first(ogURLRe, body); u != want {
			t.Errorf("%s: og:url %q", p, u)
		}
		if img := first(ogImgRe, body); !strings.HasPrefix(img, "https://") {
			t.Errorf("%s: og:image %q is not absolute", p, img)
		}
		if !strings.Contains(body, `<meta name="twitter:card"`) || !strings.Contains(body, `<meta property="og:type" content="website">`) {
			t.Errorf("%s: no twitter:card or og:type", p)
		}
		alts := altRe.FindAllStringSubmatch(body, -1)
		codes := []string{"en", "zh-CN", "de", "id", "ja", "pt-BR", "es", "fr", "ru"}
		if len(alts) != 10 || alts[9][1] != "x-default" || alts[9][2] != want {
			t.Errorf("%s: alternates %v", p, alts)
		} else {
			for i, c := range codes {
				if alts[i][1] != c || strings.ReplaceAll(alts[i][2], "&amp;", "&") != want+"?lang="+c {
					t.Errorf("%s: alternate %d = %v", p, i, alts[i])
				}
			}
		}
		// ?lang= is the page's address in that language (family rule): kept, everything else dropped;
		// a language that is not one of the nine gives the bare address; a hop_lang cookie changes nothing.
		for q, wantC := range map[string]string{
			"?lang=de": want + "?lang=de", "?lang=de&utm_source=x": want + "?lang=de", "?utm_source=x&lang=pt": want + "?lang=pt-BR",
			"?lang=en": want + "?lang=en", "?lang=xx": want, "?lang=ar": want,
		} {
			body := hostGet(h, "pixel.example", p+q, &http.Cookie{Name: "hop_lang", Value: "ja"}).Body.String()
			if c := first(canonRe, body); c != wantC {
				t.Errorf("%s%s: canonical %q, want %q", p, q, c, wantC)
			}
			if u := first(ogURLRe, body); u != wantC {
				t.Errorf("%s%s: og:url %q", p, q, u)
			}
		}
		// Asked through another host name of the same server: still the configured base URL.
		if c := first(canonRe, hostGet(h, "other.example", p).Body.String()); c != want {
			t.Errorf("%s on another host name: canonical %q", p, c)
		}
		// Through the link host: a permanent redirect to the app host, never a second copy.
		if rec := hostGet(h, "p.example", p); rec.Code != http.StatusMovedPermanently || rec.Header().Get("Location") != want {
			t.Errorf("link host %s: %d -> %q", p, rec.Code, rec.Header().Get("Location"))
		}
	}
}

// The home page and the studio pages describe PixelHop as a schema.org WebApplication published by the
// same Organization as the hub's (one @id for the whole family).
func TestSEOStructuredData(t *testing.T) {
	_, h := linkApp(t)
	for _, p := range []string{"/", "/tools", "/tools/batch"} {
		body := hostGet(h, "pixel.example", p).Body.String()
		raw := first(ldRe, body)
		var v map[string]any
		if err := json.Unmarshal([]byte(raw), &v); err != nil {
			t.Fatalf("%s: JSON-LD %q: %v", p, raw, err)
		}
		pub, _ := v["publisher"].(map[string]any)
		if v["@type"] != "WebApplication" || v["url"] != "https://pixel.example"+p || pub["@id"] != "https://hel.ink/#organization" {
			t.Errorf("%s: %v", p, v)
		}
	}
	if strings.Contains(hostGet(h, "pixel.example", "/about").Body.String(), "application/ld+json") {
		t.Error("only the home and studio pages carry the WebApplication block")
	}
}

// Sign-in, account, dashboard and API addresses ask to stay out of any index (header and tag) and are not
// in the sitemap; the sitemap lists every fixed public page.
func TestSEOPrivatePagesAndSitemap(t *testing.T) {
	a, h := linkApp(t)
	u, _ := member(t, a, "seo@example.com")
	tok := randomToken(32)
	a.db.InsertSession(context.Background(), hashToken(tok), u.ID, time.Hour, "", "")
	s := &http.Cookie{Name: sessionCookie, Value: tok}
	for _, p := range []string{"/login", "/register", "/dashboard", "/dashboard/albums", "/dashboard/trash", "/settings"} {
		var rec *httptest.ResponseRecorder
		if p == "/login" || p == "/register" {
			rec = hostGet(h, "pixel.example", p) // signed out: a member is sent on
		} else {
			rec = hostGet(h, "pixel.example", p, s)
		}
		body := rec.Body.String()
		if rec.Code != 200 || indexable(rec, body) || rec.Header().Get("X-Robots-Tag") != "noindex" || first(canonRe, body) != "" || altRe.MatchString(body) {
			t.Errorf("%s: %d, X-Robots-Tag %q, canonical %q", p, rec.Code, rec.Header().Get("X-Robots-Tag"), first(canonRe, body))
		}
	}
	for _, p := range []string{"/api/v1/stats", "/oembed?url=x", "/auth/helink"} {
		if got := hostGet(h, "pixel.example", p).Header().Get("X-Robots-Tag"); got != "noindex" {
			t.Errorf("%s: X-Robots-Tag %q", p, got)
		}
	}
	sm := hostGet(h, "pixel.example", "/sitemap.xml").Body.String()
	if !strings.Contains(sm, `xmlns:xhtml="http://www.w3.org/1999/xhtml"`) {
		t.Error("sitemap has no xhtml namespace")
	}
	for _, p := range sitePages {
		loc := "https://pixel.example" + p
		if strings.Count(sm, "<loc>"+loc+"</loc>") != 1 {
			t.Errorf("sitemap lists %s not exactly once", p)
		}
		for _, c := range []string{"en", "zh-CN", "de", "id", "ja", "pt-BR", "es", "fr", "ru"} {
			if !strings.Contains(sm, `<xhtml:link rel="alternate" hreflang="`+c+`" href="`+loc+`?lang=`+c+`"></xhtml:link>`) {
				t.Errorf("sitemap %s lacks the %s alternate", p, c)
			}
		}
		if !strings.Contains(sm, `<xhtml:link rel="alternate" hreflang="x-default" href="`+loc+`"></xhtml:link>`) {
			t.Errorf("sitemap %s lacks x-default", p)
		}
	}
	for _, p := range []string{"/login", "/register", "/dashboard", "/settings", "/api"} {
		if strings.Contains(sm, "https://pixel.example"+p+"<") || strings.Contains(sm, "https://pixel.example"+p+"/") {
			t.Errorf("sitemap lists %s", p)
		}
	}
	if robots := hostGet(h, "pixel.example", "/robots.txt").Body.String(); !strings.Contains(robots, "Sitemap: https://pixel.example/sitemap.xml") {
		t.Errorf("robots.txt: %s", robots)
	}
}

// User content: a public image page, its album and its owner's profile stay indexable as before, with their
// canonical where crawlers find them (images and albums on the link host, profiles on the app host). A
// private, password-protected, expired, trashed or content-warning file is never indexable: its page answers
// noindex or not at all, and its file carries X-Robots-Tag: noindex or is refused.
func TestSEOUserContent(t *testing.T) {
	a, h := linkApp(t)
	ctx := context.Background()
	u, key := member(t, a, "owner@example.com")
	u.Username.String, u.Username.Valid, u.ShowOwner = "tester", true, true
	a.db.UpdateUser(ctx, u)
	ids := upload(t, h, key, 2) // two files: one album
	pub := upload(t, h, key, 1)[0]
	call(t, h, "PATCH", "/api/v1/images/"+pub, key, map[string]any{"listed": true})
	var albumID string
	a.db.QueryRow(`SELECT album_id FROM images WHERE id=?`, ids[0]).Scan(&albumID)

	// Public: indexable, canonical on the link host; the app host sends guests there.
	rec := hostGet(h, "p.example", "/"+pub)
	if body := rec.Body.String(); rec.Code != 200 || !indexable(rec, body) || first(canonRe, body) != "https://p.example/"+pub {
		t.Fatalf("public image page: %d, canonical %q", rec.Code, first(canonRe, body))
	}
	if rec := hostGet(h, "pixel.example", "/"+pub); rec.Code != http.StatusFound || rec.Header().Get("Location") != "https://p.example/"+pub {
		t.Errorf("app host image page for a guest: %d %q", rec.Code, rec.Header().Get("Location"))
	}
	if rec := hostGet(h, "p.example", "/i/"+pub+".png"); rec.Code != 200 || rec.Header().Get("X-Robots-Tag") != "" {
		t.Errorf("public file: %d %q", rec.Code, rec.Header().Get("X-Robots-Tag"))
	}
	rec = hostGet(h, "p.example", "/a/"+albumID+"?lang=de")
	if body := rec.Body.String(); rec.Code != 200 || !indexable(rec, body) || first(canonRe, body) != "https://p.example/a/"+albumID || altRe.MatchString(body) {
		t.Errorf("album: %d, canonical %q", rec.Code, first(canonRe, body))
	}
	if altRe.MatchString(hostGet(h, "p.example", "/"+pub+"?lang=de").Body.String()) {
		t.Error("an image page has language alternates: its text is the owner's")
	}
	rec = hostGet(h, "pixel.example", "/u/tester?ref=x&lang=de")
	if body := rec.Body.String(); rec.Code != 200 || !indexable(rec, body) || first(canonRe, body) != "https://pixel.example/u/tester" || altRe.MatchString(body) || !strings.HasPrefix(first(ogImgRe, body), "https://") {
		t.Errorf("profile: %d, canonical %q", rec.Code, first(canonRe, body))
	}
	if rec := hostGet(h, "p.example", "/u/tester"); rec.Code != http.StatusMovedPermanently {
		t.Errorf("profile on the link host: %d", rec.Code)
	}
	sm := hostGet(h, "pixel.example", "/sitemap.xml").Body.String()
	if !strings.Contains(sm, "<loc>https://p.example/"+pub+"</loc>") || !strings.Contains(sm, "<loc>https://pixel.example/u/tester</loc>") {
		t.Error("the sitemap lost the listed image or the profile")
	}

	// Each kind of closed file, alone.
	closed := func(name, set string) {
		t.Helper()
		id := upload(t, h, key, 1)[0]
		call(t, h, "PATCH", "/api/v1/images/"+id, key, map[string]any{"listed": true})
		if _, err := a.db.Exec(`UPDATE images SET `+set+` WHERE id=?`, id); err != nil {
			t.Fatal(err)
		}
		for _, host := range []string{"p.example", "pixel.example"} {
			rec := hostGet(h, host, "/"+id)
			if rec.Code == 200 && indexable(rec, rec.Body.String()) {
				t.Errorf("%s: page on %s is indexable", name, host)
			}
			for _, f := range []string{"/i/" + id + ".png", "/t/" + id + ".png", "/i/" + id + ".jpg?w=20"} {
				if rec := hostGet(h, host, f); rec.Code == 200 && rec.Header().Get("X-Robots-Tag") != "noindex" {
					t.Errorf("%s: %s%s answers 200 without noindex", name, host, f)
				}
			}
		}
		if strings.Contains(hostGet(h, "pixel.example", "/sitemap.xml").Body.String(), id) {
			t.Errorf("%s: in the sitemap", name)
		}
	}
	closed("private", "private=1")
	closed("password", "pass_hash='x'")
	closed("expired", "expires_at=1")
	closed("trashed", "hidden=1, hidden_reason='trash', deleted_at=1")
	closed("taken down", "hidden=1, hidden_reason='rules'")
	closed("adult", "content_warning='adult'")
	closed("sensitive", "content_warning='sensitive'")

	// An album with a content warning on one of its images is not indexable either.
	a.db.Exec(`UPDATE images SET content_warning='sensitive' WHERE id=?`, ids[1])
	rec = hostGet(h, "p.example", "/a/"+albumID)
	if rec.Code == 200 && indexable(rec, rec.Body.String()) {
		t.Error("album with a content warning is indexable")
	}
}

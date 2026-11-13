package app

import (
	"encoding/json"
	"html/template"
	"net/http"
	"strings"

	"github.com/navi-crwn/pixelhop/internal/i18n"
)

// What search engines read from the pages. Every indexable page names one canonical address on the app
// host (PIXELHOP_BASE_URL), whatever host the request came in on and whatever query it carried; only a
// valid ?lang= stays, because that is the address of the page in that language (the hub does the same).
// App pages are never served on the link host (links.go answers 301 to the app host). Shared image and
// album pages keep the address their handler gives them, on the link host (PIXELHOP_LINK_URL): the app
// host sends guests there (toLinkHost), so it is where crawlers find them. The fixed public pages also
// name themselves in every language. Account pages, one-time links and the API stay out of any index.

// sitePages are the fixed public pages, in sitemap order.
var sitePages = []string{"/", "/tools", "/tools/batch", "/features", "/faq", "/help", "/docs",
	"/about", "/contact", "/legal", "/changelog", "/status"}

var isSitePage = func() map[string]bool {
	m := make(map[string]bool, len(sitePages))
	for _, p := range sitePages {
		m[p] = true
	}
	return m
}()

// privatePages are the templates meant for one person: sign-in, an account page, a delete link, an upload
// result, a password prompt. None of them belongs in a search index.
var privatePages = map[string]bool{"login": true, "register": true, "dashboard": true, "settings": true,
	"albums": true, "album_edit": true, "trash": true, "delete": true, "deleted": true, "uploaded": true, "locked": true}

// The publisher of the HOP family, named in the structured data with the same @id as the hub's
// Organization (HEL.ink app/Support/Seo.php), so search engines see one publisher for all apps.
const publisherName, publisherURL = "HEL.ink", "https://hel.ink/"

// altLink is one language version of a page, for <link rel="alternate" hreflang>.
type altLink struct{ Lang, URL string }

// seo fills in what a page tells a search engine: its canonical address and, on the fixed public pages,
// the same page in the other languages and a schema.org block. A noindex page gets none of it.
func (a *App) seo(r *http.Request, name string, p *page) {
	if p.NoIndex || p.Canon != "" {
		return
	}
	if p.OGImage == "" { // every indexable page shows a picture when shared: the site's own
		p.OGImage = a.cfg.BaseURL + "/static/img/social.jpg"
	}
	plain := a.cfg.BaseURL + p.Path
	p.Canon = plain
	if !isSitePage[p.Path] {
		// User pages (image, album, profile) keep the bare address: their text is the owner's, not
		// translated, so the ?lang= versions are not pages of their own (no hreflang either).
		return
	}
	// The family rule (KONTRAK-SEO-BAHASA.md): ?lang=<code> always renders that language, whatever the
	// cookie says (requestLang, app.go), so it is the page's address in that language. The canonical keeps
	// it when the request named one of the nine codes; nine alternates plus x-default on the bare address.
	p.Alternates = alternates(plain)
	if code, ok := i18n.Resolve(r.URL.Query().Get("lang")); ok {
		p.Canon = langURL(plain, code)
	}
	if name == "home" || name == "tools" || name == "batch" {
		p.JSONLD = a.appSchema(name, p)
	}
}

// alternates are the nine language addresses of a fixed public page, then x-default (the bare address).
func alternates(plain string) []altLink {
	out := make([]altLink, 0, len(i18n.Langs)+1)
	for _, l := range i18n.Langs {
		out = append(out, altLink{l.Code, langURL(plain, l.Code)})
	}
	return append(out, altLink{"x-default", plain})
}

func langURL(plain, code string) string { return plain + "?lang=" + code }

// studioTools are the catalog keys of the studio's tools, which share one address (/tools#name).
var studioTools = []string{"home.tool_compress", "home.tool_resize", "home.tool_crop", "home.tool_convert",
	"home.tool_redact", "home.tool_palette", "home.tool_ocr", "home.tool_rembg"}

// appSchema describes the app (home page) or one of its tool pages as a schema.org WebApplication. It is
// a data block, not a script: json.Marshal escapes <, > and &, and the page's CSP does not apply to it.
func (a *App) appSchema(name string, p *page) template.JS {
	app := map[string]any{
		"@context":            "https://schema.org",
		"@type":               "WebApplication",
		"name":                p.Site,
		"url":                 p.Canon,
		"description":         p.Desc,
		"inLanguage":          p.L.Code,
		"applicationCategory": "MultimediaApplication",
		"operatingSystem":     "Any",
		"image":               a.cfg.BaseURL + "/static/img/social.jpg",
		"offers":              map[string]any{"@type": "Offer", "price": "0", "priceCurrency": "USD"},
		"publisher":           map[string]any{"@type": "Organization", "@id": publisherURL + "#organization", "name": publisherName, "url": publisherURL},
	}
	if name != "home" {
		app["name"] = p.Site + " " + p.Title
		app["isPartOf"] = map[string]any{"@type": "WebApplication", "name": p.Site, "url": a.cfg.BaseURL + "/"}
	}
	if name == "tools" {
		list := make([]string, 0, len(studioTools))
		for _, k := range studioTools {
			list = append(list, p.L.T(k))
		}
		app["featureList"] = list
	}
	b, err := json.Marshal(app)
	if err != nil {
		return ""
	}
	return template.JS(b)
}

// noIndexPath reports whether an address never belongs in a search index, whatever it answers: the API
// and signed calls, sign-in round trips, account pages, delete links and upload results.
func noIndexPath(path string) bool {
	for _, pre := range []string{"/api", "/hop", "/auth", "/admin", "/dashboard", "/settings", "/delete",
		"/uploaded", "/result", "/oembed", "/metrics"} {
		if path == pre || strings.HasPrefix(path, pre+"/") {
			return true
		}
	}
	return strings.HasPrefix(path, "/a/") && strings.HasSuffix(path, "/edit")
}

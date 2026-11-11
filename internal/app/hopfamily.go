package app

import (
	"encoding/json"
	"html/template"
	"net/url"
	"strings"
	"sync"

	"github.com/navi-crwn/pixelhop/web"
)

// hopAsset is one file in the HOP asset manifest, with its SRI hash.
type hopAsset struct {
	URL       string `json:"url"`
	Integrity string `json:"integrity"`
}

// hopApp is one app of the HOP family as the manifest lists it.
type hopApp struct {
	Slug    string              `json:"slug"`
	Name    string              `json:"name"`
	Tagline string              `json:"tagline"`
	URL     string              `json:"url"`
	Public  bool                `json:"public"`
	Icons   map[string]hopAsset `json:"icons"`
}

type hopManifest struct {
	Version string              `json:"version"`
	Assets  map[string]hopAsset `json:"assets"`
	Apps    []hopApp            `json:"apps"`
}

var hopManifestOnce = sync.OnceValue(func() hopManifest {
	var m hopManifest
	_ = json.Unmarshal(web.HopAssets, &m) // a broken copy only drops the fonts and the family list
	return m
})

// hopFonts is the family stylesheet (Inter, Instrument Serif, JetBrains Mono) on hel.ink.
func hopFonts() *hopAsset {
	if a, ok := hopManifestOnce().Assets["fonts_css"]; ok && a.URL != "" {
		return &a
	}
	return nil
}

// hopUIAssets are the family's shared dialogs and reminder cards: hop-ui.js
// and its stylesheet, hop-assets v5 on hel.ink.
type hopUIAssets struct{ CSS, JS hopAsset }

// hopUI is nil until the bundled manifest lists both files with the script's
// SRI hash; the pages then keep the browser's own dialogs (app.js falls back).
func hopUI() *hopUIAssets {
	return hopUIFrom(hopManifestOnce().Assets)
}

// hopUIFrom is hopUI over a given asset map, so the absence of either file can be tested.
func hopUIFrom(assets map[string]hopAsset) *hopUIAssets {
	css, js := assets["ui_css"], assets["ui_js"]
	if css.URL == "" || js.URL == "" || js.Integrity == "" {
		return nil
	}
	return &hopUIAssets{CSS: css, JS: js}
}

// hopFamily lists the other public HOP apps, in family order.
func hopFamily(current string) []hopApp {
	var out []hopApp
	for _, a := range hopManifestOnce().Apps {
		if a.Public && a.Slug != current {
			out = append(out, a)
		}
	}
	return out
}

// hubMark is the HOP ID mark from the family manifest (the "Part of Hop Easy Link" row).
func hubMark() string {
	for _, a := range hopManifestOnce().Apps {
		if a.Slug == "hub" {
			return a.Icons["mark"].URL
		}
	}
	return ""
}

// partOf fills the name into the translated "Part of {name}" sentence; the name
// stays "Hop Easy Link" in every language.
func partOf(s string) template.HTML {
	return template.HTML(strings.Replace(template.HTMLEscapeString(s), "{name}", "<b>Hop Easy Link</b>", 1))
}

// hostOf is the host of an app address, shown under its name in the family section.
func hostOf(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return raw
	}
	return u.Host
}

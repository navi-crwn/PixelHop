package app

import (
	"encoding/json"
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
	Assets map[string]hopAsset `json:"assets"`
	Apps   []hopApp            `json:"apps"`
}

var hopManifestOnce = sync.OnceValue(func() hopManifest {
	var m hopManifest
	_ = json.Unmarshal(web.HopAssets, &m) // a broken copy only drops the fonts and the family list
	return m
})

// hopFonts is the family stylesheet (Syne, Plus Jakarta Sans, JetBrains Mono) on hel.ink.
func hopFonts() *hopAsset {
	if a, ok := hopManifestOnce().Assets["fonts_css"]; ok && a.URL != "" {
		return &a
	}
	return nil
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

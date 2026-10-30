// Package web holds the HTML templates and static files, compiled into the
// binary so a PixelHop install is a single file.
package web

import "embed"

//go:embed templates
var Templates embed.FS

//go:embed static
var Static embed.FS

// HopAssets is a copy of the HOP asset manifest published at
// https://hel.ink/hop-assets/apps.json (fonts, app marks and their SRI hashes).
// Refresh it with scripts/sync-hop-assets.sh.
//
//go:embed hopassets/apps.json
var HopAssets []byte

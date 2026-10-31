package app

import (
	"context"
	"net/http"
	"strconv"
	"time"
)

// Dedicated pages on PixelHop's own domain (U-07): about, features, FAQ,
// contact and a live status page. Terms and privacy stay on /legal.

func (a *App) about(w http.ResponseWriter, r *http.Request) {
	a.renderPage(w, r, "about", page{Title: "About", Desc: a.cfg.SiteName + " is free, open-source image hosting from the HOP family: no ads, no trackers, location data removed."})
}

func (a *App) features(w http.ResponseWriter, r *http.Request) {
	a.renderPage(w, r, "features", page{Title: "Features", Desc: "Image, video and PDF hosting, albums, a photo studio and an API, all free on " + a.cfg.SiteName + "."})
}

func (a *App) faq(w http.ResponseWriter, r *http.Request) {
	a.renderPage(w, r, "faq", page{Title: "FAQ", Desc: "Common questions about " + a.cfg.SiteName + ": cost, accounts, privacy and self-hosting."})
}

func (a *App) contact(w http.ResponseWriter, r *http.Request) {
	a.renderPage(w, r, "contact", page{Title: "Contact", Data: a.cfg.AdminEmail, Desc: "Report a bug, suggest an idea or send a security report to " + a.cfg.SiteName + "."})
}

type statusData struct {
	Version string
	Since   string
	DBOK    bool
}

var processStart = time.Now()

func (a *App) status(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	d := statusData{Version: Version, Since: processStart.UTC().Format("2 Jan 2006 15:04 MST"), DBOK: a.db.PingContext(ctx) == nil}
	w.Header().Set("Cache-Control", "no-store")
	if !d.DBOK {
		w.Header().Set("Retry-After", strconv.Itoa(60))
	}
	a.renderPage(w, r, "status", page{Title: "Status", Data: d, Desc: "Is " + a.cfg.SiteName + " up? Live status from the server."})
}

// backPaths are the inner pages that get a "Back to PixelHop" link above their content.
var backPaths = map[string]bool{
	"/changelog": true,
	"/tools":     true, "/explore": true, "/help": true, "/legal": true, "/docs": true,
	"/about": true, "/features": true, "/faq": true, "/contact": true, "/status": true,
}

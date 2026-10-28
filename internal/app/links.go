package app

import (
	"context"
	"net/http"
	"net/url"
	"strings"
)

// Shared links can live on another host than the app (PIXELHOP_LINK_URL,
// e.g. links on p.hel.ink while the app moved to pixel.hel.ink). Both hosts
// reach this server. On the link host only what a shared link needs is
// served: image and album pages, files, thumbnails, QR codes, the password
// and report forms, static files and the API (API keys, ShareX). Everything
// else goes to the app. The link host carries no sign-in, so the owner's own
// tools and private files stay on the app host.

type linkHostKey struct{}

// linkPatterns are the routes a shared link may use on the link host.
var linkPatterns = map[string]bool{
	"GET /{id}":                 true,
	"GET /a/{id}":               true,
	"GET /i/{file}":             true,
	"GET /i/{y}/{m}/{d}/{file}": true,
	"GET /t/{file}":             true,
	"GET /qr/{id}":              true,
	"POST /unlock/{id}":         true,
	"POST /report/{id}":         true,
	"GET /static/":              true,
	"GET /favicon.ico":          true,
	"GET /robots.txt":           true,
	"GET /healthz":              true,
	"GET /oembed":               true,
}

// Kept on the link host for clients set up with its address: API keys and
// ShareX (/api/), signed calls from the control center (/hop/).
var linkPrefixes = []string{"/api/", "/hop/"}

// unregisterWorker replaces the app's service worker on the link host, so
// phones that installed the app from there let go of it.
const unregisterWorker = `self.addEventListener('install', () => self.skipWaiting());
self.addEventListener('activate', e => { e.waitUntil(caches.keys().then(ks => Promise.all(ks.map(k => caches.delete(k)))).then(() => self.registration.unregister())); });
`

// linkHost returns the separate host of shared links, or "" when links
// live on the app itself.
func (a *App) linkHost() string {
	if a.cfg.LinkURL == a.cfg.BaseURL {
		return ""
	}
	u, err := url.Parse(a.cfg.LinkURL)
	if err != nil {
		return ""
	}
	return strings.ToLower(u.Host)
}

func (a *App) withLinkHost(mux *http.ServeMux, next http.Handler) http.Handler {
	host := a.linkHost()
	if host == "" {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.EqualFold(r.Host, host) {
			next.ServeHTTP(w, r)
			return
		}
		if r.URL.Path == "/sw.js" {
			w.Header().Set("Content-Type", "text/javascript; charset=utf-8")
			w.Header().Set("Cache-Control", "no-cache")
			w.Write([]byte(unregisterWorker))
			return
		}
		r2 := r.Clone(context.WithValue(r.Context(), linkHostKey{}, true))
		dropCookie(r2, sessionCookie) // sessions belong to the app host
		for _, p := range linkPrefixes {
			if strings.HasPrefix(r.URL.Path, p) {
				next.ServeHTTP(w, r2)
				return
			}
		}
		if _, pattern := mux.Handler(r2); !linkPatterns[pattern] {
			code := http.StatusFound
			if r.Method != http.MethodGet && r.Method != http.MethodHead {
				code = http.StatusTemporaryRedirect // keep the method and body
			}
			http.Redirect(w, r, a.cfg.BaseURL+r.URL.RequestURI(), code)
			return
		}
		r2.Header.Del("Authorization")
		next.ServeHTTP(w, r2)
	})
}

// dropCookie removes one cookie from the request headers.
func dropCookie(r *http.Request, name string) {
	cs := r.Cookies()
	r.Header.Del("Cookie")
	for _, c := range cs {
		if c.Name != name {
			r.AddCookie(c)
		}
	}
}

// onLinkHost reports whether the request came in through the link host.
func onLinkHost(r *http.Request) bool {
	v, _ := r.Context().Value(linkHostKey{}).(bool)
	return v
}

func (a *App) pageURL(id string) string  { return a.cfg.LinkURL + "/" + id }
func (a *App) albumURL(id string) string { return a.cfg.LinkURL + "/a/" + id }

// toLinkHost sends a visitor who opened a shared page on the app host to
// the same page on the link host. Signed-in people stay on the app, where
// they have their own tools.
func (a *App) toLinkHost(w http.ResponseWriter, r *http.Request) bool {
	if a.linkHost() == "" || onLinkHost(r) || r.Method != http.MethodGet || currentUser(r) != nil {
		return false
	}
	// Temporary: whether to redirect depends on who asks, so no cache may keep it.
	w.Header().Set("Cache-Control", "private, no-store")
	http.Redirect(w, r, a.cfg.LinkURL+r.URL.RequestURI(), http.StatusFound)
	return true
}

// toApp sends a visitor on the link host to the same address on the app,
// for things only a signed-in owner may open there.
func (a *App) toApp(w http.ResponseWriter, r *http.Request) bool {
	if !onLinkHost(r) {
		return false
	}
	w.Header().Set("Cache-Control", "private, no-store")
	http.Redirect(w, r, a.cfg.BaseURL+r.URL.RequestURI(), http.StatusFound)
	return true
}

// isOurHost reports whether host is the app host or the link host.
func (a *App) isOurHost(host string) bool {
	if base, err := url.Parse(a.cfg.BaseURL); err == nil && strings.EqualFold(host, base.Hostname()) {
		return true
	}
	if lu, err := url.Parse(a.cfg.LinkURL); err == nil && strings.EqualFold(host, lu.Hostname()) {
		return true
	}
	return false
}

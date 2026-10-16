package app

import (
	"errors"
	"fmt"
	"net/http"
)

// PixelHop installs as an app (PWA). On phones it then shows up in the
// share sheet: sharing photos to it uploads them and shows the links.

func (a *App) manifest(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/manifest+json")
	w.Header().Set("Cache-Control", "public, max-age=3600")
	writeJSON(w, map[string]any{
		"name": a.cfg.SiteName, "short_name": a.cfg.SiteName,
		"description": "Share images with one link. Free tools that run in your browser.",
		"id":          "/", "start_url": "/?source=pwa", "scope": "/", "display": "standalone",
		"background_color": "#08090c", "theme_color": "#08090c",
		"icons": []map[string]string{
			{"src": "/static/img/icon-192.png", "sizes": "192x192", "type": "image/png", "purpose": "any maskable"},
			{"src": "/static/img/icon-512.png", "sizes": "512x512", "type": "image/png", "purpose": "any maskable"},
			{"src": "/static/img/logo.svg", "sizes": "any", "type": "image/svg+xml"},
		},
		"shortcuts": []map[string]string{
			{"name": "My images", "url": "/dashboard"},
			{"name": "Studio", "url": "/tools"},
		},
		"share_target": map[string]any{
			"action": "/share", "method": "POST", "enctype": "multipart/form-data",
			"params": map[string]any{"title": "title", "files": []map[string]any{
				{"name": "file", "accept": []string{"image/*", "video/*", "application/pdf", ".jpg", ".jpeg", ".png", ".gif", ".webp", ".heic", ".avif", ".mp4", ".mov", ".pdf"}},
			}},
		},
	})
}

// The service worker caches the look of the site (CSS, JS, icons) and shows
// a small offline page; it never caches uploads, images or API answers.
const serviceWorker = `const V = 'ph-%s';
const SHELL = ['/static/css/app.css?v=%s', '/static/js/app.js?v=%s', '/static/js/theme.js?v=%s', '/static/img/logo.svg?v=%s', '/static/img/icon-192.png'];
self.addEventListener('install', e => { e.waitUntil(caches.open(V).then(c => c.addAll(SHELL)).then(() => self.skipWaiting())); });
self.addEventListener('activate', e => { e.waitUntil(caches.keys().then(ks => Promise.all(ks.filter(k => k !== V).map(k => caches.delete(k)))).then(() => self.clients.claim())); });
self.addEventListener('fetch', e => {
  const r = e.request, u = new URL(r.url);
  if (r.method !== 'GET' || u.origin !== location.origin) return;
  if (u.pathname.startsWith('/static/')) { e.respondWith(caches.match(r).then(m => m || fetch(r))); return; }
  if (r.mode === 'navigate') e.respondWith(fetch(r).catch(() => new Response('<!doctype html><meta name="viewport" content="width=device-width"><title>Offline</title><body style="font:16px system-ui;background:#08090c;color:#e8eaf0;display:grid;place-items:center;min-height:90vh;text-align:center"><div><h1>You are offline</h1><p>Your links still work for everyone else. Try again when you are back online.</p></div>', { headers: { 'Content-Type': 'text/html; charset=utf-8' } })));
});
`

func (a *App) serviceWorker(w http.ResponseWriter, r *http.Request) {
	v := Version + "-" + startedAt
	w.Header().Set("Content-Type", "text/javascript; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	w.Write([]byte(fmt.Sprintf(serviceWorker, v, v, v, v, v)))
}

// share receives photos from the phone's share sheet.
func (a *App) share(w http.ResponseWriter, r *http.Request) {
	results, albumID, err := a.handleUpload(r)
	var ue userError
	if err != nil {
		if !errors.As(err, &ue) {
			a.log.Error("share upload", "err", err)
			ue.msg = "Upload failed on our side. Please try again."
		}
		a.renderPage(w, r, "uploaded", page{Title: "Upload failed", NoIndex: true, Data: map[string]any{"Error": ue.msg}})
		return
	}
	type item struct {
		ID, Page, Direct, Thumb, Delete, Name, Error string
	}
	var items []item
	for _, res := range results {
		if res.Err != nil {
			msg := "couldn't save this file"
			if errors.As(res.Err, &ue) {
				msg = ue.msg
			}
			items = append(items, item{Name: res.Name, Error: msg})
			continue
		}
		it := item{ID: res.Image.ID, Page: a.cfg.BaseURL + "/" + res.Image.ID, Direct: a.directURL(res.Image), Thumb: a.thumbURL(res.Image), Name: res.Name}
		if res.DeleteToken != "" {
			it.Delete = a.cfg.BaseURL + "/delete/" + res.Image.ID + "/" + res.DeleteToken
		}
		items = append(items, it)
	}
	album := ""
	if albumID != "" {
		album = a.cfg.BaseURL + "/a/" + albumID
	}
	a.renderPage(w, r, "uploaded", page{Title: "Uploaded", NoIndex: true, Data: map[string]any{"Items": items, "Album": album}})
}

package app

import (
	"database/sql"
	"encoding/json"
	"encoding/xml"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/navi-crwn/pixelhop/internal/db"
)

// Explore, sitemap, oEmbed and the public gallery API. Only images their
// owners put on their profile ("listed") ever show up here.

func (a *App) explore(w http.ResponseWriter, r *http.Request) {
	pg, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if pg < 1 {
		pg = 1
	}
	const per = 48
	ims, total, err := a.db.Images(r.Context(), db.ImageQuery{Listed: true, PublicOwner: true, Limit: per, Offset: (pg - 1) * per})
	if err != nil {
		a.serverError(w, r, err)
		return
	}
	owners := map[int64]*owner{}
	for _, im := range ims {
		if _, ok := owners[im.UserID.Int64]; !ok {
			owners[im.UserID.Int64] = a.ownerOf(r.Context(), im.UserID)
		}
	}
	a.renderPage(w, r, "explore", page{Title: "Explore", Desc: "Images people chose to share publicly on " + a.cfg.SiteName + ".",
		Data: map[string]any{"Images": ims, "Owners": owners, "Page": pg, "Pages": (total + per - 1) / per, "Total": total}})
}

type sitemapURL struct {
	Loc     string `xml:"loc"`
	LastMod string `xml:"lastmod,omitempty"`
}

func (a *App) sitemap(w http.ResponseWriter, r *http.Request) {
	urls := []sitemapURL{}
	for _, p := range []string{"/", "/tools", "/tools/batch", "/explore", "/help", "/docs", "/legal", "/about", "/features", "/faq", "/contact"} {
		urls = append(urls, sitemapURL{Loc: a.cfg.BaseURL + p})
	}
	rows, err := a.db.QueryContext(r.Context(), `SELECT username FROM users WHERE username IS NOT NULL AND show_owner=1 AND status='active'
		AND EXISTS (SELECT 1 FROM images WHERE user_id=users.id AND listed=1 AND hidden=0 AND private=0 AND pass_hash='')`)
	if err == nil {
		for rows.Next() {
			var u string
			rows.Scan(&u)
			urls = append(urls, sitemapURL{Loc: a.cfg.BaseURL + "/u/" + u})
		}
		rows.Close()
	}
	ims, _, _ := a.db.Images(r.Context(), db.ImageQuery{Listed: true, PublicOwner: true, Limit: 5000})
	for _, im := range ims {
		urls = append(urls, sitemapURL{Loc: a.pageURL(im.ID), LastMod: time.Unix(im.CreatedAt, 0).UTC().Format("2006-01-02")})
	}
	w.Header().Set("Content-Type", "application/xml; charset=utf-8")
	w.Write([]byte(xml.Header))
	enc := xml.NewEncoder(w)
	enc.Encode(struct {
		XMLName xml.Name     `xml:"urlset"`
		NS      string       `xml:"xmlns,attr"`
		URLs    []sitemapURL `xml:"url"`
	}{NS: "http://www.sitemaps.org/schemas/sitemap/0.9", URLs: urls})
}

// oembed lets sites like Discourse, WordPress and Notion embed an image page.
func (a *App) oembed(w http.ResponseWriter, r *http.Request) {
	u, err := url.Parse(r.URL.Query().Get("url"))
	if err != nil || !a.isOurHost(u.Hostname()) {
		apiError(w, http.StatusNotFound, "not one of our links")
		return
	}
	id := strings.Trim(u.Path, "/")
	im, err := a.db.Image(r.Context(), id)
	if err != nil || im.Hidden || im.Locked() || a.mediaHasWarning(r, im) || !im.IsImage() || (im.ExpiresAt > 0 && im.ExpiresAt < time.Now().Unix()) {
		apiError(w, http.StatusNotFound, "not found")
		return
	}
	out := map[string]any{"version": "1.0", "type": "photo", "provider_name": a.cfg.SiteName, "provider_url": a.cfg.BaseURL,
		"title": im.Title, "url": a.directURL(im), "width": im.Width, "height": im.Height,
		"thumbnail_url": a.thumbURL(im)}
	if o := a.ownerOf(r.Context(), im.UserID); o != nil {
		out["author_name"] = o.Name
		if o.ProfileURL != "" {
			out["author_url"] = a.cfg.BaseURL + o.ProfileURL
		}
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	writeJSON(w, out)
}

// apiPublicGallery is what a HEL.ink bio block (or any site) reads to show
// someone's chosen images: GET /api/v1/users/{username}/images
func (a *App) apiPublicGallery(w http.ResponseWriter, r *http.Request) {
	u, err := a.db.UserByUsername(r.Context(), r.PathValue("username"))
	if err != nil || !u.ShowOwner || u.Status != "active" {
		apiError(w, http.StatusNotFound, "no such profile")
		return
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 || limit > 60 {
		limit = 24
	}
	ims, total, _ := a.db.Images(r.Context(), db.ImageQuery{UserID: u.ID, Listed: true, Limit: limit})
	list := make([]map[string]any, 0, len(ims))
	for _, im := range ims {
		list = append(list, map[string]any{"id": im.ID, "title": im.Title, "alt": im.Alt, "width": im.Width, "height": im.Height,
			"url": a.directURL(im), "thumb_url": a.thumbURL(im), "page_url": a.pageURL(im.ID), "color": im.Color})
	}
	o := a.ownerOf(r.Context(), sql.NullInt64{Int64: u.ID, Valid: true})
	w.Header().Set("Cache-Control", "public, max-age=300")
	apiOK(w, map[string]any{"username": u.Username.String, "name": o.Name, "avatar": absURL(a.cfg.BaseURL, o.Avatar),
		"profile_url": a.cfg.BaseURL + "/u/" + u.Username.String, "total": total, "images": list})
}

func absURL(base, u string) string {
	if strings.HasPrefix(u, "/") {
		return base + u
	}
	return u
}

func writeJSON(w http.ResponseWriter, v any) { json.NewEncoder(w).Encode(v) }

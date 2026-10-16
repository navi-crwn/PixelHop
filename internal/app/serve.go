package app

import (
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"rsc.io/qr"

	"github.com/navi-crwn/pixelhop/internal/db"
	"github.com/navi-crwn/pixelhop/internal/storage"
)

// lookup finds a live image, or writes the right error page.
func (a *App) lookup(w http.ResponseWriter, r *http.Request, id string, page bool) *db.Image {
	if !validID(id) {
		a.notFound(w, r)
		return nil
	}
	im, err := a.db.Image(r.Context(), id)
	switch {
	case errors.Is(err, db.ErrNotFound) || (err == nil && im.ExpiresAt > 0 && im.ExpiresAt <= time.Now().Unix()):
		a.notFound(w, r)
		return nil
	case err != nil:
		a.serverError(w, r, err)
		return nil
	case im.Hidden && !(currentUser(r) != nil && currentUser(r).IsAdmin()):
		if page {
			a.fail(w, r, http.StatusUnavailableForLegalReasons, "This image was removed by the moderators.")
		} else {
			http.Error(w, "removed", http.StatusUnavailableForLegalReasons)
		}
		return nil
	}
	return im
}

func (a *App) countView(r *http.Request, im *db.Image) {
	if a.views.first(clientIP(r) + "|" + im.ID) {
		a.db.AddView(r.Context(), im.ID)
	}
}

// serveFull sends /i/{id}.{ext}.
func (a *App) serveFull(w http.ResponseWriter, r *http.Request) {
	id, _, _ := strings.Cut(r.PathValue("file"), ".")
	im := a.lookup(w, r, id, false)
	if im == nil {
		return
	}
	if r.Header.Get("Range") == "" && r.Header.Get("Sec-Fetch-Dest") != "image" {
		// Direct hits (not <img> on our own pages) count as views too.
		a.countView(r, im)
	}
	a.sendObject(w, r, im, im.KeyFull, im.Mime)
}

func (a *App) serveThumb(w http.ResponseWriter, r *http.Request) {
	id, _, _ := strings.Cut(r.PathValue("file"), ".")
	im := a.lookup(w, r, id, false)
	if im == nil {
		return
	}
	mime := "image/jpeg"
	if strings.HasSuffix(im.KeyThumb, ".png") {
		mime = "image/png"
	} else if strings.HasSuffix(im.KeyThumb, ".webp") {
		mime = "image/webp"
	}
	a.sendObject(w, r, im, im.KeyThumb, mime)
}

// serveLegacy keeps PixelHop 2 links (/i/2026/08/27/name_AbC123_original.webp) working.
func (a *App) serveLegacy(w http.ResponseWriter, r *http.Request) {
	file := r.PathValue("file")
	base, _, _ := strings.Cut(file, ".")
	i := strings.LastIndexByte(base, '_')
	if i < 0 {
		a.notFound(w, r)
		return
	}
	id, size := base[:i], base[i+1:]
	im := a.lookup(w, r, id, false)
	if im == nil {
		return
	}
	if size == "thumb" || size == "medium" {
		a.sendObject(w, r, im, im.KeyThumb, "")
		return
	}
	a.sendObject(w, r, im, im.KeyFull, im.Mime)
}

func (a *App) sendObject(w http.ResponseWriter, r *http.Request, im *db.Image, key, mime string) {
	b := a.backends[im.Backend]
	if b == nil {
		a.notFound(w, r)
		return
	}
	h := w.Header()
	// Images can never run script, even if someone smuggles markup in.
	h.Set("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'; sandbox")
	h.Set("Cross-Origin-Resource-Policy", "cross-origin")
	h.Set("Access-Control-Allow-Origin", "*")
	h.Del("X-Frame-Options")
	if im.ExpiresAt > 0 {
		h.Set("Cache-Control", "public, max-age=300")
	} else {
		h.Set("Cache-Control", "public, max-age=2592000")
	}
	if mime != "" {
		h.Set("Content-Type", mime)
	}
	if r.URL.Query().Has("download") {
		name := im.ID + "." + im.Ext
		h.Set("Content-Disposition", `attachment; filename="`+name+`"`)
	}

	if s3, ok := b.(*storage.S3); ok {
		if pub := s3.PublicURL(key); pub != "" && !r.URL.Query().Has("download") {
			http.Redirect(w, r, pub, http.StatusFound)
			return
		}
	}
	if fs, ok := b.(storage.FileServer); ok {
		if err := fs.ServeFile(w, r, key); err != nil {
			a.missingObject(w, r, im, err)
		}
		return
	}
	obj, err := b.Get(r.Context(), key, r.Header.Get("Range"))
	if err != nil {
		a.missingObject(w, r, im, err)
		return
	}
	defer obj.Body.Close()
	if mime == "" && obj.ContentType != "" {
		h.Set("Content-Type", obj.ContentType)
	}
	if obj.Size >= 0 {
		h.Set("Content-Length", strconv.FormatInt(obj.Size, 10))
	}
	if obj.ETag != "" {
		h.Set("ETag", obj.ETag)
	}
	if obj.LastModified != "" {
		h.Set("Last-Modified", obj.LastModified)
	}
	h.Set("Accept-Ranges", "bytes")
	if obj.Status == http.StatusPartialContent {
		h.Set("Content-Range", obj.ContentRange)
	}
	w.WriteHeader(obj.Status)
	if r.Method != http.MethodHead {
		io.Copy(w, obj.Body)
	}
}

func (a *App) missingObject(w http.ResponseWriter, r *http.Request, im *db.Image, err error) {
	if !errors.Is(err, storage.ErrNotFound) {
		a.log.Error("storage", "id", im.ID, "err", err)
		http.Error(w, "storage unavailable", http.StatusBadGateway)
		return
	}
	http.Error(w, "not found", http.StatusNotFound)
}

// ---- pages ----

type viewData struct {
	Owner    *owner
	Image    *db.Image
	Album    *db.Album
	Siblings []*db.Image
	Mine     bool // the viewer may rename/delete/list it
	Links    links
}

type links struct {
	Page, Direct, Thumb, Markdown, HTML, BBCode, Download string
}

func (a *App) linksFor(im *db.Image) links {
	page := a.cfg.BaseURL + "/" + im.ID
	direct := a.directURL(im)
	return links{
		Page: page, Direct: direct, Thumb: a.thumbURL(im), Download: direct + "?download",
		Markdown: "![" + mdEscape(im.Title) + "](" + direct + ")",
		HTML:     `<a href="` + page + `"><img src="` + direct + `" alt="` + htmlAttr(im.Title) + `"></a>`,
		BBCode:   "[url=" + page + "][img]" + direct + "[/img][/url]",
	}
}

func (a *App) directURL(im *db.Image) string { return a.cfg.BaseURL + "/i/" + im.ID + "." + im.Ext }
func (a *App) thumbURL(im *db.Image) string {
	ext := im.KeyThumb[strings.LastIndexByte(im.KeyThumb, '.')+1:]
	if strings.Contains(ext, "/") || len(ext) > 5 {
		ext = "jpg"
	}
	return a.cfg.BaseURL + "/t/" + im.ID + "." + ext
}

func mdEscape(s string) string { return strings.NewReplacer("[", `\[`, "]", `\]`).Replace(s) }
func htmlAttr(s string) string {
	return strings.NewReplacer("&", "&amp;", `"`, "&quot;", "<", "&lt;", ">", "&gt;").Replace(s)
}

func (a *App) view(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	// PixelHop 2 page links sometimes carried an extension or size suffix.
	id = strings.TrimSuffix(id, ".php")
	im := a.lookup(w, r, id, true)
	if im == nil {
		return
	}
	a.countView(r, im)
	u := currentUser(r)
	d := viewData{Image: im, Links: a.linksFor(im), Mine: u != nil && (im.UserID.Int64 == u.ID || u.IsAdmin()),
		Owner: a.ownerOf(r.Context(), im.UserID)}
	if im.AlbumID.Valid {
		if al, err := a.db.Album(r.Context(), im.AlbumID.String); err == nil {
			d.Album = al
			d.Siblings, _, _ = a.db.Images(r.Context(), db.ImageQuery{AlbumID: al.ID, Sort: "old", Limit: 50})
		}
	}
	title := im.Title
	if title == "" {
		title = im.ID
	}
	desc := humanDims(im.Width, im.Height) + " · " + humanBytes(im.Size) + " · shared on " + a.cfg.SiteName
	if d.Owner != nil {
		desc = "Shared by " + d.Owner.Name + " · " + desc
	}
	a.renderPage(w, r, "view", page{Title: title, Data: d, OGImage: a.directURL(im), OGType: "image", Desc: desc})
}

// legacyResult serves PixelHop 2's upload result links (/result?id=a,b,c):
// one image redirects to its page, several are shown like an album.
func (a *App) legacyResult(w http.ResponseWriter, r *http.Request) {
	var ims []*db.Image
	now := time.Now().Unix()
	for _, id := range strings.FieldsFunc(r.URL.Query().Get("id"), func(c rune) bool { return c == ',' || c == ' ' }) {
		if len(ims) >= 50 || !validID(id) {
			continue
		}
		if im, err := a.db.Image(r.Context(), id); err == nil && !im.Hidden && (im.ExpiresAt == 0 || im.ExpiresAt > now) {
			ims = append(ims, im)
		}
	}
	switch len(ims) {
	case 0:
		a.notFound(w, r)
	case 1:
		http.Redirect(w, r, "/"+ims[0].ID, http.StatusMovedPermanently)
	default:
		title := strconv.Itoa(len(ims)) + " images"
		a.renderPage(w, r, "album", page{Title: title, OGImage: a.directURL(ims[0]),
			Desc: title + " on " + a.cfg.SiteName,
			Data: albumData{Album: &db.Album{Title: title, CreatedAt: ims[0].CreatedAt}, Images: ims, Link: a.cfg.BaseURL + r.URL.RequestURI()}})
	}
}

type albumData struct {
	By     *owner
	Album  *db.Album
	Images []*db.Image
	Owner  bool
	Link   string
}

func (a *App) album(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !validID(id) {
		a.notFound(w, r)
		return
	}
	al, err := a.db.Album(r.Context(), id)
	if err != nil {
		a.notFound(w, r)
		return
	}
	ims, _, _ := a.db.Images(r.Context(), db.ImageQuery{AlbumID: id, Sort: "old", Limit: 200})
	live := ims[:0]
	now := time.Now().Unix()
	for _, im := range ims {
		if !im.Hidden && (im.ExpiresAt == 0 || im.ExpiresAt > now) {
			live = append(live, im)
		}
	}
	if len(live) == 0 {
		a.notFound(w, r)
		return
	}
	u := currentUser(r)
	title := al.Title
	if title == "" {
		title = strconv.Itoa(len(live)) + " images"
	}
	by := a.ownerOf(r.Context(), al.UserID)
	a.renderPage(w, r, "album", page{Title: title, OGImage: a.directURL(live[0]),
		Desc: albumDesc(len(live), by, a.cfg.SiteName),
		Data: albumData{Album: al, Images: live, Link: a.cfg.BaseURL + "/a/" + al.ID, Owner: u != nil && (al.UserID.Int64 == u.ID || u.IsAdmin()), By: by}})
}

// qr draws a QR code that opens the image page; ?a=1 points at an album.
func (a *App) qr(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSuffix(r.PathValue("id"), ".png")
	if !validID(id) {
		a.notFound(w, r)
		return
	}
	target := a.cfg.BaseURL + "/" + id
	if r.URL.Query().Has("a") {
		target = a.cfg.BaseURL + "/a/" + id
	}
	code, err := qr.Encode(target, qr.M)
	if err != nil {
		a.serverError(w, r, err)
		return
	}
	code.Scale = 8
	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Cache-Control", "public, max-age=86400")
	w.Write(code.PNG())
}

// ---- deleting with the guest's secret link ----

func (a *App) tokenImage(w http.ResponseWriter, r *http.Request) *db.Image {
	im := a.lookup(w, r, r.PathValue("id"), true)
	if im == nil {
		return nil
	}
	if im.OwnerToken == "" || hashToken(r.PathValue("token")) != im.OwnerToken {
		a.fail(w, r, http.StatusForbidden, "This delete link isn't valid.")
		return nil
	}
	return im
}

func (a *App) deleteConfirm(w http.ResponseWriter, r *http.Request) {
	if im := a.tokenImage(w, r); im != nil {
		a.render(w, r, "delete", "Delete image", im)
	}
}

func (a *App) deleteWithToken(w http.ResponseWriter, r *http.Request) {
	im := a.tokenImage(w, r)
	if im == nil {
		return
	}
	if err := a.removeImage(r.Context(), im); err != nil {
		a.serverError(w, r, err)
		return
	}
	if strings.Contains(r.Header.Get("Accept"), "application/json") {
		apiOK(w, map[string]any{"deleted": im.ID})
		return
	}
	a.render(w, r, "deleted", "Deleted", nil)
}

var reportReasons = map[string]bool{"nsfw": true, "illegal": true, "privacy": true, "copyright": true, "spam": true, "other": true}

func (a *App) report(w http.ResponseWriter, r *http.Request) {
	im := a.lookup(w, r, r.PathValue("id"), true)
	if im == nil {
		return
	}
	reason := r.FormValue("reason")
	if !reportReasons[reason] {
		apiError(w, http.StatusBadRequest, "pick a reason")
		return
	}
	ip := clientIP(r)
	if !a.limits.allow("report:"+ip, 10, time.Hour) || a.db.RecentReport(r.Context(), im.ID, ip) {
		apiOK(w, map[string]any{"reported": true}) // say thanks either way; don't help flooders
		return
	}
	if err := a.db.InsertReport(r.Context(), &db.Report{ImageID: im.ID, Reason: reason, Note: trim(r.FormValue("note"), 1000), IP: ip}); err != nil {
		apiError(w, http.StatusInternalServerError, "couldn't save the report")
		return
	}
	apiOK(w, map[string]any{"reported": true})
}

func albumDesc(n int, by *owner, site string) string {
	if by != nil {
		return "An album of " + strconv.Itoa(n) + " images shared by " + by.Name + " on " + site
	}
	return "An album of " + strconv.Itoa(n) + " images on " + site
}

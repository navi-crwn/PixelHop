package app

import (
	"errors"
	"io"
	"net/http"
	"net/url"
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

// serveFull sends /i/{id}.{ext}.
func (a *App) serveFull(w http.ResponseWriter, r *http.Request) {
	id, _, _ := strings.Cut(r.PathValue("file"), ".")
	im := a.lookup(w, r, id, false)
	if im == nil || !a.fileAllowed(w, r, im) {
		return
	}
	if im.IsAdult() && !a.hasGatePassed(r, im.ID) {
		ref := r.Referer()
		fromOurHost := false
		if u, err := url.Parse(ref); err == nil && u.Host != "" {
			fromOurHost = a.isOurHost(u.Hostname())
		}
		if !fromOurHost {
			http.Redirect(w, r, a.pageURL(im.ID), http.StatusSeeOther)
			return
		}
	}
	if a.transformRequested(r) && im.IsImage() {
		a.serveTransformed(w, r, im)
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
	if im == nil || !a.fileAllowed(w, r, im) {
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
	if im == nil || !a.fileAllowed(w, r, im) {
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
	if im.Kind() == "pdf" && key == im.KeyFull {
		// Browsers refuse to show PDFs inside a sandbox; their viewer runs apart from the page.
		h.Set("Content-Security-Policy", "default-src 'none'; object-src 'self'; plugin-types application/pdf")
		h.Set("X-Content-Type-Options", "nosniff")
	} else {
		h.Set("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'; sandbox")
	}
	h.Set("Cross-Origin-Resource-Policy", "cross-origin")
	h.Set("Access-Control-Allow-Origin", "*")
	h.Del("X-Frame-Options")
	if im.Locked() {
		h.Set("Cache-Control", "private, no-store") // never let a CDN keep a private image
	} else if im.ExpiresAt > 0 {
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
	Removed    []string // what metadata was stripped, in words
	Owner      *owner
	Image      *db.Image
	Album      *db.Album
	Siblings   []*db.Image
	Mine       bool // the viewer may rename/delete/list it
	Optimizing bool // a video still being shrunk in the background
	Links      links
	GatePassed bool
}

type links struct {
	Page, Direct, Thumb, Markdown, HTML, BBCode, Download string
}

func (a *App) linksFor(im *db.Image) links {
	page := a.pageURL(im.ID)
	direct := a.directURL(im)
	alt := im.Alt
	if alt == "" {
		alt = im.Title
	}
	return links{
		Page: page, Direct: direct, Thumb: a.thumbURL(im), Download: direct + "?download",
		Markdown: "![" + mdEscape(alt) + "](" + direct + ")",
		HTML:     `<a href="` + page + `"><img src="` + direct + `" alt="` + htmlAttr(alt) + `" width="` + strconv.Itoa(im.Width) + `" height="` + strconv.Itoa(im.Height) + `"></a>`,
		BBCode:   "[url=" + page + "][img]" + direct + "[/img][/url]",
	}
}

// previewURL is the picture for link previews: the image, or a video's poster / a PDF's first page.
func (a *App) previewURL(im *db.Image) string {
	if im.IsImage() {
		return a.directURL(im)
	}
	return a.thumbURL(im)
}

func (a *App) directURL(im *db.Image) string { return a.cfg.LinkURL + "/i/" + im.ID + "." + im.Ext }
func (a *App) thumbURL(im *db.Image) string {
	ext := im.KeyThumb[strings.LastIndexByte(im.KeyThumb, '.')+1:]
	if strings.Contains(ext, "/") || len(ext) > 5 {
		ext = "jpg"
	}
	return a.cfg.LinkURL + "/t/" + im.ID + "." + ext
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
	if !im.Private && a.toLinkHost(w, r) {
		return
	}
	if !a.canSee(r, im) {
		if im.Private {
			if a.toApp(w, r) { // its owner may be signed in there
				return
			}
			a.notFound(w, r)
		} else {
			a.renderLocked(w, r, im, "")
		}
		return
	}
	a.countView(r, im)
	u := currentUser(r)
	mine := u != nil && (im.UserID.Int64 == u.ID || u.IsAdmin())
	gatePassed := !im.HasWarning() || a.hasGatePassed(r, im.ID) || mine
	d := viewData{Image: im, Links: a.linksFor(im), Mine: mine, GatePassed: gatePassed,
		Owner: a.ownerOf(r.Context(), im.UserID), Optimizing: im.Kind() == "video" && a.db.MediaJobPending(r.Context(), im.ID)}
	for _, k := range removedList(im.Stripped) {
		if w := removedWords[k]; w != "" {
			d.Removed = append(d.Removed, w)
		}
	}
	if im.AlbumID.Valid {
		if al, err := a.db.Album(r.Context(), im.AlbumID.String); err == nil {
			d.Album = al
			d.Siblings, _, _ = a.db.Images(r.Context(), db.ImageQuery{AlbumID: al.ID, Sort: "album", Limit: 50})
		}
	}
	title := im.Title
	if title == "" {
		title = im.ID
	}
	desc := humanDims(im.Width, im.Height) + " · " + humanBytes(im.Size) + " · shared on " + a.cfg.SiteName
	switch im.Kind() {
	case "video":
		desc = "Video · " + desc
	case "pdf":
		desc = "PDF · " + humanBytes(im.Size) + " · shared on " + a.cfg.SiteName
	}
	if d.Owner != nil {
		desc = "Shared by " + d.Owner.Name + " · " + desc
	}
	og := a.directURL(im)
	if !im.IsImage() {
		og = a.thumbURL(im)
	}
	if im.Locked() || im.IsAdult() {
		og = "" // previews of locked or adult images show no picture
	}
	if im.IsAdult() {
		desc = "Content warning: 18+ adult material · shared on " + a.cfg.SiteName
	}
	a.renderPage(w, r, "view", page{Title: title, Canon: a.pageURL(im.ID), Data: d, OGImage: og, OGType: "image", Desc: desc, NoIndex: im.Locked() || im.IsAdult()})
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
		if im, err := a.db.Image(r.Context(), id); err == nil && !im.Hidden && (im.ExpiresAt == 0 || im.ExpiresAt > now) && a.canSee(r, im) {
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
		a.renderPage(w, r, "album", page{Title: title, OGImage: a.previewURL(ims[0]),
			Desc: title + " on " + a.cfg.SiteName,
			Data: albumData{Album: &db.Album{Title: title, CreatedAt: ims[0].CreatedAt}, Images: ims, Link: a.cfg.BaseURL + r.URL.RequestURI()}})
	}
}

type albumData struct {
	By         *owner
	Album      *db.Album
	Images     []*db.Image
	Owner      bool
	Link       string
	GatePassed bool
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
	ims, _, _ := a.db.Images(r.Context(), db.ImageQuery{AlbumID: id, Sort: "album", Limit: 200})
	live := ims[:0]
	now := time.Now().Unix()
	for _, im := range ims {
		if !im.Hidden && (im.ExpiresAt == 0 || im.ExpiresAt > now) && a.canSee(r, im) {
			live = append(live, im)
		}
	}
	if len(live) == 0 {
		if a.toApp(w, r) {
			return
		}
		a.notFound(w, r)
		return
	}
	if a.toLinkHost(w, r) {
		return
	}
	u := currentUser(r)
	title := al.Title
	if title == "" {
		title = strconv.Itoa(len(live)) + " images"
	}
	by := a.ownerOf(r.Context(), al.UserID)
	cover := live[0]
	for _, im := range live {
		if im.ID == al.CoverID {
			cover = im
		}
	}
	mine := u != nil && (al.UserID.Int64 == u.ID || u.IsAdmin())
	gatePassed := !al.HasWarning() || a.hasAlbumGatePassed(r, al.ID) || mine
	og := a.previewURL(cover)
	desc := albumDesc(len(live), by, a.cfg.SiteName)
	if al.IsAdult() {
		og = ""
		desc = "Content warning: 18+ adult material · shared on " + a.cfg.SiteName
	}
	a.renderPage(w, r, "album", page{Title: title, Canon: a.albumURL(al.ID), OGImage: og,
		Desc: desc, NoIndex: al.IsAdult(),
		Data: albumData{Album: al, Images: live, Link: a.albumURL(al.ID), Owner: mine, By: by, GatePassed: gatePassed}})
}

// qr draws a QR code that opens the image page; ?a=1 points at an album.
func (a *App) qr(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSuffix(r.PathValue("id"), ".png")
	if !validID(id) {
		a.notFound(w, r)
		return
	}
	target := a.pageURL(id)
	if r.URL.Query().Has("a") {
		target = a.albumURL(id)
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

// fileAllowed stops private and password-protected files for visitors who
// may not see them (404 for private, 403 for "needs a password").
func (a *App) fileAllowed(w http.ResponseWriter, r *http.Request, im *db.Image) bool {
	if a.canSee(r, im) {
		return true
	}
	w.Header().Set("Cache-Control", "private, no-store")
	if im.Private {
		http.Error(w, "not found", http.StatusNotFound)
	} else {
		http.Error(w, "this image is password protected", http.StatusForbidden)
	}
	return false
}

func gateCookieName(id string) string {
	return "hop_gate_ph_" + id
}

func albumGateCookieName(id string) string {
	return "hop_gate_pha_" + id
}

func (a *App) hasGatePassed(r *http.Request, id string) bool {
	c, err := r.Cookie(gateCookieName(id))
	return err == nil && c.Value == "1"
}

func (a *App) hasAlbumGatePassed(r *http.Request, id string) bool {
	c, err := r.Cookie(albumGateCookieName(id))
	return err == nil && c.Value == "1"
}

func (a *App) gateConfirm(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	im := a.lookup(w, r, id, true)
	if im == nil {
		return
	}
	maxAge := 86400
	http.SetCookie(w, &http.Cookie{
		Name:     gateCookieName(im.ID),
		Value:    "1",
		Path:     "/",
		HttpOnly: true,
		Secure:   a.cfg.SecureCookies(),
		SameSite: http.SameSiteLaxMode,
		MaxAge:   maxAge,
	})
	http.Redirect(w, r, a.pageURL(im.ID), http.StatusSeeOther)
}

func (a *App) albumGateConfirm(w http.ResponseWriter, r *http.Request) {
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
	maxAge := 86400
	http.SetCookie(w, &http.Cookie{
		Name:     albumGateCookieName(al.ID),
		Value:    "1",
		Path:     "/",
		HttpOnly: true,
		Secure:   a.cfg.SecureCookies(),
		SameSite: http.SameSiteLaxMode,
		MaxAge:   maxAge,
	})
	http.Redirect(w, r, a.albumURL(al.ID), http.StatusSeeOther)
}

package app

import (
	"encoding/json"
	"errors"
	"maps"
	"net/http"
	"strconv"
	"strings"

	"github.com/navi-crwn/pixelhop/internal/db"
)

// The JSON API. Every response has "success"; data sits under "data", which
// keeps ShareX configs and scripts written for PixelHop 2 working.

func apiOK(w http.ResponseWriter, data any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"success": true, "data": data})
}

func apiError(w http.ResponseWriter, code int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]any{"success": false, "error": msg})
}

func (a *App) imageJSON(im *db.Image) map[string]any {
	l := a.linksFor(im)
	m := map[string]any{
		"id": im.ID, "title": im.Title, "filename": im.OrigName, "extension": im.Ext, "mime": im.Mime,
		"size": im.Size, "width": im.Width, "height": im.Height, "color": im.Color,
		"views": im.Views, "created_at": im.CreatedAt,
		"url": l.Direct, "view_url": l.Page, "thumb_url": l.Thumb,
		"urls":  map[string]string{"original": l.Direct, "thumb": l.Thumb, "page": l.Page},
		"embed": map[string]string{"markdown": l.Markdown, "html": l.HTML, "bbcode": l.BBCode},
	}
	if im.ExpiresAt > 0 {
		m["expires_at"] = im.ExpiresAt
	}
	if im.AlbumID.Valid {
		m["album_url"] = a.albumURL(im.AlbumID.String)
	}
	m["listed"] = im.Listed
	m["alt"] = im.Alt
	m["private"] = im.Private
	m["password"] = im.PassHash != ""
	m["removed"] = removedList(im.Stripped)
	return m
}

func (a *App) apiUpload(w http.ResponseWriter, r *http.Request) {
	results, albumID, err := a.handleUpload(r)
	if err != nil {
		var ue userError
		if errors.As(err, &ue) {
			code := http.StatusBadRequest
			if strings.Contains(ue.msg, "fast") {
				code = http.StatusTooManyRequests
			} else if strings.Contains(ue.msg, "sign in") {
				code = http.StatusUnauthorized
			}
			apiError(w, code, ue.msg)
			return
		}
		a.log.Error("upload", "err", err)
		apiError(w, http.StatusInternalServerError, "Upload failed on our side. Please try again.")
		return
	}

	var images []map[string]any
	var errs []map[string]string
	for _, res := range results {
		if res.Err != nil {
			msg := res.Err.Error()
			var ue userError
			if !errors.As(res.Err, &ue) {
				a.log.Error("upload", "file", res.Name, "err", res.Err)
				msg = "couldn't save this file"
			}
			errs = append(errs, map[string]string{"file": res.Name, "error": msg})
			continue
		}
		m := a.imageJSON(res.Image)
		if res.DeleteToken != "" {
			m["delete_url"] = a.cfg.BaseURL + "/delete/" + res.Image.ID + "/" + res.DeleteToken
		}
		m["duplicate"] = res.Duplicate
		images = append(images, m)
	}
	if len(images) == 0 {
		msg := "Upload failed."
		if len(errs) > 0 {
			msg = errs[0]["error"]
		}
		apiError(w, http.StatusBadRequest, msg)
		return
	}

	// Single upload: the image fields sit right in "data" (PixelHop 2 shape).
	data := map[string]any{}
	if len(images) == 1 && len(errs) == 0 {
		maps.Copy(data, images[0]) // a copy: data.images[0] must not point back at data
	}
	data["images"] = images
	if len(errs) > 0 {
		data["errors"] = errs
	}
	if albumID != "" {
		data["album_url"] = a.albumURL(albumID)
	}
	apiOK(w, data)
}

func (a *App) apiUser(w http.ResponseWriter, r *http.Request) *db.User {
	u := currentUser(r)
	if u == nil {
		apiError(w, http.StatusUnauthorized, "sign in or send an API key (Authorization: Bearer ph_...)")
	}
	return u
}

func (a *App) apiImages(w http.ResponseWriter, r *http.Request) {
	u := a.apiUser(w, r)
	if u == nil {
		return
	}
	q := r.URL.Query()
	limit, _ := strconv.Atoi(q.Get("limit"))
	page, _ := strconv.Atoi(q.Get("page"))
	if page < 1 {
		page = 1
	}
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	ims, total, err := a.db.Images(r.Context(), db.ImageQuery{UserID: u.ID, Search: q.Get("q"), Sort: q.Get("sort"), Limit: limit, Offset: (page - 1) * limit})
	if err != nil {
		apiError(w, http.StatusInternalServerError, "couldn't list images")
		return
	}
	out := make([]map[string]any, 0, len(ims))
	for _, im := range ims {
		out = append(out, a.imageJSON(im))
	}
	apiOK(w, map[string]any{"images": out, "total": total, "page": page, "limit": limit})
}

// ownImage loads an image the caller may change (their own, or any for admins).
func (a *App) ownImage(w http.ResponseWriter, r *http.Request) *db.Image {
	u := a.apiUser(w, r)
	if u == nil {
		return nil
	}
	im, err := a.db.Image(r.Context(), r.PathValue("id"))
	if err != nil || (im.UserID.Int64 != u.ID && !u.IsAdmin()) {
		apiError(w, http.StatusNotFound, "image not found")
		return nil
	}
	return im
}

func (a *App) apiImage(w http.ResponseWriter, r *http.Request) {
	if im := a.ownImage(w, r); im != nil {
		apiOK(w, a.imageJSON(im))
	}
}

func (a *App) apiDeleteImage(w http.ResponseWriter, r *http.Request) {
	im := a.ownImage(w, r)
	if im == nil {
		return
	}
	if err := a.removeImage(r.Context(), im); err != nil {
		apiError(w, http.StatusInternalServerError, "couldn't delete")
		return
	}
	a.db.DeleteEmptyAlbums(r.Context())
	apiOK(w, map[string]any{"deleted": im.ID})
}

func (a *App) apiMe(w http.ResponseWriter, r *http.Request) {
	u := a.apiUser(w, r)
	if u == nil {
		return
	}
	used, count := a.db.UserStorage(r.Context(), u.ID)
	apiOK(w, map[string]any{"id": u.ID, "email": u.Email, "name": u.Name, "role": u.Role,
		"images": count, "storage_used": used, "storage_quota": u.QuotaBytes})
}

func (a *App) apiStats(w http.ResponseWriter, r *http.Request) {
	n, views := a.db.PublicStats(r.Context())
	apiOK(w, map[string]any{"images": n, "views": views, "version": Version})
}

// removedList turns "gps,camera" into ["gps","camera"] (empty list, not null).
func removedList(s string) []string {
	out := []string{}
	for _, k := range strings.Split(s, ",") {
		if w, ok := removedWords[k]; ok {
			out = append(out, w)
		} else if k != "" {
			out = append(out, k)
		}
	}
	return out
}

// removedWords describes stripped metadata for people.
var removedWords = map[string]string{
	"gps": "GPS location", "camera": "camera details", "date": "date taken", "software": "editing software",
	"xmp": "XMP data", "iptc": "IPTC data", "comment": "hidden comment", "text": "embedded text",
}

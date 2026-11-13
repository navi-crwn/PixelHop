package app

import (
	"context"
	"encoding/json"
	"errors"
	"maps"
	"net/http"
	"strconv"
	"strings"
	"time"

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

// apiErrorWith is apiError plus extra fields; clients that only read "error" are not affected.
func apiErrorWith(w http.ResponseWriter, code int, msg string, extra map[string]any) {
	body := map[string]any{"success": false, "error": msg}
	for k, v := range extra {
		body[k] = v
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(body)
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
	m["content_warning"] = im.ContentWarning
	m["warning_reason"] = im.WarningReason
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
			var re rateError
			if errors.As(err, &re) {
				retryAfter(w, re)
				code = http.StatusTooManyRequests
			} else if strings.Contains(ue.msg, "fast") {
				code = http.StatusTooManyRequests
			} else if strings.Contains(ue.msg, "sign in") {
				code = http.StatusUnauthorized
			}
			apiError(w, code, ue.msg)
			return
		}
		a.log.Error("upload", "err", err)
		apiError(w, http.StatusInternalServerError, a.tr(r, "Upload failed on our side. Please try again."))
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
		msg := a.tr(r, "Upload failed.")
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
		out := a.imageJSON(im)
		out["tags"] = a.db.ImageTags(r.Context(), im.ID)
		apiOK(w, out)
	}
}

func (a *App) apiDeleteImage(w http.ResponseWriter, r *http.Request) {
	im := a.ownImage(w, r)
	if im == nil {
		return
	}
	// Your own files go to the trash first (restorable for a week); ?permanent=1, or an admin, removes for good.
	if u := currentUser(r); u != nil && im.UserID.Int64 == u.ID && r.URL.Query().Get("permanent") != "1" && !a.db.Trashed(r.Context(), im.ID) {
		if err := a.db.TrashImage(r.Context(), im.ID); err != nil {
			apiError(w, http.StatusInternalServerError, "couldn't delete")
			return
		}
		apiOK(w, map[string]any{"deleted": im.ID, "trashed": true})
		return
	}
	if err := a.removeImage(r.Context(), im); err != nil {
		apiError(w, http.StatusInternalServerError, "couldn't delete")
		return
	}
	a.db.DeleteEmptyAlbums(r.Context())
	apiOK(w, map[string]any{"deleted": im.ID})
}

// apiRestoreImage brings a file back from the trash.
func (a *App) apiRestoreImage(w http.ResponseWriter, r *http.Request) {
	im := a.ownImage(w, r)
	if im == nil {
		return
	}
	if err := a.db.RestoreImage(r.Context(), im.ID); err != nil {
		apiError(w, http.StatusInternalServerError, "couldn't restore")
		return
	}
	apiOK(w, map[string]any{"restored": im.ID})
}

func (a *App) apiMe(w http.ResponseWriter, r *http.Request) {
	u := a.apiUser(w, r)
	if u == nil {
		return
	}
	used, count := a.db.UserStorage(r.Context(), u.ID)
	sum := a.db.UserSummary(r.Context(), u.ID)
	apiOK(w, map[string]any{"id": u.ID, "email": u.Email, "name": u.Name, "role": u.Role,
		"images": count, "storage_used": used, "storage_quota": u.QuotaBytes,
		// what the dashboard shows: live files by kind (the trash left out) and albums
		"files": sum.Images + sum.Videos + sum.PDFs, "kinds": map[string]int{"image": sum.Images, "video": sum.Videos, "pdf": sum.PDFs},
		"albums": sum.Albums, "ai_day": a.aiDayJSON(r.Context(), u.ID)})
}

// aiDayJSON is what is left of today's share for each server tool: {"upscale": {"limit":10,"used":3,"remaining":7,
// "resets_at":"..."}}. resets_at is absent until the member has used the tool in the current window.
func (a *App) aiDayJSON(ctx context.Context, userID int64) map[string]any {
	out := map[string]any{}
	for name, tool := range aiTools {
		limit := tool.perDay
		if name == "remove-background" {
			limit = a.cfg.RembgPerDay
		}
		if limit <= 0 { // no limit set for this tool
			continue
		}
		used, resets, _ := a.db.AIUsed(ctx, userID, name, aiDay, time.Now())
		m := map[string]any{"limit": limit, "used": used, "remaining": max(limit-used, 0)}
		if !resets.IsZero() {
			m["resets_at"] = resets.UTC().Format(time.RFC3339)
		}
		out[name] = m
	}
	return out
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

package app

import (
	"context"
	"errors"
	"io"
	"net/http"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/navi-crwn/pixelhop/internal/db"
)

// POST /hop/media: another HOP app (FileHop's "Save to PixelHop") stores one
// file in a member's library on their behalf.
//
//	POST /hop/media?name=<file name>       the body is the file, as is
//	X-HOP-App, X-HOP-Timestamp, X-HOP-Nonce, X-HOP-Signature   HOP App Protocol v1 (see internal/hop)
//	X-HOP-Actor: <the member's HOP ID>
//
// Only an app listed in PIXELHOP_STORE_CLIENTS that holds its own secret
// (HOP_STORE_SECRET_<APP>) may call it. That secret is not the SSO secret and
// not the picker secret, so no other app can sign as the store client. The
// file goes to the library of the member whose HOP ID the app names, who must
// already have an active account here; there is no way to name another place.
// The size limit, the allowed types, the safety checks (decoding, blocked
// hashes, metadata stripping), the member's quota and the webhooks are those of
// a normal upload. The whole body is held in memory to verify the signature, so
// videos and PDFs are accepted only up to the picture limit.

// hopMediaAnswer is the data of a successful call.
func (a *App) hopMediaAnswer(im *db.Image, duplicate bool) map[string]any {
	return map[string]any{
		"id": im.ID, "url": a.directURL(im), "page_url": a.pageURL(im.ID), "thumb_url": a.thumbURL(im),
		"mime": im.Mime, "bytes": im.Size, "width": im.Width, "height": im.Height, "kind": im.Kind(),
		"title": im.Title, "duplicate": duplicate,
	}
}

func (a *App) hopMedia(w http.ResponseWriter, r *http.Request) {
	if !a.cfg.StoreClientsConfigured() {
		hopFail(w, http.StatusNotFound, "not_configured", "Storing files for other apps is not switched on here.")
		return
	}
	app := r.Header.Get("X-HOP-App")
	if r.Header.Get("X-HOP-Signature") == "" || !a.cfg.StoreClient(app) {
		hopFail(w, http.StatusUnauthorized, "bad_signature", "Unknown calling app.")
		return
	}
	// Cheap limits before the body is read: per calling app and per address.
	if !a.limits.allow("hopmedia:app:"+app, 600, time.Minute) || !a.limits.allow("hopmedia:ip:"+clientIP(r), 600, time.Minute) {
		w.Header().Set("Retry-After", "60")
		hopFail(w, http.StatusTooManyRequests, "rate_limited", "Too many calls. Wait a minute.")
		return
	}
	limit := a.cfg.MaxUploadBytes // a member's picture limit
	if r.ContentLength > limit {
		hopFail(w, http.StatusRequestEntityTooLarge, "too_large", "The file is larger than "+strconv.FormatInt(limit>>20, 10)+" MB.")
		return
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, limit+1))
	if err != nil {
		hopFail(w, http.StatusBadRequest, "bad_request", "The body could not be read.")
		return
	}
	if int64(len(body)) > limit {
		hopFail(w, http.StatusRequestEntityTooLarge, "too_large", "The file is larger than "+strconv.FormatInt(limit>>20, 10)+" MB.")
		return
	}
	if err := a.hopNonces.Verify(r, body, app, a.cfg.StoreClientSecrets(app), hopMaxAge); err != nil {
		hopFail(w, http.StatusUnauthorized, err.Error(), "The request signature was not accepted.")
		return
	}
	actor := trim(r.Header.Get("X-HOP-Actor"), 100)
	if actor == "" || actor == "system" {
		hopFail(w, http.StatusBadRequest, "no_actor", "X-HOP-Actor must be the member's HOP ID.")
		return
	}
	u, err := a.db.UserByHelink(r.Context(), actor)
	switch {
	case errors.Is(err, db.ErrNotFound):
		hopFail(w, http.StatusNotFound, "no_account", "That HOP ID has no account on this app yet. Sign in here once first.")
		return
	case err != nil:
		hopFail(w, http.StatusInternalServerError, "db_error", "Couldn't look up the account.")
		return
	case u.Status != "active":
		hopFail(w, http.StatusForbidden, "account_suspended", "This account can't store files right now.")
		return
	}
	if len(body) == 0 {
		hopFail(w, http.StatusBadRequest, "empty_body", "The body is empty.")
		return
	}
	if !a.limits.allow("hopmedia:u:"+strconv.FormatInt(u.ID, 10), a.cfg.UploadsPerMinute*4, time.Minute) {
		w.Header().Set("Retry-After", "60")
		hopFail(w, http.StatusTooManyRequests, "rate_limited", "Too many files in a minute. Wait a bit.")
		return
	}

	name := path.Base(strings.ReplaceAll(strings.TrimSpace(r.URL.Query().Get("name")), `\`, "/"))
	if name == "." || name == "/" || name == "" {
		name = "file"
	}
	name = trim(name, 200)

	ctx := context.WithValue(r.Context(), keyHop, &hopCaller{App: app, Actor: actor})
	done, used, fits := a.reserveQuota(ctx, u, int64(len(body)))
	if !fits {
		if ctx.Err() != nil {
			hopFail(w, http.StatusServiceUnavailable, "busy", "Try again in a moment.")
			return
		}
		hopFail(w, http.StatusRequestEntityTooLarge, "quota_exceeded", "The member's storage is full ("+humanBytes(used)+" of "+humanBytes(u.QuotaBytes)+").")
		return
	}
	im, _, dup, err := func() (*db.Image, string, bool, error) {
		defer done()
		return a.storeImage(ctx, uploadInput{name: name, data: body, size: int64(len(body))}, u, "", "", 0, true, false, "")
	}()
	var ue userError
	switch {
	case errors.Is(err, errBlockedFile):
		hopFail(w, http.StatusForbidden, "blocked_file", "This file is not allowed on this server.")
	case errors.As(err, &ue):
		hopFail(w, http.StatusUnsupportedMediaType, "unsupported_type", ue.msg)
	case err != nil:
		a.log.Error("hop media", "app", app, "err", err)
		hopFail(w, http.StatusInternalServerError, "store_failed", "The file couldn't be stored.")
	default:
		if !dup {
			a.audit(r.WithContext(ctx), "media.store", im.ID, map[string]any{"user": u.ID, "bytes": im.Size})
		}
		if !dup {
			w.WriteHeader(http.StatusCreated)
		}
		hopOK(w, a.hopMediaAnswer(im, dup))
	}
}

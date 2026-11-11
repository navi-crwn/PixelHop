package app

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"path"
	"strconv"
	"strings"
	"sync"
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

const (
	hopMediaUnverified = 4   // bodies held in memory before their signature is checked, at most one per address
	hopMediaPerIP      = 300 // calls a minute from one address, counted before the signature is known
	hopMediaPerApp     = 600 // calls a minute for a verified app
)

// hopMediaReadWindow is how long the body of a call may take to arrive once its headers did. The caller is a
// server on the same network or close to it, and 20 MB (the default picture limit) at 10 Mbit/s takes 16 s, so
// 20 s is enough for it and short enough that a sender without the secret cannot sit on a slot. It is a variable
// only so a test can shorten it.
var hopMediaReadWindow = 20 * time.Second

// hopSlots limits the unverified bodies in memory: at most max at once and one per address.
type hopSlots struct {
	mu   sync.Mutex
	held map[string]bool
}

func (s *hopSlots) take(ip string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.held == nil {
		s.held = map[string]bool{}
	}
	if s.held[ip] || len(s.held) >= hopMediaUnverified {
		return false
	}
	s.held[ip] = true
	return true
}

func (s *hopSlots) release(ip string) {
	s.mu.Lock()
	delete(s.held, ip)
	s.mu.Unlock()
}

// hopSignatureShape refuses, without reading the body, a call whose signature headers cannot be valid:
// a missing or malformed signature, a nonce of the wrong length or a timestamp outside the allowed age.
func hopSignatureShape(r *http.Request) string {
	sig := r.Header.Get("X-HOP-Signature")
	if len(sig) != 43 { // base64url of a SHA-256 HMAC, no padding
		return "bad_signature"
	}
	for _, c := range sig {
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '-' || c == '_') {
			return "bad_signature"
		}
	}
	if n := r.Header.Get("X-HOP-Nonce"); len(n) < 16 || len(n) > 128 {
		return "bad_signature"
	}
	t, err := strconv.ParseInt(r.Header.Get("X-HOP-Timestamp"), 10, 64)
	if err != nil {
		return "stale_request"
	}
	if d := time.Now().Unix() - t; d > int64(hopMaxAge.Seconds()) || -d > int64(hopMaxAge.Seconds()) {
		return "stale_request"
	}
	return ""
}

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
	// Before the body is read, and before the signature is known to be good, only what costs nothing:
	// the shape of the headers, a small limit per address, the size the caller announces, and a
	// fixed number of bodies in memory at once. Nothing here is counted against the app or the member,
	// so a caller without the secret cannot use up their limits.
	if code := hopSignatureShape(r); code != "" {
		hopFail(w, http.StatusUnauthorized, code, "The request signature was not accepted.")
		return
	}
	if !a.limits.allow("hopmedia:ip:"+clientIP(r), hopMediaPerIP, time.Minute) {
		w.Header().Set("Retry-After", "60")
		hopFail(w, http.StatusTooManyRequests, "rate_limited", "Too many calls from this address. Wait a minute.")
		return
	}
	limit := a.cfg.MaxUploadBytes // a member's picture limit
	if r.ContentLength > limit {
		hopFail(w, http.StatusRequestEntityTooLarge, "too_large", "The file is larger than "+strconv.FormatInt(limit>>20, 10)+" MB.")
		return
	}
	ip := clientIP(r)
	if !a.hopMediaSlots.take(ip) {
		w.Header().Set("Retry-After", "5")
		hopFail(w, http.StatusServiceUnavailable, "busy", "Try again in a moment.")
		return
	}
	// The only legitimate caller is another server, so the whole body must arrive within a short time:
	// a sender that stalls loses its slot instead of keeping it.
	http.NewResponseController(w).SetReadDeadline(time.Now().Add(hopMediaReadWindow))
	body, err := io.ReadAll(io.LimitReader(r.Body, limit+1))
	var verr error
	if err == nil && int64(len(body)) <= limit {
		verr = a.hopNonces.Verify(r, body, app, a.cfg.StoreClientSecrets(app), hopMaxAge)
	}
	a.hopMediaSlots.release(ip) // from here on the caller is known (or refused), and the limits below apply
	switch {
	case err != nil:
		var ne net.Error
		if errors.As(err, &ne) && ne.Timeout() {
			hopFail(w, http.StatusRequestTimeout, "body_timeout", "The file did not arrive in time.")
			return
		}
		hopFail(w, http.StatusBadRequest, "bad_request", "The body could not be read.")
		return
	case int64(len(body)) > limit:
		hopFail(w, http.StatusRequestEntityTooLarge, "too_large", "The file is larger than "+strconv.FormatInt(limit>>20, 10)+" MB.")
		return
	case verr != nil:
		hopFail(w, http.StatusUnauthorized, verr.Error(), "The request signature was not accepted.")
		return
	}
	if !a.limits.allow("hopmedia:app:"+app, hopMediaPerApp, time.Minute) {
		w.Header().Set("Retry-After", "60")
		hopFail(w, http.StatusTooManyRequests, "rate_limited", "Too many calls. Wait a minute.")
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

	// The same gate as a normal upload: read-only mode and free disk space, and one upload admitted at a time.
	if a.readOnly(r) {
		w.Header().Set("Retry-After", "300")
		hopFail(w, http.StatusServiceUnavailable, "read_only", "PixelHop is paused for maintenance.")
		return
	}
	a.uploadMu.Lock()
	defer a.uploadMu.Unlock()
	if !a.storageAvailable(int64(len(body))) {
		w.Header().Set("Retry-After", "60")
		hopFail(w, http.StatusInsufficientStorage, "storage_full", "PixelHop is short of disk space.")
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

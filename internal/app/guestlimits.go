package app

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"time"
)

// Guests (no account) are nudged toward an account: an album they make
// holds at most a few images, and one address may store only so many files
// a day. Both limits have a default from the environment and can be changed
// from the control center (settings guest_album_max and guest_daily_max);
// 0 turns a limit off. Members are never limited this way.

const guestDay = 24 * time.Hour

// rateError is a userError for a limit that lifts by itself: the API answers
// 429 with Retry-After.
type rateError struct {
	userError
	retryAfter time.Duration
}

func (e rateError) Unwrap() error { return e.userError }

// intSetting reads a whole-number runtime setting, falling back to def when
// the control center has not set one (or set something that is not a number).
func (a *App) intSetting(ctx context.Context, key string, def int) int {
	n, err := strconv.Atoi(a.db.Setting(ctx, key, strconv.Itoa(def)))
	if err != nil || n < 0 {
		return def
	}
	return n
}

func (a *App) guestAlbumMax(ctx context.Context) int {
	return a.intSetting(ctx, "guest_album_max", a.cfg.GuestAlbumMax)
}

func (a *App) guestDailyMax(ctx context.Context) int {
	return a.intSetting(ctx, "guest_daily_max", a.cfg.GuestDailyMax)
}

// guestKey is how a guest address is stored: hashed, never the address itself.
func guestKey(r *http.Request) string { return hashToken("guest:" + clientIP(r)) }

func (a *App) signUpURL() string { return a.cfg.BaseURL + "/register" }

func (a *App) guestAlbumFull(r *http.Request, max int) userError {
	return userError{a.tr(r, "Guest albums hold up to {n} images. Create a free account to remove this limit: {url}", "n", max, "url", a.signUpURL())}
}

// guestDailyRoom says how many more files the guest behind r may store in
// the next 24 hours (a negative number means no limit), and, when none, the
// error to answer with.
func (a *App) guestDailyRoom(r *http.Request) (int, error) {
	ctx := r.Context()
	max := a.guestDailyMax(ctx)
	if max <= 0 {
		return -1, nil
	}
	n, oldest := a.db.GuestUploads(ctx, guestKey(r), time.Now().Add(-guestDay).Unix())
	if n < max {
		return max - n, nil
	}
	retry := time.Until(time.Unix(oldest, 0).Add(guestDay))
	if retry < time.Minute {
		retry = time.Minute
	}
	return 0, rateError{
		userError{a.tr(r, "Guests can upload {n} files a day from one address. Create a free account to remove this limit: {url}", "n", max, "url", a.signUpURL())},
		retry,
	}
}

// countGuestUpload records a file a guest just stored.
func (a *App) countGuestUpload(r *http.Request) {
	if a.guestDailyMax(r.Context()) > 0 {
		a.db.AddGuestUpload(r.Context(), guestKey(r))
	}
}

// guestDayUsedUp answers a guest whose address stored its day's share of
// files (429, Retry-After) and reports whether it did.
func (a *App) guestDayUsedUp(w http.ResponseWriter, r *http.Request) bool {
	_, err := a.guestDailyRoom(r)
	if err == nil {
		return false
	}
	var re rateError
	if errors.As(err, &re) {
		retryAfter(w, re)
	}
	apiError(w, http.StatusTooManyRequests, err.Error())
	return true
}

// retryAfter sets Retry-After (whole seconds, at least 1) for a rateError.
func retryAfter(w http.ResponseWriter, e rateError) {
	secs := int(e.retryAfter.Seconds())
	if secs < 1 {
		secs = 1
	}
	w.Header().Set("Retry-After", strconv.Itoa(secs))
}

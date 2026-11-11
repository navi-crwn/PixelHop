package app

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/navi-crwn/pixelhop/internal/db"
)

// Claiming guest uploads: a browser that uploaded while signed out kept the
// delete link of each file (in localStorage). After signing in it may send the
// ids and tokens of those files in the body of POST /dashboard/claim; the files
// that still have no owner move into the member's library. A token only ever
// travels in that body: never in a URL, and never in a log line.

const (
	claimMaxItems = 200
	claimMaxBody  = 64 << 10
)

type claimRequest struct {
	Items []struct {
		ID    string `json:"id"`
		Token string `json:"token"`
	} `json:"items"`
}

func (a *App) claimUploads(w http.ResponseWriter, r *http.Request) {
	u := currentUser(r)
	if !a.limits.allow("claim:u:"+strconv.FormatInt(u.ID, 10), 10, time.Hour) || !a.limits.allow("claim:ip:"+clientIP(r), 30, time.Hour) {
		w.Header().Set("Retry-After", "3600")
		apiError(w, http.StatusTooManyRequests, a.tr(r, "You're doing this very fast. Please wait a while."))
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, claimMaxBody)
	var req claimRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || len(req.Items) == 0 || len(req.Items) > claimMaxItems {
		apiError(w, http.StatusBadRequest, "bad_request")
		return
	}
	seen := map[string]bool{}
	items := make([]db.ClaimItem, 0, len(req.Items))
	for _, it := range req.Items {
		if !validID(it.ID) || it.Token == "" || len(it.Token) > 128 {
			apiError(w, http.StatusBadRequest, "bad_request")
			return
		}
		if !seen[it.ID] {
			seen[it.ID] = true
			items = append(items, db.ClaimItem{ID: it.ID, TokenHash: hashToken(it.Token)})
		}
	}

	// Serialise with uploads of the same member so the quota check holds.
	ctx := r.Context()
	if !a.quota.lock(ctx, u.ID) {
		apiError(w, http.StatusServiceUnavailable, "busy")
		return
	}
	defer a.quota.unlock(u.ID)
	used, _ := a.db.UserStorage(ctx, u.ID)
	room := max(u.QuotaBytes-used, 0)
	ttl := int64(a.guestTTLDays(ctx)) * int64(guestDay/time.Second)
	res, err := a.db.ClaimImages(ctx, u.ID, items, room, ttl)
	if err != nil {
		a.log.Error("claim", "user", u.ID, "err", err)
		apiError(w, http.StatusInternalServerError, "couldn't claim")
		return
	}
	skipped := make([]map[string]string, 0, len(res.Skipped))
	for _, s := range res.Skipped {
		skipped = append(skipped, map[string]string{"id": s.ID, "reason": s.Reason})
	}
	a.log.Info("claim", "user", u.ID, "claimed", len(res.Claimed), "already", len(res.Already), "skipped", len(res.Skipped))
	apiOK(w, map[string]any{"claimed": nz(res.Claimed), "already": nz(res.Already), "skipped": skipped})
}

func nz(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

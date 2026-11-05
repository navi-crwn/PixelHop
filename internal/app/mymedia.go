package app

import (
	"errors"
	"io"
	"net/http"

	"github.com/navi-crwn/pixelhop/internal/db"
)

// GET /api/v1/me/media: the caller's own files that another page may show,
// for the media picker in BioHop (and any member tool). Besides the normal
// session or API key, a HOP app named in PIXELHOP_MEDIA_CLIENTS may call it
// server to server with a HOP App Protocol signature and X-HOP-Actor set to
// the member's HOP ID; the files stay here, the other app keeps the address.

// apiMyMedia lists pictures (type=image, the default) or PDFs (type=pdf).
// Private, password-protected, expiring, hidden and content-warning files
// are left out: a bio page could not show them anyway.
func (a *App) apiMyMedia(w http.ResponseWriter, r *http.Request) {
	u := a.mediaCaller(w, r)
	if u == nil {
		return
	}
	kind := r.URL.Query().Get("type")
	if kind == "" {
		kind = "image"
	}
	if kind != "image" && kind != "pdf" {
		apiError(w, http.StatusBadRequest, "bad_type")
		return
	}
	limit, offset := hopCursor(r, 40, 100)
	ims, total, err := a.db.Images(r.Context(), db.ImageQuery{UserID: u.ID, Kind: kind, Embeddable: true, Limit: limit, Offset: offset})
	if err != nil {
		apiError(w, http.StatusInternalServerError, "couldn't list files")
		return
	}
	albums := map[string]string{}
	items := make([]map[string]any, 0, len(ims))
	for _, im := range ims {
		album := ""
		if im.AlbumID.Valid {
			title, seen := albums[im.AlbumID.String]
			if !seen {
				if al, err := a.db.Album(r.Context(), im.AlbumID.String); err == nil {
					title = al.Title
				}
				albums[im.AlbumID.String] = title
			}
			album = title
		}
		items = append(items, map[string]any{
			"id": im.ID, "url": a.directURL(im), "thumb_url": a.thumbURL(im), "page_url": a.pageURL(im.ID),
			"width": im.Width, "height": im.Height, "bytes": im.Size, "mime": im.Mime, "kind": im.Kind(),
			"title": im.Title, "alt": im.Alt, "created_at": isoTime(im.CreatedAt), "album": album,
		})
	}
	apiOK(w, map[string]any{
		"items": items, "next": nextCursor(offset, len(ims), total), "total": total,
		// The owner blocks embedding on other websites: the files would not load on a bio page.
		"hotlinks_blocked": a.db.BlockHotlinks(r.Context(), u.ID),
	})
}

// mediaCaller is the member whose files are asked for: the signed-in user
// (session or API key), or the HOP ID a media client is acting for. When it
// returns nil an error has been written. Error codes for the calling app:
// not_configured, bad_signature, stale_request, replay, no_actor, no_account,
// account_suspended.
func (a *App) mediaCaller(w http.ResponseWriter, r *http.Request) *db.User {
	app := r.Header.Get("X-HOP-App")
	if app == "" && r.Header.Get("X-HOP-Signature") == "" {
		return a.apiUser(w, r)
	}
	if !a.cfg.SSOEnabled() {
		apiError(w, http.StatusNotFound, "not_configured")
		return nil
	}
	if !a.cfg.MediaClient(app) {
		apiError(w, http.StatusUnauthorized, "bad_signature")
		return nil
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, hopMaxBody+1))
	if err != nil || len(body) > hopMaxBody {
		apiError(w, http.StatusRequestEntityTooLarge, "too_large")
		return nil
	}
	if err := a.hopNonces.Verify(r, body, app, a.cfg.MediaClientSecrets(app), hopMaxAge); err != nil {
		apiError(w, http.StatusUnauthorized, err.Error())
		return nil
	}
	actor := trim(r.Header.Get("X-HOP-Actor"), 100)
	if actor == "" || actor == "system" {
		apiError(w, http.StatusBadRequest, "no_actor")
		return nil
	}
	u, err := a.db.UserByHelink(r.Context(), actor)
	switch {
	case errors.Is(err, db.ErrNotFound):
		apiError(w, http.StatusNotFound, "no_account")
		return nil
	case err != nil:
		apiError(w, http.StatusInternalServerError, "couldn't look up the account")
		return nil
	case u.Status != "active":
		apiError(w, http.StatusForbidden, "account_suspended")
		return nil
	}
	return u
}

package app

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/navi-crwn/pixelhop/internal/db"
)

// Albums can be edited after upload: title, description, cover, order,
// adding and removing images, and new albums made from a selection.

func (a *App) ownAlbum(w http.ResponseWriter, r *http.Request) (*db.User, *db.Album) {
	u := a.apiUser(w, r)
	if u == nil {
		return nil, nil
	}
	al, err := a.db.Album(r.Context(), r.PathValue("id"))
	if err != nil || !(al.UserID.Valid && al.UserID.Int64 == u.ID || u.IsAdmin()) {
		apiError(w, http.StatusNotFound, "album not found")
		return nil, nil
	}
	return u, al
}

func (a *App) albumJSON(r *http.Request, al *db.Album) map[string]any {
	ims, _, _ := a.db.Images(r.Context(), db.ImageQuery{AlbumID: al.ID, Sort: "album", Limit: 200})
	list := make([]map[string]any, 0, len(ims))
	for _, im := range ims {
		list = append(list, a.imageJSON(im))
	}
	return map[string]any{"id": al.ID, "title": al.Title, "description": al.Description, "cover_id": al.CoverID,
		"url": a.cfg.BaseURL + "/a/" + al.ID, "created_at": al.CreatedAt, "images": list}
}

func (a *App) apiAlbums(w http.ResponseWriter, r *http.Request) {
	u := a.apiUser(w, r)
	if u == nil {
		return
	}
	rows, err := a.db.Albums(r.Context(), u.ID)
	if err != nil {
		apiError(w, http.StatusInternalServerError, "couldn't list albums")
		return
	}
	out := make([]map[string]any, 0, len(rows))
	for _, al := range rows {
		out = append(out, map[string]any{"id": al.ID, "title": al.Title, "count": al.Count, "cover_id": al.CoverID,
			"url": a.cfg.BaseURL + "/a/" + al.ID, "created_at": al.CreatedAt})
	}
	apiOK(w, map[string]any{"albums": out})
}

func (a *App) apiAlbum(w http.ResponseWriter, r *http.Request) {
	if _, al := a.ownAlbum(w, r); al != nil {
		apiOK(w, a.albumJSON(r, al))
	}
}

// apiCreateAlbum makes an album from your images: {"title", "ids": [...]}
func (a *App) apiCreateAlbum(w http.ResponseWriter, r *http.Request) {
	u := a.apiUser(w, r)
	if u == nil {
		return
	}
	var body struct {
		Title string   `json:"title"`
		IDs   []string `json:"ids"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10)).Decode(&body); err != nil {
		apiError(w, http.StatusBadRequest, `send JSON like {"title": "Trip", "ids": ["…"]}`)
		return
	}
	al := &db.Album{ID: newID(""), Title: trim(strings.TrimSpace(body.Title), 120), UserID: sql.NullInt64{Int64: u.ID, Valid: true}}
	if err := a.db.InsertAlbum(r.Context(), al); err != nil {
		apiError(w, http.StatusInternalServerError, "couldn't create the album")
		return
	}
	a.addToAlbum(r, u, al, body.IDs)
	apiOK(w, a.albumJSON(r, al))
}

func (a *App) addToAlbum(r *http.Request, u *db.User, al *db.Album, ids []string) int {
	existing, _, _ := a.db.Images(r.Context(), db.ImageQuery{AlbumID: al.ID, Limit: 200})
	pos, n := len(existing), 0
	for _, id := range ids {
		im, err := a.db.Image(r.Context(), id)
		if err != nil || !(im.UserID.Valid && im.UserID.Int64 == u.ID || u.IsAdmin()) {
			continue
		}
		im.AlbumID = sql.NullString{String: al.ID, Valid: true}
		im.Position = pos
		pos++
		if a.db.UpdateImageMeta(r.Context(), im) == nil {
			n++
		}
		if n >= 200 {
			break
		}
	}
	return n
}

// apiUpdateAlbum: {"title", "description", "cover_id", "order": [ids], "add": [ids], "remove": [ids]}
func (a *App) apiUpdateAlbum(w http.ResponseWriter, r *http.Request) {
	u, al := a.ownAlbum(w, r)
	if al == nil {
		return
	}
	var body struct {
		Title       *string  `json:"title"`
		Description *string  `json:"description"`
		CoverID     *string  `json:"cover_id"`
		Order       []string `json:"order"`
		Add         []string `json:"add"`
		Remove      []string `json:"remove"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10)).Decode(&body); err != nil {
		apiError(w, http.StatusBadRequest, "send JSON")
		return
	}
	if body.Title != nil {
		al.Title = trim(strings.TrimSpace(*body.Title), 120)
	}
	if body.Description != nil {
		al.Description = trim(strings.TrimSpace(*body.Description), 1000)
	}
	if body.CoverID != nil {
		al.CoverID = *body.CoverID
	}
	for _, id := range body.Remove {
		if im, err := a.db.Image(r.Context(), id); err == nil && im.AlbumID.String == al.ID {
			im.AlbumID = sql.NullString{}
			a.db.UpdateImageMeta(r.Context(), im)
			if al.CoverID == id {
				al.CoverID = ""
			}
		}
	}
	a.addToAlbum(r, u, al, body.Add)
	for i, id := range body.Order {
		if im, err := a.db.Image(r.Context(), id); err == nil && im.AlbumID.String == al.ID {
			im.Position = i
			a.db.UpdateImageMeta(r.Context(), im)
		}
	}
	if err := a.db.UpdateAlbum(r.Context(), al); err != nil {
		apiError(w, http.StatusInternalServerError, "couldn't save")
		return
	}
	apiOK(w, a.albumJSON(r, al))
}

// apiDeleteAlbum removes the album; its images stay.
func (a *App) apiDeleteAlbum(w http.ResponseWriter, r *http.Request) {
	_, al := a.ownAlbum(w, r)
	if al == nil {
		return
	}
	if err := a.db.DeleteAlbum(r.Context(), al.ID); err != nil {
		apiError(w, http.StatusInternalServerError, "couldn't delete")
		return
	}
	apiOK(w, map[string]any{"deleted": al.ID})
}

// ---- pages ----

func (a *App) myAlbums(w http.ResponseWriter, r *http.Request) {
	rows, _ := a.db.Albums(r.Context(), currentUser(r).ID)
	thumbs := map[string]string{}
	for _, al := range rows {
		cover := al.CoverID
		if cover == "" {
			cover = al.FirstID
		}
		if im, err := a.db.Image(r.Context(), cover); err == nil {
			thumbs[al.ID] = strings.TrimPrefix(a.thumbURL(im), a.cfg.BaseURL)
		}
	}
	a.render(w, r, "albums", "My albums", map[string]any{"Albums": rows, "Thumbs": thumbs})
}

func (a *App) editAlbum(w http.ResponseWriter, r *http.Request) {
	u := currentUser(r)
	al, err := a.db.Album(r.Context(), r.PathValue("id"))
	if err != nil || !(al.UserID.Valid && al.UserID.Int64 == u.ID || u.IsAdmin()) {
		a.notFound(w, r)
		return
	}
	in, _, _ := a.db.Images(r.Context(), db.ImageQuery{AlbumID: al.ID, Sort: "album", Limit: 200})
	mine, _, _ := a.db.Images(r.Context(), db.ImageQuery{UserID: u.ID, Limit: 200})
	var others []*db.Image
	for _, im := range mine {
		if im.AlbumID.String != al.ID {
			others = append(others, im)
		}
	}
	a.render(w, r, "album_edit", "Edit album", map[string]any{"Album": al, "Images": in, "Others": others})
}

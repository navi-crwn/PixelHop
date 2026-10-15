package app

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/navi-crwn/pixelhop/internal/db"
)

type dashData struct {
	Images  []*db.Image
	Total   int
	Page    int
	Pages   int
	Query   string
	Sort    string
	Used    int64
	Quota   int64
	Percent int
	Welcome bool
}

func (a *App) dashboard(w http.ResponseWriter, r *http.Request) {
	u := currentUser(r)
	q := r.URL.Query()
	pg, _ := strconv.Atoi(q.Get("page"))
	if pg < 1 {
		pg = 1
	}
	const per = 48
	ims, total, err := a.db.Images(r.Context(), db.ImageQuery{UserID: u.ID, Search: q.Get("q"), Sort: q.Get("sort"), Limit: per, Offset: (pg - 1) * per})
	if err != nil {
		a.serverError(w, r, err)
		return
	}
	used, _ := a.db.UserStorage(r.Context(), u.ID)
	d := dashData{Images: ims, Total: total, Page: pg, Pages: (total + per - 1) / per, Query: q.Get("q"), Sort: q.Get("sort"),
		Used: used, Quota: u.QuotaBytes, Welcome: q.Has("welcome")}
	if u.QuotaBytes > 0 {
		d.Percent = int(min(100, used*100/u.QuotaBytes))
	}
	a.render(w, r, "dashboard", "My images", d)
}

type settingsData struct {
	Keys    []db.APIKey
	NewKey  string
	Notice  string
	Error   string
	HasPass bool
	Google  bool
}

func (a *App) settingsData(r *http.Request) settingsData {
	u := currentUser(r)
	keys, _ := a.db.APIKeys(r.Context(), u.ID)
	return settingsData{Keys: keys, HasPass: u.PassHash != "", Google: u.GoogleID.Valid, Notice: r.URL.Query().Get("ok")}
}

func (a *App) settingsPage(w http.ResponseWriter, r *http.Request) {
	a.render(w, r, "settings", "Settings", a.settingsData(r))
}

func (a *App) saveProfile(w http.ResponseWriter, r *http.Request) {
	u := currentUser(r)
	u.Name = trim(strings.TrimSpace(r.FormValue("name")), 60)
	if err := a.db.UpdateUser(r.Context(), u); err != nil {
		a.serverError(w, r, err)
		return
	}
	http.Redirect(w, r, "/settings?ok=Profile+saved", http.StatusSeeOther)
}

func (a *App) savePassword(w http.ResponseWriter, r *http.Request) {
	u := currentUser(r)
	d := a.settingsData(r)
	cur, pw := r.FormValue("current"), r.FormValue("password")
	switch {
	case u.PassHash != "" && !CheckPassword(u.PassHash, cur):
		d.Error = "Your current password isn't right."
	case len(pw) < 8 || len(pw) > 200:
		d.Error = "Use 8 to 200 characters for your new password."
	}
	if d.Error != "" {
		w.WriteHeader(http.StatusBadRequest)
		a.render(w, r, "settings", "Settings", d)
		return
	}
	u.PassHash = HashPassword(pw)
	a.db.UpdateUser(r.Context(), u)
	// Sign out everywhere else, then start a fresh session here.
	a.db.DeleteUserSessions(r.Context(), u.ID)
	a.startSession(w, r, u)
	http.Redirect(w, r, "/settings?ok=Password+changed", http.StatusSeeOther)
}

func (a *App) createKey(w http.ResponseWriter, r *http.Request) {
	u := currentUser(r)
	name := trim(strings.TrimSpace(r.FormValue("name")), 40)
	if name == "" {
		name = "My key"
	}
	if keys, _ := a.db.APIKeys(r.Context(), u.ID); len(keys) >= 20 {
		d := a.settingsData(r)
		d.Error = "You have 20 keys already; delete one first."
		a.render(w, r, "settings", "Settings", d)
		return
	}
	key := "ph_" + randomToken(24)
	if err := a.db.InsertAPIKey(r.Context(), u.ID, name, key[:9], hashToken(key)); err != nil {
		a.serverError(w, r, err)
		return
	}
	d := a.settingsData(r)
	d.NewKey = key // shown once; only its hash is stored
	a.render(w, r, "settings", "Settings", d)
}

func (a *App) deleteKey(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(r.PathValue("id"), 10, 64)
	a.db.DeleteAPIKey(r.Context(), currentUser(r).ID, id)
	http.Redirect(w, r, "/settings?ok=Key+deleted#api", http.StatusSeeOther)
}

// sharexConfig downloads a blank .sxcu file. Keys are only stored as hashes,
// so the settings page builds a filled-in file in the browser right after a
// key is made; this one has a placeholder to paste a key into.
func (a *App) sharexConfig(w http.ResponseWriter, r *http.Request) {
	cfg := map[string]any{
		"Version": "15.0.0", "Name": a.cfg.SiteName, "DestinationType": "ImageUploader",
		"RequestMethod": "POST", "RequestURL": a.cfg.BaseURL + "/api/v1/upload",
		"Headers": map[string]string{"Authorization": "Bearer PASTE_YOUR_API_KEY_HERE"}, "Body": "MultipartFormData",
		"FileFormName": "file", "URL": "{json:data.url}", "ThumbnailURL": "{json:data.thumb_url}",
		"DeletionURL": "{json:data.delete_url}", "ErrorMessage": "{json:error}",
	}
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", `attachment; filename="`+strings.ToLower(a.cfg.SiteName)+`.sxcu"`)
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	enc.Encode(cfg)
}

func (a *App) deleteAccount(w http.ResponseWriter, r *http.Request) {
	u := currentUser(r)
	if u.PassHash != "" && !CheckPassword(u.PassHash, r.FormValue("password")) {
		d := a.settingsData(r)
		d.Error = "Type your password to confirm deleting your account."
		w.WriteHeader(http.StatusBadRequest)
		a.render(w, r, "settings", "Settings", d)
		return
	}
	if u.PassHash == "" && r.FormValue("confirm") != u.Email {
		d := a.settingsData(r)
		d.Error = "Type your email address to confirm deleting your account."
		w.WriteHeader(http.StatusBadRequest)
		a.render(w, r, "settings", "Settings", d)
		return
	}
	a.purgeUser(r, u)
	a.logout(w, r)
}

func (a *App) ackWarning(w http.ResponseWriter, r *http.Request) {
	u := currentUser(r)
	u.Warning = ""
	a.db.UpdateUser(r.Context(), u)
	http.Redirect(w, r, safeNext(r.FormValue("back")), http.StatusSeeOther)
}

// purgeUser deletes an account and every image it uploaded.
func (a *App) purgeUser(r *http.Request, u *db.User) {
	ctx := r.Context()
	for {
		ims, _, err := a.db.Images(ctx, db.ImageQuery{UserID: u.ID, Limit: 200})
		if err != nil || len(ims) == 0 {
			break
		}
		for _, im := range ims {
			a.removeImage(ctx, im)
		}
	}
	a.db.DeleteUser(ctx, u.ID)
	a.db.DeleteEmptyAlbums(ctx)
}

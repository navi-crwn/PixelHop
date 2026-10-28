package app

import (
	"archive/zip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

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
	Kind    string
	Sum     db.UserSummary
	Keys    int // API keys made, to nudge people towards ShareX
}

func (a *App) dashboard(w http.ResponseWriter, r *http.Request) {
	u := currentUser(r)
	q := r.URL.Query()
	pg, _ := strconv.Atoi(q.Get("page"))
	if pg < 1 {
		pg = 1
	}
	const per = 48
	kind := q.Get("kind")
	if kind != "image" && kind != "video" && kind != "pdf" {
		kind = ""
	}
	ims, total, err := a.db.Images(r.Context(), db.ImageQuery{UserID: u.ID, Search: q.Get("q"), Sort: q.Get("sort"), Kind: kind, Limit: per, Offset: (pg - 1) * per})
	if err != nil {
		a.serverError(w, r, err)
		return
	}
	used, _ := a.db.UserStorage(r.Context(), u.ID)
	d := dashData{Images: ims, Total: total, Page: pg, Pages: (total + per - 1) / per, Query: q.Get("q"), Sort: q.Get("sort"),
		Used: used, Quota: u.QuotaBytes, Welcome: q.Has("welcome"), Kind: kind, Sum: a.db.UserSummary(r.Context(), u.ID)}
	if keys, _ := a.db.APIKeys(r.Context(), u.ID); keys != nil {
		d.Keys = len(keys)
	}
	if u.QuotaBytes > 0 {
		d.Percent = int(min(100, used*100/u.QuotaBytes))
	}
	a.render(w, r, "dashboard", "Dashboard", d)
}

// settingsNotices are the confirmations Settings shows after a change. The
// link carries a short code, not the text, so nobody can put words on the page.
var settingsNotices = map[string]string{
	"profile":         "Profile saved.",
	"photo":           "Profile photo updated.",
	"photo-removed":   "Profile photo removed.",
	"password":        "Password changed. Other devices were signed out.",
	"key-deleted":     "API key deleted.",
	"webhook-deleted": "Webhook deleted.",
	"webhook-test":    "Test sent; refresh in a few seconds to see the result.",
}

type settingsData struct {
	Webhooks         []db.Webhook
	NewWebhookSecret string
	Keys             []db.APIKey
	NewKey           string
	Notice           string
	Error            string
	HasPass          bool
	Google           bool
}

func (a *App) settingsData(r *http.Request) settingsData {
	u := currentUser(r)
	keys, _ := a.db.APIKeys(r.Context(), u.ID)
	hooks, _ := a.db.Webhooks(r.Context(), u.ID)
	return settingsData{Keys: keys, Webhooks: hooks, HasPass: u.PassHash != "", Google: u.GoogleID.Valid, Notice: settingsNotices[r.URL.Query().Get("ok")]}
}

func (a *App) settingsPage(w http.ResponseWriter, r *http.Request) {
	a.render(w, r, "settings", "Settings", a.settingsData(r))
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
	http.Redirect(w, r, "/settings?ok=password", http.StatusSeeOther)
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
	http.Redirect(w, r, "/settings?ok=key-deleted#api", http.StatusSeeOther)
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
	a.purgeUser(r.Context(), u)
	a.logout(w, r)
}

func (a *App) ackWarning(w http.ResponseWriter, r *http.Request) {
	u := currentUser(r)
	u.Warning = ""
	a.db.UpdateUser(r.Context(), u)
	http.Redirect(w, r, safeNext(r.FormValue("back")), http.StatusSeeOther)
}

// purgeUser deletes an account and every image it uploaded. Albums, API
// keys, webhooks and sessions go with the account (foreign keys cascade).
func (a *App) purgeUser(ctx context.Context, u *db.User) (images int) {
	for {
		ims, _, err := a.db.Images(ctx, db.ImageQuery{UserID: u.ID, Limit: 200})
		if err != nil || len(ims) == 0 {
			break
		}
		for _, im := range ims {
			if a.removeImage(ctx, im) == nil {
				images++
			}
		}
	}
	a.db.DeleteUser(ctx, u.ID)
	a.db.DeleteEmptyAlbums(ctx)
	return images
}

// downloadZip streams the chosen images (your own) as one ZIP file.
func (a *App) downloadZip(w http.ResponseWriter, r *http.Request) {
	u := currentUser(r)
	r.ParseForm()
	ids := r.Form["id"]
	if len(ids) == 0 || len(ids) > 500 {
		a.fail(w, r, http.StatusBadRequest, "Choose between 1 and 500 images.")
		return
	}
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", `attachment; filename="pixelhop-images.zip"`)
	zw := zip.NewWriter(w)
	defer zw.Close()
	used := map[string]bool{}
	for _, id := range ids {
		im, err := a.db.Image(r.Context(), id)
		if err != nil || (im.UserID.Int64 != u.ID && !u.IsAdmin()) {
			continue
		}
		b := a.backends[im.Backend]
		if b == nil {
			continue
		}
		obj, err := b.Get(r.Context(), im.KeyFull, "")
		if err != nil {
			continue
		}
		name := slugify(im.Title)
		if name == "" {
			name = im.ID
		}
		for n, base := 2, name; used[name+"."+im.Ext]; n++ {
			name = fmt.Sprintf("%s-%d", base, n)
		}
		used[name+"."+im.Ext] = true
		// Images are already compressed; storing them keeps this fast.
		f, err := zw.CreateHeader(&zip.FileHeader{Name: name + "." + im.Ext, Method: zip.Store, Modified: time.Unix(im.CreatedAt, 0)})
		if err == nil {
			io.Copy(f, obj.Body)
		}
		obj.Body.Close()
	}
}

package app

import (
	"archive/zip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/navi-crwn/pixelhop/internal/db"
	"github.com/navi-crwn/pixelhop/internal/storage"
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
	Tag     string
	Tags    []db.TagCount
	// Reminders are the reminder cards due now, by key (reminders.go).
	Reminders map[string]bool
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
	ims, total, err := a.db.Images(r.Context(), db.ImageQuery{UserID: u.ID, Search: q.Get("q"), Tag: q.Get("tag"), Sort: q.Get("sort"), Kind: kind, Limit: per, Offset: (pg - 1) * per})
	if err != nil {
		a.serverError(w, r, err)
		return
	}
	used, _ := a.db.UserStorage(r.Context(), u.ID)
	d := dashData{Images: ims, Total: total, Page: pg, Pages: (total + per - 1) / per, Query: q.Get("q"), Sort: q.Get("sort"),
		Used: used, Quota: u.QuotaBytes, Welcome: q.Has("welcome"), Kind: kind, Sum: a.db.UserSummary(r.Context(), u.ID), Tag: q.Get("tag"), Tags: a.db.UserTags(r.Context(), u.ID)}
	if keys, _ := a.db.APIKeys(r.Context(), u.ID); keys != nil {
		d.Keys = len(keys)
	}
	if u.QuotaBytes > 0 {
		d.Percent = int(min(100, used*100/u.QuotaBytes))
	}
	// The four getting-started steps, as the dashboard ticks them off.
	startOpen := d.Sum.Images+d.Sum.Videos+d.Sum.PDFs == 0 || d.Sum.Albums == 0 || !(u.ShowOwner && u.Username.Valid) || d.Keys == 0
	d.Reminders = map[string]bool{remindStart: a.reminderDue(r, u, remindStart, startOpen, time.Now())}
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
	"hotlinks":        "Embedding setting saved.",
	"sessions":        "Signed out on all your other devices.",
}

type settingsData struct {
	Webhooks         []db.Webhook
	NewWebhookSecret string
	Keys             []db.APIKey
	NewKey           string
	Notice           string
	Error            string
	HasPass          bool
	BlockHotlinks    bool
	Sessions         []sessionView
	Google           bool
}

func (a *App) settingsData(r *http.Request) settingsData {
	u := currentUser(r)
	keys, _ := a.db.APIKeys(r.Context(), u.ID)
	hooks, _ := a.db.Webhooks(r.Context(), u.ID)
	return settingsData{Keys: keys, Webhooks: hooks, HasPass: u.PassHash != "", BlockHotlinks: a.db.BlockHotlinks(r.Context(), u.ID), Sessions: a.sessionViews(r), Google: u.GoogleID.Valid, Notice: a.tr(r, settingsNotices[r.URL.Query().Get("ok")])}
}

func (a *App) settingsPage(w http.ResponseWriter, r *http.Request) {
	a.render(w, r, "settings", "Settings", a.settingsData(r))
}

func (a *App) savePassword(w http.ResponseWriter, r *http.Request) {
	u := currentUser(r)
	d := a.settingsData(r)
	cur, pw := r.FormValue("current"), r.FormValue("password")
	switch {
	case a.cfg.SSOEnabled() && u.HelinkID.Valid: // the same test as the template that hides the form: the password belongs to the hub
		d.Error = a.tr(r, "Your password is managed in HOP ID, not here.")
	case u.PassHash != "" && !CheckPassword(u.PassHash, cur):
		d.Error = a.tr(r, "Your current password isn't right.")
	case len(pw) < 8 || len(pw) > 200:
		d.Error = a.tr(r, "Use 8 to 200 characters for your new password.")
	}
	if d.Error != "" {
		a.renderStatus(w, r, http.StatusBadRequest, "settings", "Settings", d)
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
		d.Error = a.tr(r, "You have 20 keys already; delete one first.")
		a.render(w, r, "settings", "Settings", d)
		return
	}
	key := "ph_" + randomToken(24)
	if err := a.db.InsertAPIKeyScoped(r.Context(), u.ID, name, key[:9], hashToken(key), r.FormValue("scope")); err != nil {
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
		d.Error = a.tr(r, "Type your password to confirm deleting your account.")
		a.renderStatus(w, r, http.StatusBadRequest, "settings", "Settings", d)
		return
	}
	if u.PassHash == "" && r.FormValue("confirm") != u.Email {
		d := a.settingsData(r)
		d.Error = a.tr(r, "Type your email address to confirm deleting your account.")
		if u.HelinkID.Valid { // a HOP ID member loses their images and profile here; the account lives at the hub
			d.Error = a.tr(r, "Type your email address to confirm deleting your images and profile.")
		}
		a.renderStatus(w, r, http.StatusBadRequest, "settings", "Settings", d)
		return
	}
	if _, err := a.purgeUser(r.Context(), u); err != nil {
		a.serverError(w, r, err)
		return
	}
	a.logout(w, r)
}

func (a *App) ackWarning(w http.ResponseWriter, r *http.Request) {
	u := currentUser(r)
	u.Warning = ""
	a.db.UpdateUser(r.Context(), u)
	http.Redirect(w, r, safeNext(r.FormValue("back")), http.StatusSeeOther)
}

// purgeUser deletes an account and every image it uploaded, trash included, and
// its profile photo. Albums, API keys, webhooks and sessions go with the account
// (foreign keys cascade). The account row is the only place that knows the photo's
// storage key, so it goes last: when the images cannot be listed or the photo
// cannot be removed the account stays and an error comes back, and running the
// purge again finishes the job.
func (a *App) purgeUser(ctx context.Context, u *db.User) (images int, err error) {
	for _, trash := range []bool{false, true} { // the query lists one or the other, never both
		for {
			ims, _, err := a.db.Images(ctx, db.ImageQuery{UserID: u.ID, Trash: trash, Limit: 200})
			if err != nil {
				return images, fmt.Errorf("list images: %w", err)
			}
			if len(ims) == 0 {
				break
			}
			for _, im := range ims {
				if err := a.removeImage(ctx, im); err != nil {
					return images, fmt.Errorf("delete image %s: %w", im.ID, err)
				}
				images++
			}
		}
	}
	if u.AvatarKey != "" {
		if err := a.store.Delete(ctx, u.AvatarKey); err != nil && !errors.Is(err, storage.ErrNotFound) {
			return images, fmt.Errorf("delete profile photo: %w", err)
		}
	}
	if err := a.db.DeleteUser(ctx, u.ID); err != nil {
		return images, err
	}
	a.db.DeleteEmptyAlbums(ctx)
	return images, nil
}

// downloadZip streams the chosen images (your own) as one ZIP file.
func (a *App) downloadZip(w http.ResponseWriter, r *http.Request) {
	u := currentUser(r)
	r.ParseForm()
	ids := r.Form["id"]
	if len(ids) == 0 || len(ids) > 500 {
		a.fail(w, r, http.StatusBadRequest, a.tr(r, "Choose between 1 and 500 images."))
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

func (a *App) saveHotlinks(w http.ResponseWriter, r *http.Request) {
	u := currentUser(r)
	on := r.FormValue("block_hotlinks") != ""
	if err := a.db.SetBlockHotlinks(r.Context(), u.ID, on); err != nil {
		a.serverError(w, r, err)
		return
	}
	a.hotlinks.set(u.ID, on)
	http.Redirect(w, r, "/settings?ok=hotlinks#profile", http.StatusSeeOther)
}

type sessionView struct {
	Device  string
	IP      string
	Since   int64
	Current bool
}

// sessionViews lists the browsers this account is signed in on; the one in use is marked.
func (a *App) sessionViews(r *http.Request) []sessionView {
	u := currentUser(r)
	cur := ""
	if c, err := r.Cookie(sessionCookie); err == nil {
		cur = hashToken(c.Value)
	}
	var out []sessionView
	for _, s := range a.db.UserSessions(r.Context(), u.ID) {
		out = append(out, sessionView{Device: deviceName(s.UserAgent), IP: s.IP, Since: s.CreatedAt, Current: s.TokenHash == cur})
	}
	return out
}

// deviceName turns a User-Agent into something a person recognises: "Chrome on macOS".
func deviceName(ua string) string {
	browser := "Browser"
	for _, b := range [][2]string{{"Edg/", "Edge"}, {"OPR/", "Opera"}, {"Firefox/", "Firefox"}, {"Chrome/", "Chrome"}, {"Safari/", "Safari"}} {
		if strings.Contains(ua, b[0]) {
			browser = b[1]
			break
		}
	}
	system := ""
	for _, s := range [][2]string{{"iPhone", "iPhone"}, {"iPad", "iPad"}, {"Android", "Android"}, {"Windows", "Windows"}, {"Mac OS X", "macOS"}, {"Linux", "Linux"}} {
		if strings.Contains(ua, s[0]) {
			system = s[1]
			break
		}
	}
	if system == "" {
		return browser
	}
	return browser + " · " + system
}

// revokeSessions signs the account out everywhere except this browser.
func (a *App) revokeSessions(w http.ResponseWriter, r *http.Request) {
	u := currentUser(r)
	if c, err := r.Cookie(sessionCookie); err == nil {
		a.db.DeleteOtherSessions(r.Context(), u.ID, hashToken(c.Value))
	}
	http.Redirect(w, r, "/settings?ok=sessions#sessions", http.StatusSeeOther)
}

// exportData sends everything the account holds: the original files, and a
// JSON file with the profile, the details of each file and the albums.
func (a *App) exportData(w http.ResponseWriter, r *http.Request) {
	u := currentUser(r)
	ctx := r.Context()
	type fileInfo struct {
		ID        string `json:"id"`
		File      string `json:"file"`
		Title     string `json:"title,omitempty"`
		Alt       string `json:"alt,omitempty"`
		Mime      string `json:"mime"`
		Bytes     int64  `json:"bytes"`
		Width     int    `json:"width,omitempty"`
		Height    int    `json:"height,omitempty"`
		Views     int64  `json:"views"`
		Album     string `json:"album,omitempty"`
		Private   bool   `json:"private,omitempty"`
		Protected bool   `json:"password_protected,omitempty"`
		Page      string `json:"page"`
		Uploaded  string `json:"uploaded"`
	}
	var files []fileInfo
	albums, _ := a.db.Albums(ctx, u.ID)
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", `attachment; filename="pixelhop-export.zip"`)
	zw := zip.NewWriter(w)
	defer zw.Close()
	used := map[string]bool{}
	for off := 0; ; off += 200 {
		ims, _, err := a.db.Images(ctx, db.ImageQuery{UserID: u.ID, Limit: 200, Offset: off})
		if err != nil || len(ims) == 0 {
			break
		}
		for _, im := range ims {
			b := a.backends[im.Backend]
			if b == nil {
				continue
			}
			obj, err := b.Get(ctx, im.KeyFull, "")
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
			path := "files/" + name + "." + im.Ext
			if f, err := zw.CreateHeader(&zip.FileHeader{Name: path, Method: zip.Store, Modified: time.Unix(im.CreatedAt, 0)}); err == nil {
				io.Copy(f, obj.Body)
			}
			obj.Body.Close()
			files = append(files, fileInfo{ID: im.ID, File: path, Title: im.Title, Alt: im.Alt, Mime: im.Mime, Bytes: im.Size, Width: im.Width, Height: im.Height,
				Views: int64(im.Views), Album: im.AlbumID.String, Private: im.Private, Protected: im.PassHash != "", Page: a.pageURL(im.ID),
				Uploaded: time.Unix(im.CreatedAt, 0).UTC().Format(time.RFC3339)})
		}
	}
	type albumInfo struct {
		ID      string `json:"id"`
		Title   string `json:"title"`
		Images  int    `json:"images"`
		Page    string `json:"page"`
		Created string `json:"created"`
	}
	var al []albumInfo
	for _, x := range albums {
		al = append(al, albumInfo{ID: x.ID, Title: x.Title, Images: x.Count, Page: a.albumURL(x.ID), Created: time.Unix(x.CreatedAt, 0).UTC().Format(time.RFC3339)})
	}
	account := map[string]any{
		"exported": time.Now().UTC().Format(time.RFC3339), "site": a.cfg.SiteName,
		"account": map[string]any{"email": u.Email, "name": u.Name, "username": u.Username.String, "bio": u.Bio, "website": u.Website,
			"member_since": time.Unix(u.CreatedAt, 0).UTC().Format(time.RFC3339)},
		"files": files, "albums": al,
	}
	if f, err := zw.Create("account.json"); err == nil {
		enc := json.NewEncoder(f)
		enc.SetIndent("", "  ")
		enc.Encode(account)
	}
}

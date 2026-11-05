package app

import (
	"bytes"
	"context"
	"crypto/rand"
	"database/sql"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/url"
	"strings"
	"unicode"

	"github.com/navi-crwn/pixelhop/internal/db"
	"github.com/navi-crwn/pixelhop/internal/imaging"
	"github.com/navi-crwn/pixelhop/internal/netsafe"
)

// Public profiles: every account has a handle (/u/{username}), a display
// name, a photo and a short bio. Image pages show who shared them (unless the
// owner turns that off); the profile page shows only images the owner chose.

// reservedNames can't be handles: they'd be confusing next to site pages.
var reservedNames = map[string]bool{
	"admin": true, "api": true, "about": true, "help": true, "support": true, "settings": true, "login": true,
	"logout": true, "register": true, "dashboard": true, "tools": true, "docs": true, "legal": true, "static": true,
	"pixelhop": true, "helink": true, "hel": true, "root": true, "system": true, "moderator": true, "staff": true,
	"official": true, "null": true, "undefined": true, "me": true, "you": true, "guest": true, "anonymous": true,
}

// usernameProblem explains why name can't be used, or returns "".
func usernameProblem(name string) string {
	if len(name) < 3 || len(name) > 30 {
		return "Usernames are 3 to 30 characters."
	}
	for i, r := range name {
		ok := r < 128 && (unicode.IsLower(r) || unicode.IsDigit(r)) || (i > 0 && (r == '-' || r == '_' || r == '.'))
		if !ok {
			return "Use lowercase letters, numbers, dots, dashes or underscores, starting with a letter or number."
		}
	}
	if strings.HasSuffix(name, ".") || strings.Contains(name, "..") {
		return "A username can't end with a dot or have two in a row."
	}
	if reservedNames[name] {
		return "That username is reserved."
	}
	return ""
}

// suggestUsername makes a free handle from the display name. Accounts without
// a name get "user" plus digits: the email is never used, since the handle is
// public and would give part of the address away.
func (a *App) suggestUsername(ctx context.Context, u *db.User) string {
	base := strings.ReplaceAll(slugify(u.Name), "-", "")
	if len(base) < 3 {
		n, _ := rand.Int(rand.Reader, big.NewInt(90000))
		base = fmt.Sprintf("user%d", 10000+n.Int64())
	}
	if len(base) > 24 {
		base = base[:24]
	}
	name := base
	for n := 2; usernameProblem(name) != "" || a.db.UsernameTaken(ctx, name, u.ID); n++ {
		name = fmt.Sprintf("%s%d", base, n)
	}
	return name
}

// backfillUsernames gives every account without a handle one, at startup.
func (a *App) backfillUsernames(ctx context.Context) {
	users, err := a.db.UsersWithoutUsername(ctx)
	if err != nil {
		return
	}
	for _, u := range users {
		u.Username = sql.NullString{String: a.suggestUsername(ctx, u), Valid: true}
		a.db.UpdateUser(ctx, u)
	}
}

// displayName is what others see: the chosen name, else the handle.
func displayName(u *db.User) string {
	if strings.TrimSpace(u.Name) != "" {
		return u.Name
	}
	if u.Username.Valid {
		return u.Username.String
	}
	return "PixelHop user"
}

// avatarURL is the photo to show: an uploaded one, else the HEL.ink/Google
// picture, else "" (pages draw the first letter instead).
func avatarURL(u *db.User) string {
	if u == nil {
		return ""
	}
	if u.AvatarKey != "" {
		return fmt.Sprintf("/avatar/%d?v=%s", u.ID, hashToken(u.AvatarKey)[:10])
	}
	return u.AvatarURL
}

// avatarOf is the photo to show for u, ready to put in an <img src>: the
// uploaded one (a path on this host) or the HEL.ink/Google picture made
// absolute. HOP ID sends the picture as a bare path or an http:// address
// when its storage URL is configured loosely; a bare path would be asked of
// this host (404) and http:// is blocked as mixed content, so both are fixed here.
func (a *App) avatarOf(u *db.User) string {
	if u == nil {
		return ""
	}
	if u.AvatarKey != "" {
		return avatarURL(u)
	}
	return a.externalAvatar(u.AvatarURL)
}

// externalAvatar turns a picture address received from HOP ID or Google into
// one the browser can load from any of our pages, or "" when there is none.
func (a *App) externalAvatar(raw string) string {
	raw = strings.TrimSpace(raw)
	switch {
	case raw == "":
		return ""
	case strings.HasPrefix(raw, "https://"):
		return raw
	case strings.HasPrefix(raw, "http://"):
		if a.cfg.SecureCookies() { // an https site cannot show http images
			return "https://" + strings.TrimPrefix(raw, "http://")
		}
		return raw
	case strings.HasPrefix(raw, "//"):
		return "https:" + raw
	case strings.HasPrefix(raw, "/"):
		if a.cfg.HelinkURL != "" { // a path on the account host
			return a.cfg.HelinkURL + raw
		}
	}
	return ""
}

// owner is the "shared by" card on image and album pages.
type owner struct {
	Name, Username, Avatar, Initial, ProfileURL string
}

func (a *App) ownerOf(ctx context.Context, userID sql.NullInt64) *owner {
	if !userID.Valid {
		return nil
	}
	u, err := a.db.UserByID(ctx, userID.Int64)
	if err != nil || !u.ShowOwner || u.Status != "active" {
		return nil
	}
	o := &owner{Name: displayName(u), Avatar: a.avatarOf(u)}
	o.Initial = strings.ToUpper(string([]rune(o.Name)[:1]))
	if u.Username.Valid {
		o.Username = u.Username.String
		o.ProfileURL = "/u/" + u.Username.String
	}
	return o
}

// ---- profile page ----

type profileData struct {
	Owner   *owner
	User    *db.User
	Images  []*db.Image
	Total   int
	Website string
	Mine    bool
}

func (a *App) profile(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("username")
	u, err := a.db.UserByUsername(r.Context(), name)
	if err != nil || !u.ShowOwner || u.Status != "active" {
		a.notFound(w, r)
		return
	}
	if u.Username.String != name { // one canonical address per profile
		http.Redirect(w, r, "/u/"+u.Username.String, http.StatusMovedPermanently)
		return
	}
	ims, total, _ := a.db.Images(r.Context(), db.ImageQuery{UserID: u.ID, Listed: true, Limit: 120})
	me := currentUser(r)
	d := profileData{Owner: a.ownerOf(r.Context(), sql.NullInt64{Int64: u.ID, Valid: true}), User: u, Images: ims, Total: total,
		Mine: me != nil && me.ID == u.ID}
	if site, err := url.Parse(shownWebsite(u.Website)); err == nil && site.Host != "" {
		d.Website = strings.TrimPrefix(strings.TrimPrefix(u.Website, "https://"), "http://")
	}
	desc := u.Bio
	if desc == "" {
		desc = fmt.Sprintf("%d images shared by %s on %s", total, displayName(u), a.cfg.SiteName)
	}
	og := ""
	if len(ims) > 0 && !a.mediaHasWarning(r, ims[0]) {
		og = a.previewURL(ims[0]) // a video's or PDF's poster, never the file itself
	}
	a.renderPage(w, r, "profile", page{Title: displayName(u) + " (@" + u.Username.String + ")", Desc: desc, OGImage: og, Data: d})
}

// ---- avatar ----

func (a *App) serveAvatar(w http.ResponseWriter, r *http.Request) {
	var id int64
	fmt.Sscan(r.PathValue("id"), &id)
	u, err := a.db.UserByID(r.Context(), id)
	// Someone who hides their profile still sees their own photo in their own
	// pages (top bar, settings); everybody else gets a 404.
	me := currentUser(r)
	self := me != nil && (me.ID == id || me.IsAdmin())
	if err != nil || (!u.ShowOwner && !self) {
		a.notFound(w, r)
		return
	}
	if u.AvatarKey == "" {
		if pic := a.externalAvatar(u.AvatarURL); pic != "" {
			http.Redirect(w, r, pic, http.StatusFound)
			return
		}
		a.notFound(w, r)
		return
	}
	obj, err := a.store.Get(r.Context(), u.AvatarKey, "")
	if err != nil {
		a.notFound(w, r)
		return
	}
	defer obj.Body.Close()
	w.Header().Set("Content-Type", "image/jpeg")
	if u.ShowOwner {
		w.Header().Set("Cache-Control", "public, max-age=604800, immutable") // the URL changes with the photo
	} else {
		w.Header().Set("Cache-Control", "private, max-age=3600")
	}
	// Public photos are shown on other sites too (the gallery API hands out this address).
	w.Header().Set("Cross-Origin-Resource-Policy", "cross-origin")
	w.Header().Set("Content-Security-Policy", "default-src 'none'; sandbox")
	io.Copy(w, obj.Body)
}

func (a *App) saveAvatar(w http.ResponseWriter, r *http.Request) {
	u := currentUser(r)
	r.Body = http.MaxBytesReader(w, r.Body, 10<<20)
	data, err := formBytes(r, "avatar", 8<<20)
	if err == nil {
		data, err = imaging.Avatar(data, 40_000_000, 400)
	}
	if err != nil {
		a.settingsError(w, r, a.tr(r, "Profile photo: {err}", "err", err.Error()))
		return
	}
	key := fmt.Sprintf("avatars/%d-%s.jpg", u.ID, randomToken(6))
	if err := a.store.Put(r.Context(), key, bytes.NewReader(data), int64(len(data)), "image/jpeg"); err != nil {
		a.serverError(w, r, err)
		return
	}
	old := u.AvatarKey
	u.AvatarKey = key
	if err := a.db.UpdateUser(r.Context(), u); err != nil {
		a.serverError(w, r, err)
		return
	}
	if old != "" {
		a.store.Delete(r.Context(), old)
	}
	http.Redirect(w, r, "/settings?ok=photo#profile", http.StatusSeeOther)
}

func (a *App) deleteAvatar(w http.ResponseWriter, r *http.Request) {
	u := currentUser(r)
	if u.AvatarKey != "" {
		a.store.Delete(r.Context(), u.AvatarKey)
		u.AvatarKey = ""
		a.db.UpdateUser(r.Context(), u)
	}
	http.Redirect(w, r, "/settings?ok=photo-removed#profile", http.StatusSeeOther)
}

// holdsNoteKey reports whether a profile address carries the key of an encrypted
// note, as the address itself or inside another one, with the # escaped or not.
func holdsNoteKey(site string) bool {
	return netsafe.NoteKeyLink(site) || netsafe.NoteKeyInText(site)
}

// shownWebsite is the stored address unless it holds a note key: rows from before
// the key was refused stay until their owner changes them, and are not handed out.
func shownWebsite(site string) string {
	if holdsNoteKey(site) {
		return ""
	}
	return site
}

// saveProfile stores the public profile: name, handle, bio, website, visibility.
func (a *App) saveProfile(w http.ResponseWriter, r *http.Request) {
	u := currentUser(r)
	name := trim(strings.TrimSpace(netsafe.ScrubNoteKeys(r.FormValue("name"))), 60)
	handle := strings.ToLower(strings.TrimSpace(strings.TrimPrefix(r.FormValue("username"), "@")))
	bio := trim(strings.TrimSpace(netsafe.ScrubNoteKeys(r.FormValue("bio"))), 300)
	site := strings.TrimSpace(r.FormValue("website"))
	if site != "" && !strings.Contains(site, "://") {
		site = "https://" + site
	}
	if p := usernameProblem(handle); p != "" {
		p = a.tr(r, p)
		a.settingsError(w, r, p)
		return
	}
	if a.db.UsernameTaken(r.Context(), handle, u.ID) {
		a.settingsError(w, r, a.tr(r, "Someone already has the username @{name}.", "name", handle))
		return
	}
	if site != "" && holdsNoteKey(site) {
		// The key of an encrypted note is in the #fragment; a profile is public and its address is stored.
		a.settingsError(w, r, a.tr(r, "That address opens an encrypted note, and the part after the # is its key. Share the note link directly instead of putting it on a profile."))
		return
	}
	if site != "" {
		pu, err := url.Parse(site)
		if err != nil || (pu.Scheme != "https" && pu.Scheme != "http") || !strings.Contains(pu.Host, ".") || len(site) > 200 {
			a.settingsError(w, r, a.tr(r, "That website address doesn't look right."))
			return
		}
	}
	u.Name, u.Bio, u.Website = name, bio, site
	u.Username = sql.NullString{String: handle, Valid: true}
	u.ShowOwner = r.FormValue("show_owner") == "on"
	if err := a.db.UpdateUser(r.Context(), u); err != nil {
		if strings.Contains(err.Error(), "UNIQUE") {
			a.settingsError(w, r, a.tr(r, "Someone already has the username @{name}.", "name", handle))
			return
		}
		a.serverError(w, r, err)
		return
	}
	http.Redirect(w, r, "/settings?ok=profile#profile", http.StatusSeeOther)
}

func (a *App) settingsError(w http.ResponseWriter, r *http.Request, msg string) {
	d := a.settingsData(r)
	d.Error = msg
	w.WriteHeader(http.StatusBadRequest)
	a.render(w, r, "settings", "Settings", d)
}

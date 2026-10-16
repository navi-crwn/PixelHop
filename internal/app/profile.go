package app

import (
	"bytes"
	"context"
	"database/sql"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"unicode"

	"github.com/navi-crwn/pixelhop/internal/db"
	"github.com/navi-crwn/pixelhop/internal/imaging"
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

// suggestUsername makes a free handle from a name or email.
func (a *App) suggestUsername(ctx context.Context, u *db.User) string {
	base := slugify(u.Name)
	if base == "" {
		local, _, _ := strings.Cut(u.Email, "@")
		base = slugify(local)
	}
	base = strings.ReplaceAll(base, "-", "")
	if len(base) < 3 {
		base = "user" + base
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
	o := &owner{Name: displayName(u), Avatar: avatarURL(u)}
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
	if site, err := url.Parse(u.Website); err == nil && site.Host != "" {
		d.Website = strings.TrimPrefix(strings.TrimPrefix(u.Website, "https://"), "http://")
	}
	desc := u.Bio
	if desc == "" {
		desc = fmt.Sprintf("%d images shared by %s on %s", total, displayName(u), a.cfg.SiteName)
	}
	og := ""
	if len(ims) > 0 {
		og = a.directURL(ims[0])
	}
	a.renderPage(w, r, "profile", page{Title: displayName(u) + " (@" + u.Username.String + ")", Desc: desc, OGImage: og, Data: d})
}

// ---- avatar ----

func (a *App) serveAvatar(w http.ResponseWriter, r *http.Request) {
	var id int64
	fmt.Sscan(r.PathValue("id"), &id)
	u, err := a.db.UserByID(r.Context(), id)
	if err != nil || !u.ShowOwner {
		a.notFound(w, r)
		return
	}
	if u.AvatarKey == "" {
		if strings.HasPrefix(u.AvatarURL, "https://") {
			http.Redirect(w, r, u.AvatarURL, http.StatusFound)
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
	w.Header().Set("Cache-Control", "public, max-age=604800, immutable") // the URL changes with the photo
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
		a.settingsError(w, r, "Profile photo: "+err.Error())
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
	http.Redirect(w, r, "/settings?ok=Profile+photo+updated#profile", http.StatusSeeOther)
}

func (a *App) deleteAvatar(w http.ResponseWriter, r *http.Request) {
	u := currentUser(r)
	if u.AvatarKey != "" {
		a.store.Delete(r.Context(), u.AvatarKey)
		u.AvatarKey = ""
		a.db.UpdateUser(r.Context(), u)
	}
	http.Redirect(w, r, "/settings?ok=Profile+photo+removed#profile", http.StatusSeeOther)
}

// saveProfile stores the public profile: name, handle, bio, website, visibility.
func (a *App) saveProfile(w http.ResponseWriter, r *http.Request) {
	u := currentUser(r)
	name := trim(strings.TrimSpace(r.FormValue("name")), 60)
	handle := strings.ToLower(strings.TrimSpace(strings.TrimPrefix(r.FormValue("username"), "@")))
	bio := trim(strings.TrimSpace(r.FormValue("bio")), 300)
	site := strings.TrimSpace(r.FormValue("website"))
	if site != "" && !strings.Contains(site, "://") {
		site = "https://" + site
	}
	if p := usernameProblem(handle); p != "" {
		a.settingsError(w, r, p)
		return
	}
	if a.db.UsernameTaken(r.Context(), handle, u.ID) {
		a.settingsError(w, r, "Someone already has the username @"+handle+".")
		return
	}
	if site != "" {
		pu, err := url.Parse(site)
		if err != nil || (pu.Scheme != "https" && pu.Scheme != "http") || !strings.Contains(pu.Host, ".") || len(site) > 200 {
			a.settingsError(w, r, "That website address doesn't look right.")
			return
		}
	}
	u.Name, u.Bio, u.Website = name, bio, site
	u.Username = sql.NullString{String: handle, Valid: true}
	u.ShowOwner = r.FormValue("show_owner") == "on"
	if err := a.db.UpdateUser(r.Context(), u); err != nil {
		if strings.Contains(err.Error(), "UNIQUE") {
			a.settingsError(w, r, "Someone already has the username @"+handle+".")
			return
		}
		a.serverError(w, r, err)
		return
	}
	http.Redirect(w, r, "/settings?ok=Profile+saved#profile", http.StatusSeeOther)
}

func (a *App) settingsError(w http.ResponseWriter, r *http.Request, msg string) {
	d := a.settingsData(r)
	d.Error = msg
	w.WriteHeader(http.StatusBadRequest)
	a.render(w, r, "settings", "Settings", d)
}

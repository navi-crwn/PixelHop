package app

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/navi-crwn/pixelhop/internal/db"
)

// Visibility: by default an image is "unlisted" (anyone with the link). The
// owner can make it private (only them) or put a password on it; then the
// page asks for the password and the file itself is only served to people
// who unlocked it, so hotlinks stop working too.

func (a *App) unlockKey() []byte {
	return []byte(a.db.Secret(a.ctx(), "unlock", func() string { return randomToken(32) }))
}

func unlockCookieName(id string) string { return "ph_u_" + id }

// unlockToken is tied to the current password: changing it locks everyone out again.
func (a *App) unlockToken(im *db.Image) string {
	m := hmac.New(sha256.New, a.unlockKey())
	m.Write([]byte(im.ID + "|" + im.PassHash))
	return base64.RawURLEncoding.EncodeToString(m.Sum(nil))
}

// canSee reports whether this visitor may see a locked image.
func (a *App) canSee(r *http.Request, im *db.Image) bool {
	if !im.Locked() {
		return true
	}
	if u := currentUser(r); u != nil && (u.IsAdmin() || (im.UserID.Valid && im.UserID.Int64 == u.ID)) {
		return true
	}
	if im.Private {
		return false
	}
	c, err := r.Cookie(unlockCookieName(im.ID))
	return err == nil && hmac.Equal([]byte(c.Value), []byte(a.unlockToken(im)))
}

func (a *App) unlock(w http.ResponseWriter, r *http.Request) {
	im, err := a.db.Image(r.Context(), r.PathValue("id"))
	if err != nil || im.Hidden || im.Private || im.PassHash == "" {
		a.notFound(w, r)
		return
	}
	if !a.limits.allow("unlock:"+clientIP(r), 10, 10*time.Minute) {
		a.renderLocked(w, r, im, "Too many tries. Please wait a few minutes.")
		return
	}
	if !CheckPassword(im.PassHash, r.FormValue("password")) {
		a.renderLocked(w, r, im, "That password isn't right.")
		return
	}
	http.SetCookie(w, &http.Cookie{Name: unlockCookieName(im.ID), Value: a.unlockToken(im), Path: "/", HttpOnly: true,
		Secure: a.cfg.SecureCookies(), SameSite: http.SameSiteLaxMode, MaxAge: 30 * 86400})
	http.Redirect(w, r, "/"+im.ID, http.StatusSeeOther)
}

func (a *App) renderLocked(w http.ResponseWriter, r *http.Request, im *db.Image, msg string) {
	if msg != "" {
		w.WriteHeader(http.StatusUnauthorized)
	}
	a.renderPage(w, r, "locked", page{Title: "Password required", NoIndex: true, Data: map[string]any{"ID": im.ID, "Error": msg}})
}

// ---- editing an image's details ----

// apiUpdateImage changes title, alt text, profile listing and visibility.
// Body: {"title","alt","listed","visibility":"unlisted|private|password","password"}
func (a *App) apiUpdateImage(w http.ResponseWriter, r *http.Request) {
	im := a.ownImage(w, r)
	if im == nil {
		return
	}
	var body struct {
		Title      *string `json:"title"`
		Alt        *string `json:"alt"`
		Listed     *bool   `json:"listed"`
		Visibility *string `json:"visibility"`
		Password   string  `json:"password"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8192)).Decode(&body); err != nil {
		apiError(w, http.StatusBadRequest, `send JSON like {"title": "…"}`)
		return
	}
	if body.Title != nil {
		im.Title = trim(strings.TrimSpace(*body.Title), 120)
	}
	if body.Alt != nil {
		im.Alt = trim(strings.TrimSpace(*body.Alt), 300)
	}
	if body.Visibility != nil {
		switch *body.Visibility {
		case "unlisted":
			im.Private, im.PassHash = false, ""
		case "private":
			im.Private, im.PassHash = true, ""
		case "password":
			if body.Password == "" && im.PassHash == "" {
				apiError(w, http.StatusBadRequest, "choose a password")
				return
			}
			if body.Password != "" {
				if len(body.Password) < 4 || len(body.Password) > 200 {
					apiError(w, http.StatusBadRequest, "passwords are 4 to 200 characters")
					return
				}
				im.PassHash = HashPassword(body.Password)
			}
			im.Private = false
		default:
			apiError(w, http.StatusBadRequest, "visibility is unlisted, private or password")
			return
		}
	}
	if body.Listed != nil {
		im.Listed = *body.Listed
	}
	if im.Locked() {
		im.Listed = false // locked images never appear on profiles or Explore
	}
	if err := a.db.UpdateImageMeta(r.Context(), im); err != nil {
		apiError(w, http.StatusInternalServerError, "couldn't save")
		return
	}
	apiOK(w, a.imageJSON(im))
}

// ---- views and where they came from ----

// countView adds one view (once per visitor per hour) and remembers the
// referring site's host, so owners can see where their image is shared.
func (a *App) countView(r *http.Request, im *db.Image) {
	if !a.views.first(clientIP(r) + "|" + im.ID) {
		return
	}
	host := ""
	if ref, err := url.Parse(r.Referer()); err == nil && ref.Host != "" {
		host = strings.TrimPrefix(strings.ToLower(ref.Hostname()), "www.")
		if base, err := url.Parse(a.cfg.BaseURL); err == nil && strings.EqualFold(base.Hostname(), ref.Hostname()) {
			host = "" // our own pages don't count as referrers
		}
		host = trim(host, 100)
	}
	a.db.CountView(r.Context(), im.ID, host)
}

func (a *App) apiImageStats(w http.ResponseWriter, r *http.Request) {
	im := a.ownImage(w, r)
	if im == nil {
		return
	}
	days, hosts := a.db.ImageStats(r.Context(), im.ID)
	apiOK(w, map[string]any{"id": im.ID, "views": im.Views, "days": days, "referrers": hosts})
}

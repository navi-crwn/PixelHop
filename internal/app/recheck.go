package app

import (
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/navi-crwn/pixelhop/internal/db"
)

// recheckAfter is how long the hub's word on a session is trusted. After that, the next page the
// browser loads makes one quiet trip to HEL.ink (prompt=none, no page shown): the same account
// carries on, another account replaces the session, and no hub session ends this one. A var so
// tests can move it.
var recheckAfter = 5 * time.Minute

// recheckCooldown is the least time between two quiet trips from one browser. It is what stops a
// loop when the hub answers "try later" (the session stays) and what keeps a flood of page loads
// from becoming a flood of hub requests.
const recheckCooldown = time.Minute

const recheckCookie = "ph_recheck"

// pageNavigation says whether r is a browser loading a page (not an image, a script, a fetch or a
// prefetch): only then is a trip through the hub acceptable, since it replaces the page being loaded
// and so can never interrupt an upload or a chosen file on a page that is already open.
func pageNavigation(r *http.Request) bool {
	if r.Method != http.MethodGet || r.Header.Get("Sec-Purpose") != "" || r.Header.Get("Purpose") != "" {
		return false
	}
	switch dest := r.Header.Get("Sec-Fetch-Dest"); dest {
	case "document":
	case "":
		if !strings.Contains(r.Header.Get("Accept"), "text/html") {
			return false
		}
	default:
		return false
	}
	p := r.URL.Path
	for _, pre := range []string{"/auth/", "/api/", "/hop/", "/i/", "/t/", "/static/", "/avatar/", "/qr/", "/i18n/"} {
		if strings.HasPrefix(p, pre) {
			return false
		}
	}
	switch p {
	case "/oembed", "/healthz", "/metrics", "/sw.js", "/manifest.webmanifest", "/favicon.ico", "/robots.txt", "/sitemap.xml":
		return false
	}
	return true
}

// recheckDue reports whether this request should be answered with a quiet trip to the hub.
// It never is for an API key, for a session the hub has no say over (password, Google), when the
// hub does not answer, or when a trip was made a moment ago.
func (a *App) recheckDue(r *http.Request, token string, u *db.User) bool {
	if !a.cfg.SSOEnabled() || !u.HelinkID.Valid || !pageNavigation(r) {
		return false
	}
	at, err := a.db.SessionVerifiedAt(r.Context(), hashToken(token))
	if err != nil || at < 0 || time.Since(time.Unix(at, 0)) <= recheckAfter {
		return false
	}
	if _, err := r.Cookie(recheckCookie); err == nil {
		return false
	}
	return a.hubUp != nil && a.hubUp()
}

// recheck sends the browser to the hub with prompt=none, back to the page it asked for.
func (a *App) recheck(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{Name: recheckCookie, Value: "1", Path: "/", HttpOnly: true,
		Secure: a.cfg.SecureCookies(), SameSite: http.SameSiteLaxMode, MaxAge: int(recheckCooldown.Seconds())})
	w.Header().Set("Cache-Control", "no-store")
	http.Redirect(w, r, "/auth/helink?prompt=none&next="+url.QueryEscape(r.URL.RequestURI()), http.StatusSeeOther)
}

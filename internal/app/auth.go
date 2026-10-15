package app

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/mail"
	"net/url"
	"strings"
	"time"

	"golang.org/x/crypto/argon2"
	"golang.org/x/crypto/bcrypt"

	"github.com/navi-crwn/pixelhop/internal/config"
	"github.com/navi-crwn/pixelhop/internal/db"
)

const sessionCookie = "ph_session"

// ---- tokens ----

func randomToken(n int) string {
	b := make([]byte, n)
	rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}

func hashToken(t string) string {
	s := sha256.Sum256([]byte(t))
	return hex.EncodeToString(s[:])
}

// ---- passwords ----

// Argon2id with OWASP's recommended minimum (19 MiB, 2 passes) keeps logins
// fast on a 1 GB server. Hashes use PHP's format, so passwords made by
// PixelHop 2's password_hash(PASSWORD_ARGON2ID) still verify.
const (
	argonMem  = 19 * 1024
	argonTime = 2
	argonPar  = 1
)

func HashPassword(pw string) string {
	salt := make([]byte, 16)
	rand.Read(salt)
	key := argon2.IDKey([]byte(pw), salt, argonTime, argonMem, argonPar, 32)
	return fmt.Sprintf("$argon2id$v=19$m=%d,t=%d,p=%d$%s$%s", argonMem, argonTime, argonPar,
		base64.RawStdEncoding.EncodeToString(salt), base64.RawStdEncoding.EncodeToString(key))
}

func CheckPassword(hash, pw string) bool {
	switch {
	case strings.HasPrefix(hash, "$argon2id$"):
		parts := strings.Split(hash, "$")
		if len(parts) != 6 {
			return false
		}
		var m, t uint32
		var p uint8
		if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &m, &t, &p); err != nil {
			return false
		}
		salt, err1 := base64.RawStdEncoding.DecodeString(parts[4])
		want, err2 := base64.RawStdEncoding.DecodeString(parts[5])
		if err1 != nil || err2 != nil || m > 1<<20 || t > 16 {
			return false
		}
		got := argon2.IDKey([]byte(pw), salt, t, m, p, uint32(len(want)))
		return subtle.ConstantTimeCompare(got, want) == 1
	case strings.HasPrefix(hash, "$2"):
		return bcrypt.CompareHashAndPassword([]byte(hash), []byte(pw)) == nil
	}
	return false
}

// dummyHash makes "no such user" take as long as "wrong password".
var dummyHash = HashPassword("timing-equaliser")

// ---- who is asking ----

func currentUser(r *http.Request) *db.User {
	u, _ := r.Context().Value(keyUser).(*db.User)
	return u
}

func viaAPIKey(r *http.Request) bool {
	v, _ := r.Context().Value(keyViaAPIKey).(bool)
	return v
}

// withUser attaches the signed-in user, from an API key (Authorization:
// Bearer ph_...) or the session cookie.
func (a *App) withUser(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		if auth := r.Header.Get("Authorization"); auth != "" {
			key := strings.TrimSpace(strings.TrimPrefix(auth, "Bearer "))
			u, err := a.db.APIKeyUser(ctx, hashToken(key))
			if err != nil || u.Status != "active" {
				apiError(w, http.StatusUnauthorized, "invalid API key")
				return
			}
			ctx = context.WithValue(ctx, keyUser, u)
			ctx = context.WithValue(ctx, keyViaAPIKey, true)
		} else if c, err := r.Cookie(sessionCookie); err == nil && c.Value != "" {
			if u, err := a.db.SessionUser(ctx, hashToken(c.Value)); err == nil && u.Status == "active" {
				ctx = context.WithValue(ctx, keyUser, u)
			}
		}
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (a *App) member(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if currentUser(r) == nil {
			http.Redirect(w, r, "/login?next="+url.QueryEscape(r.URL.RequestURI()), http.StatusSeeOther)
			return
		}
		h(w, r)
	}
}

func (a *App) admin(h http.HandlerFunc) http.HandlerFunc {
	return a.member(func(w http.ResponseWriter, r *http.Request) {
		if !currentUser(r).IsAdmin() {
			a.fail(w, r, http.StatusForbidden, "This page is for admins.")
			return
		}
		h(w, r)
	})
}

func (a *App) startSession(w http.ResponseWriter, r *http.Request, u *db.User) error {
	tok := randomToken(32)
	if err := a.db.InsertSession(r.Context(), hashToken(tok), u.ID, config.SessionTTL, clientIP(r), trim(r.UserAgent(), 200)); err != nil {
		return err
	}
	u.LastLogin = time.Now().Unix()
	a.db.UpdateUser(r.Context(), u)
	http.SetCookie(w, &http.Cookie{
		Name: sessionCookie, Value: tok, Path: "/", HttpOnly: true, Secure: a.cfg.SecureCookies(),
		SameSite: http.SameSiteLaxMode, MaxAge: int(config.SessionTTL.Seconds()),
	})
	return nil
}

func safeNext(next string) string {
	if strings.HasPrefix(next, "/") && !strings.HasPrefix(next, "//") && !strings.HasPrefix(next, "/\\") {
		return next
	}
	return "/dashboard"
}

// ---- pages & forms ----

type authForm struct {
	Email, Name, Next, Error string
	Google, Registration     bool
}

func (a *App) authForm(r *http.Request) authForm {
	return authForm{Next: r.FormValue("next"), Google: a.cfg.GoogleEnabled(), Registration: a.registrationOpen(r.Context())}
}

func (a *App) loginPage(w http.ResponseWriter, r *http.Request) {
	if currentUser(r) != nil {
		http.Redirect(w, r, "/dashboard", http.StatusSeeOther)
		return
	}
	a.render(w, r, "login", "Sign in", a.authForm(r))
}

func (a *App) login(w http.ResponseWriter, r *http.Request) {
	f := a.authForm(r)
	f.Email = strings.TrimSpace(r.FormValue("email"))
	if !a.limits.allow("login:"+clientIP(r), 10, 15*time.Minute) {
		f.Error = "Too many attempts. Please wait a few minutes and try again."
		w.WriteHeader(http.StatusTooManyRequests)
		a.render(w, r, "login", "Sign in", f)
		return
	}
	u, err := a.db.UserByEmail(r.Context(), f.Email)
	pw := r.FormValue("password")
	if err != nil {
		CheckPassword(dummyHash, pw)
	}
	if err != nil || !CheckPassword(u.PassHash, pw) {
		f.Error = "That email and password don't match."
		w.WriteHeader(http.StatusUnauthorized)
		a.render(w, r, "login", "Sign in", f)
		return
	}
	if msg := statusMessage(u); msg != "" {
		f.Error = msg
		w.WriteHeader(http.StatusForbidden)
		a.render(w, r, "login", "Sign in", f)
		return
	}
	// Upgrade old hashes to the current parameters while we have the password.
	if !strings.HasPrefix(u.PassHash, fmt.Sprintf("$argon2id$v=19$m=%d,t=%d,p=%d$", argonMem, argonTime, argonPar)) {
		u.PassHash = HashPassword(pw)
	}
	if err := a.startSession(w, r, u); err != nil {
		a.serverError(w, r, err)
		return
	}
	http.Redirect(w, r, safeNext(f.Next), http.StatusSeeOther)
}

func statusMessage(u *db.User) string {
	switch u.Status {
	case "locked":
		return "This account is locked. Your images are still online; contact the site owner to unlock it."
	case "suspended":
		return "This account is suspended."
	}
	return ""
}

func (a *App) registrationOpen(ctx context.Context) bool {
	return a.db.Setting(ctx, "registration", fmt.Sprint(a.cfg.Registration)) == "true"
}

func (a *App) registerPage(w http.ResponseWriter, r *http.Request) {
	a.render(w, r, "register", "Create account", a.authForm(r))
}

func (a *App) register(w http.ResponseWriter, r *http.Request) {
	f := a.authForm(r)
	f.Email = strings.TrimSpace(r.FormValue("email"))
	f.Name = trim(strings.TrimSpace(r.FormValue("name")), 60)
	pw := r.FormValue("password")
	first := a.db.CountUsers(r.Context()) == 0
	switch {
	case !f.Registration && !first:
		f.Error = "Sign-ups are closed on this server."
	case !a.limits.allow("register:"+clientIP(r), 5, time.Hour):
		f.Error = "Too many sign-ups from your network. Try again later."
	case !validEmail(f.Email):
		f.Error = "Please enter a valid email address."
	case len(pw) < 8:
		f.Error = "Use at least 8 characters for your password."
	case len(pw) > 200:
		f.Error = "That password is too long."
	}
	if f.Error == "" {
		u := &db.User{Email: f.Email, Name: f.Name, PassHash: HashPassword(pw), QuotaBytes: a.cfg.DefaultQuota}
		if first {
			u.Role = "admin" // the first account runs the server
		}
		if err := a.db.InsertUser(r.Context(), u); err != nil {
			f.Error = "An account with this email already exists."
		} else if err := a.startSession(w, r, u); err != nil {
			a.serverError(w, r, err)
			return
		} else {
			http.Redirect(w, r, "/dashboard?welcome=1", http.StatusSeeOther)
			return
		}
	}
	w.WriteHeader(http.StatusBadRequest)
	a.render(w, r, "register", "Create account", f)
}

func validEmail(s string) bool {
	addr, err := mail.ParseAddress(s)
	return err == nil && addr.Address == s && strings.Contains(s, ".") && len(s) <= 254
}

func (a *App) logout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(sessionCookie); err == nil {
		a.db.DeleteSession(r.Context(), hashToken(c.Value))
	}
	http.SetCookie(w, &http.Cookie{Name: sessionCookie, Value: "", Path: "/", MaxAge: -1, HttpOnly: true, Secure: a.cfg.SecureCookies()})
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// ---- Sign in with Google (plain OAuth 2.0, no SDK) ----

const googleState = "ph_oauth"

func (a *App) googleStart(w http.ResponseWriter, r *http.Request) {
	if !a.cfg.GoogleEnabled() {
		a.notFound(w, r)
		return
	}
	state := randomToken(18)
	http.SetCookie(w, &http.Cookie{Name: googleState, Value: state + "|" + safeNext(r.FormValue("next")), Path: "/auth/",
		HttpOnly: true, Secure: a.cfg.SecureCookies(), SameSite: http.SameSiteLaxMode, MaxAge: 600})
	q := url.Values{
		"client_id": {a.cfg.GoogleID}, "redirect_uri": {a.cfg.GoogleRedirect}, "response_type": {"code"},
		"scope": {"openid email profile"}, "state": {state}, "prompt": {"select_account"},
	}
	http.Redirect(w, r, "https://accounts.google.com/o/oauth2/v2/auth?"+q.Encode(), http.StatusFound)
}

func (a *App) googleCallback(w http.ResponseWriter, r *http.Request) {
	c, err := r.Cookie(googleState)
	state, next, _ := strings.Cut(func() string {
		if err != nil {
			return ""
		}
		return c.Value
	}(), "|")
	if !a.cfg.GoogleEnabled() || state == "" || subtle.ConstantTimeCompare([]byte(state), []byte(r.FormValue("state"))) != 1 {
		a.fail(w, r, http.StatusBadRequest, "That sign-in link expired. Please try again.")
		return
	}
	info, err := a.googleExchange(r, r.FormValue("code"))
	if err != nil {
		a.log.Warn("google sign-in", "err", err)
		a.fail(w, r, http.StatusBadGateway, "Google sign-in didn't work. Please try again.")
		return
	}
	ctx := r.Context()
	u, err := a.db.UserByGoogle(ctx, info.Sub)
	if errors.Is(err, db.ErrNotFound) && info.EmailVerified {
		// Link to an existing account with the same (verified) email.
		if u, err = a.db.UserByEmail(ctx, info.Email); err == nil {
			u.GoogleID = sql.NullString{String: info.Sub, Valid: true}
		}
	}
	if errors.Is(err, db.ErrNotFound) {
		first := a.db.CountUsers(ctx) == 0
		if !a.registrationOpen(ctx) && !first {
			a.fail(w, r, http.StatusForbidden, "Sign-ups are closed on this server.")
			return
		}
		u = &db.User{Email: info.Email, Name: info.Name, AvatarURL: info.Picture, QuotaBytes: a.cfg.DefaultQuota,
			GoogleID: sql.NullString{String: info.Sub, Valid: true}}
		if first {
			u.Role = "admin"
		}
		err = a.db.InsertUser(ctx, u)
	}
	if err != nil {
		a.serverError(w, r, err)
		return
	}
	if msg := statusMessage(u); msg != "" {
		a.fail(w, r, http.StatusForbidden, msg)
		return
	}
	if u.AvatarURL == "" {
		u.AvatarURL = info.Picture
	}
	if err := a.startSession(w, r, u); err != nil {
		a.serverError(w, r, err)
		return
	}
	http.SetCookie(w, &http.Cookie{Name: googleState, Path: "/auth/", MaxAge: -1})
	http.Redirect(w, r, safeNext(next), http.StatusSeeOther)
}

type googleInfo struct {
	Sub           string `json:"sub"`
	Email         string `json:"email"`
	EmailVerified bool   `json:"email_verified"`
	Name          string `json:"name"`
	Picture       string `json:"picture"`
}

func (a *App) googleExchange(r *http.Request, code string) (*googleInfo, error) {
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()
	form := url.Values{"code": {code}, "client_id": {a.cfg.GoogleID}, "client_secret": {a.cfg.GoogleKey},
		"redirect_uri": {a.cfg.GoogleRedirect}, "grant_type": {"authorization_code"}}
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, "https://oauth2.googleapis.com/token", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var tok struct {
		AccessToken string `json:"access_token"`
		Error       string `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&tok); err != nil || tok.AccessToken == "" {
		return nil, fmt.Errorf("token exchange: %v %s", err, tok.Error)
	}
	req, _ = http.NewRequestWithContext(ctx, http.MethodGet, "https://openidconnect.googleapis.com/v1/userinfo", nil)
	req.Header.Set("Authorization", "Bearer "+tok.AccessToken)
	resp2, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp2.Body.Close()
	var info googleInfo
	if err := json.NewDecoder(resp2.Body).Decode(&info); err != nil || info.Sub == "" || info.Email == "" {
		return nil, fmt.Errorf("userinfo: %v", err)
	}
	return &info, nil
}

func trim(s string, n int) string {
	if len(s) <= n {
		return s
	}
	r := []rune(s)
	if len(r) > n {
		r = r[:n]
	}
	return string(r)
}

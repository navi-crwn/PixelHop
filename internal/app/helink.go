package app

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/navi-crwn/pixelhop/internal/db"
)

// Sign in with HEL.ink. HEL.ink owns the account (sign-up, password, Google,
// 2FA, email verification and resets); PixelHop only receives a short-lived
// ticket signed with the shared secret and keeps a local profile linked by
// the HEL.ink user id.

const helinkCookie = "ph_helink"

type helinkTicket struct {
	Sub    string `json:"sub"`
	Aud    string `json:"aud"` // the app the ticket is for; HEL.ink sends "pixelhop"
	Email  string `json:"email"`
	Name   string `json:"name"`
	Avatar string `json:"avatar"`
	Admin  bool   `json:"admin"`
	State  string `json:"state"`
	Iat    int64  `json:"iat"`
	Exp    int64  `json:"exp"`
	Jti    string `json:"jti"`

	// Ticket v2 (HOP App Protocol). Roles decide the local role when present
	// (promotion and demotion); Admin is only a fallback for v1 tickets. The
	// profile fields are copied when sent, since HOP ID owns the profile.
	V              int      `json:"v,omitempty"`
	Username       string   `json:"username,omitempty"`
	Roles          []string `json:"roles"` // nil = absent (v1); [] = no roles (demotes)
	Bio            *string  `json:"bio,omitempty"`
	Website        *string  `json:"website,omitempty"`
	ShowOwner      *bool    `json:"show_owner,omitempty"`
	ProfileVersion int      `json:"profile_version,omitempty"`
}

// adminRoles are the HOP ID roles that make someone a PixelHop admin.
var adminRoles = map[string]bool{"owner": true, "admin": true, "admin:pixelhop": true}

// roleFromHop maps HOP ID roles to the local role; ok is false when the
// ticket or event carries no roles at all (then nothing should change).
func roleFromHop(roles []string) (role string, ok bool) {
	if roles == nil {
		return "", false
	}
	for _, r := range roles {
		if adminRoles[r] {
			return "admin", true
		}
	}
	return "user", true
}

// applyHopProfile copies the profile fields HOP ID owns onto a local user.
// A username that breaks the local rules or belongs to someone else is left
// as it is: the person keeps their current handle.
func (a *App) applyHopProfile(ctx context.Context, u *db.User, username string, bio, website *string, showOwner *bool) {
	if h := strings.ToLower(strings.TrimSpace(strings.TrimPrefix(username, "@"))); h != "" &&
		usernameProblem(h) == "" && !a.db.UsernameTaken(ctx, h, u.ID) {
		u.Username = sql.NullString{String: h, Valid: true}
	}
	if bio != nil {
		u.Bio = trim(strings.TrimSpace(*bio), 300)
	}
	if website != nil {
		u.Website = trim(strings.TrimSpace(*website), 200)
	}
	if showOwner != nil {
		u.ShowOwner = *showOwner
	}
}

var errBadTicket = errors.New("this sign-in link is not valid any more")

// parseTicket checks the signature and lifetime of a HEL.ink ticket.
func parseTicket(raw string, secrets []string, now time.Time) (*helinkTicket, error) {
	body, sig, ok := strings.Cut(raw, ".")
	if !ok || body == "" || sig == "" {
		return nil, errBadTicket
	}
	got, err := base64.RawURLEncoding.DecodeString(sig)
	if err != nil {
		return nil, errBadTicket
	}
	var validSig bool
	for _, secret := range secrets {
		mac := hmac.New(sha256.New, []byte(secret))
		mac.Write([]byte(body))
		want := mac.Sum(nil)
		if hmac.Equal(got, want) {
			validSig = true
			break
		}
	}
	if !validSig {
		return nil, errBadTicket
	}
	js, err := base64.RawURLEncoding.DecodeString(body)
	if err != nil {
		return nil, errBadTicket
	}
	var t helinkTicket
	// A ticket made for another HEL.ink app (NoteHop…) is never valid here.
	if err := json.Unmarshal(js, &t); err != nil || t.Sub == "" || t.Email == "" || t.Jti == "" || (t.Aud != "" && t.Aud != "pixelhop") {
		return nil, errBadTicket
	}
	n := now.Unix()
	if t.Exp < n || t.Iat > n+60 || t.Exp-t.Iat > 600 {
		return nil, errBadTicket
	}
	return &t, nil
}

// usedTickets remembers ticket ids until they expire, so each works once.
var usedTickets = struct {
	sync.Mutex
	m map[string]int64
}{m: map[string]int64{}}

func firstUse(jti string, exp int64) bool {
	usedTickets.Lock()
	defer usedTickets.Unlock()
	now := time.Now().Unix()
	for k, e := range usedTickets.m {
		if e < now {
			delete(usedTickets.m, k)
		}
	}
	if _, seen := usedTickets.m[jti]; seen {
		return false
	}
	usedTickets.m[jti] = exp
	return true
}

// helinkStart sends the visitor to HEL.ink to sign in.
func (a *App) helinkStart(w http.ResponseWriter, r *http.Request) {
	if !a.cfg.SSOEnabled() {
		a.notFound(w, r)
		return
	}
	state := randomToken(24)
	http.SetCookie(w, &http.Cookie{Name: helinkCookie, Value: state + "|" + safeNext(r.FormValue("next")), Path: "/auth/",
		HttpOnly: true, Secure: a.cfg.SecureCookies(), SameSite: http.SameSiteLaxMode, MaxAge: 900})
	q := url.Values{"state": {state}, "return_to": {a.cfg.BaseURL + "/auth/helink/callback"}}
	http.Redirect(w, r, a.cfg.HelinkURL+"/sso/pixelhop?"+q.Encode(), http.StatusFound)
}

func (a *App) helinkCallback(w http.ResponseWriter, r *http.Request) {
	if !a.cfg.SSOEnabled() {
		a.notFound(w, r)
		return
	}
	c, err := r.Cookie(helinkCookie)
	if err != nil {
		a.fail(w, r, http.StatusBadRequest, "That sign-in took too long or came from another browser. Please try again.")
		return
	}
	state, next, _ := strings.Cut(c.Value, "|")
	t, err := parseTicket(r.FormValue("ticket"), a.cfg.HelinkSecrets(), time.Now())
	if err != nil || !hmac.Equal([]byte(t.State), []byte(state)) || !firstUse(t.Jti, t.Exp) {
		a.fail(w, r, http.StatusBadRequest, "This sign-in link is not valid any more. Please try again.")
		return
	}
	u, err := a.linkHelinkUser(r.Context(), t)
	if err != nil {
		a.serverError(w, r, err)
		return
	}
	if msg := statusMessage(u); msg != "" {
		a.fail(w, r, http.StatusForbidden, msg)
		return
	}
	if err := a.startSession(w, r, u); err != nil {
		a.serverError(w, r, err)
		return
	}
	http.SetCookie(w, &http.Cookie{Name: helinkCookie, Path: "/auth/", MaxAge: -1})
	http.Redirect(w, r, safeNext(next), http.StatusSeeOther)
}

// linkHelinkUser finds the PixelHop profile for a HEL.ink account: by HEL.ink
// id, then by email (HEL.ink only sends verified addresses), else a new one.
func (a *App) linkHelinkUser(ctx context.Context, t *helinkTicket) (*db.User, error) {
	u, err := a.db.UserByHelink(ctx, t.Sub)
	if errors.Is(err, db.ErrNotFound) {
		u, err = a.db.UserByEmail(ctx, t.Email)
		if err == nil {
			u.HelinkID = sql.NullString{String: t.Sub, Valid: true}
		}
	}
	if errors.Is(err, db.ErrNotFound) {
		u = &db.User{Email: t.Email, Name: trim(t.Name, 60), AvatarURL: t.Avatar, QuotaBytes: a.cfg.DefaultQuota,
			HelinkID: sql.NullString{String: t.Sub, Valid: true}}
		if role, ok := roleFromHop(t.Roles); ok {
			u.Role = role
		} else if t.Admin {
			u.Role = "admin"
		}
		if a.db.CountUsers(ctx) == 0 {
			u.Role = "admin"
		}
		if err := a.db.InsertUser(ctx, u); err != nil {
			return nil, err
		}
		if t.Username != "" || t.Bio != nil || t.Website != nil || t.ShowOwner != nil {
			a.applyHopProfile(ctx, u, t.Username, t.Bio, t.Website, t.ShowOwner)
			return u, a.db.UpdateUser(ctx, u)
		}
		return u, nil
	}
	if err != nil {
		return nil, err
	}
	// HEL.ink owns the email; the public name and photo are edited here, so
	// HEL.ink's only fill them in when they're empty.
	u.Email = t.Email
	if strings.TrimSpace(u.Name) == "" && t.Name != "" {
		u.Name = trim(t.Name, 60)
	}
	if t.Avatar != "" {
		u.AvatarURL = t.Avatar // used only while no photo is uploaded here
	}
	if role, ok := roleFromHop(t.Roles); ok {
		u.Role = role // HOP ID's roles promote and demote
	} else if t.Admin {
		u.Role = "admin" // v1 ticket: HEL.ink's owner runs PixelHop too; never demoted from here
	}
	a.applyHopProfile(ctx, u, t.Username, t.Bio, t.Website, t.ShowOwner)
	return u, a.db.UpdateUser(ctx, u)
}

// ---- Cloudflare Turnstile ----

// turnstileOK checks the widget's answer. Without keys configured it passes,
// so development and self-hosting without Cloudflare keep working, except
// when accounts come from HEL.ink: then the password forms check HEL.ink
// passwords and must not run unprotected ("Continue with HEL.ink" still works).
var turnstileVerifyURL = "https://challenges.cloudflare.com/turnstile/v0/siteverify"

func (a *App) turnstileOK(r *http.Request) bool {
	if !a.cfg.TurnstileEnabled() {
		return !a.cfg.SSOEnabled()
	}
	tok := r.FormValue("cf-turnstile-response")
	if tok == "" || len(tok) > 2048 {
		return false
	}
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	form := url.Values{"secret": {a.cfg.TurnstileSecret}, "response": {tok}, "remoteip": {clientIP(r)}}
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, turnstileVerifyURL, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		a.log.Warn("turnstile", "err", err)
		return false
	}
	defer resp.Body.Close()
	var out struct {
		Success bool `json:"success"`
	}
	return json.NewDecoder(resp.Body).Decode(&out) == nil && out.Success
}

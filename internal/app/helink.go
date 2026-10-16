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
	Email  string `json:"email"`
	Name   string `json:"name"`
	Avatar string `json:"avatar"`
	Admin  bool   `json:"admin"`
	State  string `json:"state"`
	Iat    int64  `json:"iat"`
	Exp    int64  `json:"exp"`
	Jti    string `json:"jti"`
}

var errBadTicket = errors.New("this sign-in link is not valid any more")

// parseTicket checks the signature and lifetime of a HEL.ink ticket.
func parseTicket(raw, secret string, now time.Time) (*helinkTicket, error) {
	body, sig, ok := strings.Cut(raw, ".")
	if !ok || body == "" || sig == "" {
		return nil, errBadTicket
	}
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(body))
	want := mac.Sum(nil)
	got, err := base64.RawURLEncoding.DecodeString(sig)
	if err != nil || !hmac.Equal(got, want) {
		return nil, errBadTicket
	}
	js, err := base64.RawURLEncoding.DecodeString(body)
	if err != nil {
		return nil, errBadTicket
	}
	var t helinkTicket
	if err := json.Unmarshal(js, &t); err != nil || t.Sub == "" || t.Email == "" || t.Jti == "" {
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
	t, err := parseTicket(r.FormValue("ticket"), a.cfg.HelinkSecret, time.Now())
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
		if t.Admin || a.db.CountUsers(ctx) == 0 {
			u.Role = "admin"
		}
		return u, a.db.InsertUser(ctx, u)
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
	if t.Admin {
		u.Role = "admin" // HEL.ink's owner runs PixelHop too; never demoted from here
	}
	return u, a.db.UpdateUser(ctx, u)
}

// ---- Cloudflare Turnstile ----

// turnstileOK checks the widget's answer. Without keys configured it always
// passes, so development and self-hosting without Cloudflare keep working.
func (a *App) turnstileOK(r *http.Request) bool {
	if !a.cfg.TurnstileEnabled() {
		return true
	}
	tok := r.FormValue("cf-turnstile-response")
	if tok == "" || len(tok) > 2048 {
		return false
	}
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	form := url.Values{"secret": {a.cfg.TurnstileSecret}, "response": {tok}, "remoteip": {clientIP(r)}}
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, "https://challenges.cloudflare.com/turnstile/v0/siteverify", strings.NewReader(form.Encode()))
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

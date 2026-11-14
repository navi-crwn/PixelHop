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
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/navi-crwn/pixelhop/internal/db"
	"github.com/navi-crwn/pixelhop/internal/netsafe"
)

// Sign in with HEL.ink. HEL.ink owns the account (sign-up, password, Google,
// 2FA, email verification and resets); PixelHop only receives a short-lived
// ticket signed with the shared secret and keeps a local profile linked by
// the HEL.ink user id.

const helinkCookie = "ph_helink"

// helinkModeDelete marks the trip that asks the hub for a fresh sign-in before an account's data is deleted.
const helinkModeDelete = "delete"

type helinkTicket struct {
	Sub    string `json:"sub"`
	Aud    string `json:"aud"` // the app the ticket is for; HEL.ink sends "pixelhop"
	Email  string `json:"email"`
	Name   string `json:"name"`
	Avatar string `json:"avatar"`
	Admin  bool   `json:"admin"`
	Status string `json:"status,omitempty"` // v2: "active"; anything else is refused
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

	// AuthTime is when the person last typed their credentials at the hub (unix seconds); 0 = the hub
	// does not say (an older hub, or a session restored from a "remember me" cookie).
	AuthTime int64 `json:"auth_time,omitempty"`
}

// deleteMaxAge is how recent the sign-in must be for the hub account's owner to delete their data here.
const deleteMaxAge = 300 * time.Second

// signedInRecently is the check a sign-in made with max_age must pass: the hub states when the person
// signed in, and it is no older than maxAge (plus a minute of clock drift). A browser can drop max_age
// from the address, but it cannot forge the signed auth_time; no auth_time at all never passes.
func signedInRecently(authTime int64, now time.Time, maxAge time.Duration) bool {
	if authTime <= 0 {
		return false
	}
	age := now.Unix() - authTime
	return age >= -60 && age <= int64((maxAge+time.Minute).Seconds())
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
// as it is: the person keeps their current handle. So is a website that holds
// the key of an encrypted note: the hub only checks that it is an http(s) address.
func (a *App) applyHopProfile(ctx context.Context, u *db.User, username string, bio, website *string, showOwner *bool) {
	if h := strings.ToLower(strings.TrimSpace(strings.TrimPrefix(username, "@"))); h != "" &&
		usernameProblem(h) == "" && !a.db.UsernameTaken(ctx, h, u.ID) {
		u.Username = sql.NullString{String: h, Valid: true}
	}
	if bio != nil {
		u.Bio = trim(strings.TrimSpace(netsafe.ScrubNoteKeys(*bio)), 300)
	}
	if website != nil && !holdsNoteKey(*website) {
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
	// prompt=none: the hub must never show a page. It answers with a ticket, or with error=login_required
	// to the callback, which then ends this app's session. The cookie remembers which kind of trip this is.
	silent := r.FormValue("prompt") == "none"
	mode := "-"
	if silent {
		mode = "none"
	}
	http.SetCookie(w, &http.Cookie{Name: helinkCookie, Value: state + "|" + mode + "|" + safeNext(r.FormValue("next")), Path: "/auth/",
		HttpOnly: true, Secure: a.cfg.SecureCookies(), SameSite: http.SameSiteLaxMode, MaxAge: 900})
	q := url.Values{"state": {state}, "return_to": {a.cfg.BaseURL + "/auth/helink/callback"}}
	if silent {
		q.Set("prompt", "none")
	}
	http.Redirect(w, r, a.cfg.HelinkURL+"/sso/pixelhop?"+q.Encode(), http.StatusFound)
}

// helinkReauth sends a signed-in HOP ID member to the hub to sign in again just now (max_age), on the way
// to deleting their data here. The cookie says this trip is for that and nothing else.
func (a *App) helinkReauth(w http.ResponseWriter, r *http.Request) {
	state := randomToken(24)
	http.SetCookie(w, &http.Cookie{Name: helinkCookie, Value: state + "|" + helinkModeDelete + "|/settings", Path: "/auth/",
		HttpOnly: true, Secure: a.cfg.SecureCookies(), SameSite: http.SameSiteLaxMode, MaxAge: 300})
	q := url.Values{"state": {state}, "return_to": {a.cfg.BaseURL + "/auth/helink/callback"},
		"max_age": {strconv.Itoa(int(deleteMaxAge.Seconds()))}}
	http.Redirect(w, r, a.cfg.HelinkURL+"/sso/pixelhop?"+q.Encode(), http.StatusFound)
}

// confirmedDelete finishes the trip helinkReauth began: the ticket must be for the account signed in
// here and must say the person signed in at the hub within the last few minutes. Otherwise nothing is deleted.
func (a *App) confirmedDelete(w http.ResponseWriter, r *http.Request, t *helinkTicket) {
	http.SetCookie(w, &http.Cookie{Name: helinkCookie, Path: "/auth/", MaxAge: -1})
	u := currentUser(r)
	if u == nil || !u.HelinkID.Valid || u.HelinkID.String != t.Sub {
		a.fail(w, r, http.StatusForbidden, a.tr(r, "That HOP ID sign-in was for a different account than the one you are using here, so nothing was deleted."))
		return
	}
	if !signedInRecently(t.AuthTime, time.Now(), deleteMaxAge) {
		a.fail(w, r, http.StatusForbidden, a.tr(r, "HOP ID did not confirm that you just signed in, so nothing was deleted. Please try again."))
		return
	}
	if _, err := a.purgeUser(r.Context(), u); err != nil {
		a.serverError(w, r, err)
		return
	}
	a.endSession(w, r)
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (a *App) helinkCallback(w http.ResponseWriter, r *http.Request) {
	if !a.cfg.SSOEnabled() {
		a.notFound(w, r)
		return
	}
	c, err := r.Cookie(helinkCookie)
	if err != nil {
		a.fail(w, r, http.StatusBadRequest, a.tr(r, "That sign-in took too long or came from another browser. Please try again."))
		return
	}
	state, rest, _ := strings.Cut(c.Value, "|")
	mode, next, _ := strings.Cut(rest, "|")
	silent := mode == "none"
	if e := r.FormValue("error"); e != "" {
		// Only the answer to our own quiet check counts (same state, a trip we made).
		if !silent || !hmac.Equal([]byte(r.FormValue("state")), []byte(state)) {
			a.fail(w, r, http.StatusBadRequest, a.tr(r, "This sign-in link is not valid any more. Please try again."))
			return
		}
		http.SetCookie(w, &http.Cookie{Name: helinkCookie, Path: "/auth/", MaxAge: -1})
		if e == "login_required" {
			// The hub has no session for this browser: whatever account this app held is no longer valid.
			// A public page carries on as a guest; the dashboard goes to the sign-in flow.
			if c, err := r.Cookie(sessionCookie); err == nil && c.Value != "" {
				a.db.DeleteSession(r.Context(), hashToken(c.Value))
			}
			http.SetCookie(w, &http.Cookie{Name: sessionCookie, Value: "", Path: "/", MaxAge: -1, HttpOnly: true, Secure: a.cfg.SecureCookies(), SameSite: http.SameSiteLaxMode})
		}
		// Any other error (temporarily_unavailable): keep the session, the cooldown cookie spaces the next try.
		http.Redirect(w, r, safeNext(next), http.StatusSeeOther)
		return
	}
	t, err := parseTicket(r.FormValue("ticket"), a.cfg.HelinkSecrets(), time.Now())
	if err != nil || !hmac.Equal([]byte(t.State), []byte(state)) || !firstUse(t.Jti, t.Exp) {
		a.fail(w, r, http.StatusBadRequest, a.tr(r, "This sign-in link is not valid any more. Please try again."))
		return
	}
	// The hub never issues a ticket for an account that is not active; a ticket that says otherwise is refused
	// before any local account is made or linked (older servers send no status).
	if t.Status != "" && t.Status != "active" {
		a.fail(w, r, http.StatusForbidden, a.tr(r, "This account is suspended."))
		return
	}
	if mode == helinkModeDelete {
		a.confirmedDelete(w, r, t)
		return
	}
	u, err := a.linkHelinkUser(r.Context(), t)
	if errors.Is(err, errLinkConflict) {
		a.fail(w, r, http.StatusConflict, a.tr(r, linkConflictMsg))
		return
	}
	if err != nil {
		a.serverError(w, r, err)
		return
	}
	if msg := a.statusMessage(r, u); msg != "" {
		a.fail(w, r, http.StatusForbidden, msg)
		return
	}
	if err := a.startHubSession(w, r, u); err != nil {
		a.serverError(w, r, err)
		return
	}
	http.SetCookie(w, &http.Cookie{Name: helinkCookie, Path: "/auth/", MaxAge: -1})
	http.Redirect(w, r, safeNext(next), http.StatusSeeOther)
}

// errLinkConflict means the HOP ID cannot be linked without touching a PixelHop account that is not
// its own: its e-mail belongs to a row linked to another HOP ID, or to another row altogether.
var errLinkConflict = errors.New("this HOP ID's e-mail belongs to another PixelHop account")

const linkConflictMsg = "This HOP ID can't be linked to PixelHop because its e-mail already belongs to another PixelHop account. Please contact the site owner."

// linkHelinkUser finds the PixelHop profile for a HEL.ink account: by HEL.ink
// id, then by email (HEL.ink only sends verified addresses), else a new one.
//
// Linking an older account by e-mail is the one moment a local row changes hands. That row never had a
// verified e-mail, so whoever knows its old password or Google login must not come along: both are
// cleared and every session of the row ends (the new one is started by the caller), and what the person
// filled in here (bio, website) is not wiped by an empty profile on the hub. A row already
// linked to a different HOP ID is never taken over, and a hub e-mail that another row holds changes nothing.
func (a *App) linkHelinkUser(ctx context.Context, t *helinkTicket) (*db.User, error) {
	u, err := a.db.UserByHelink(ctx, t.Sub)
	firstLink := false
	if errors.Is(err, db.ErrNotFound) {
		u, err = a.db.UserByEmail(ctx, t.Email)
		if err == nil {
			if u.HelinkID.Valid {
				return nil, errLinkConflict // another HOP ID already owns this row
			}
			u.HelinkID = sql.NullString{String: t.Sub, Valid: true}
			u.PassHash = ""
			u.GoogleID = sql.NullString{}
			firstLink = true
		}
	}
	if errors.Is(err, db.ErrNotFound) {
		u = &db.User{Email: t.Email, Name: trim(netsafe.ScrubNoteKeys(t.Name), 60), AvatarURL: t.Avatar, QuotaBytes: a.cfg.DefaultQuota,
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
	// HEL.ink owns the email. Another row holding it would break the unique index: say so, change nothing.
	if !strings.EqualFold(u.Email, t.Email) && a.db.EmailTaken(ctx, t.Email, u.ID) {
		return nil, errLinkConflict
	}
	u.Email = t.Email
	// The public name and photo were edited here, so HEL.ink's only fill them in when they're empty.
	if strings.TrimSpace(u.Name) == "" && t.Name != "" {
		u.Name = trim(netsafe.ScrubNoteKeys(t.Name), 60)
	}
	if t.Avatar != "" {
		u.AvatarURL = t.Avatar // used only while no photo is uploaded here
	}
	if role, ok := roleFromHop(t.Roles); ok {
		u.Role = role // HOP ID's roles promote and demote
	} else if t.Admin {
		u.Role = "admin" // v1 ticket: HEL.ink's owner runs PixelHop too; never demoted from here
	}
	bio, website := u.Bio, u.Website
	a.applyHopProfile(ctx, u, t.Username, t.Bio, t.Website, t.ShowOwner)
	if firstLink {
		// An empty profile on the hub does not erase what this person wrote here before linking.
		if u.Bio == "" {
			u.Bio = bio
		}
		if u.Website == "" {
			u.Website = website
		}
	}
	if err := a.db.UpdateUser(ctx, u); err != nil {
		return nil, err
	}
	if firstLink {
		// Whatever could act for the row without the hub (sessions, API keys, webhooks) ends with the link.
		a.db.DeleteUserSessions(ctx, u.ID)
		a.db.DeleteUserAPIKeys(ctx, u.ID)
		a.db.DeleteUserWebhooks(ctx, u.ID)
	}
	return u, nil
}

// ---- Cloudflare Turnstile ----

// turnstileOK checks the widget's answer. Without keys configured it passes,
// so development and self-hosting without Cloudflare keep working.
var turnstileVerifyURL = "https://challenges.cloudflare.com/turnstile/v0/siteverify"

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

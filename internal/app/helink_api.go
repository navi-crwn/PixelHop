package app

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/navi-crwn/pixelhop/internal/db"
)

// With HEL.ink accounts switched on, PixelHop's own sign-in and sign-up
// forms still work: they check the password with HEL.ink (server to server,
// signed with the shared secret), so one account works in both apps and
// nobody has to visit HEL.ink first. People with two-factor codes are sent
// through the normal "Continue with HEL.ink" flow, which asks for the code.

type helinkAccount struct {
	Sub      string `json:"sub"`
	Email    string `json:"email"`
	Name     string `json:"name"`
	Avatar   string `json:"avatar"`
	Admin    bool   `json:"admin"`
	Verified bool   `json:"verified"`
}

type helinkAnswer struct {
	Status  int
	Account *helinkAccount
	Code    string // bad_credentials, two_factor, suspended, invalid, too_many_attempts…
	Message string
}

var errHelinkDown = errors.New("HEL.ink didn't answer")

func (a *App) helinkClient() *http.Client {
	t := http.DefaultTransport.(*http.Transport).Clone()
	if addr := a.cfg.HelinkAPIAddr; addr != "" {
		// Talk to HEL.ink on this machine (same certificate, no round trip
		// through the CDN). The TLS name stays HEL.ink's own.
		d := &net.Dialer{Timeout: 5 * time.Second}
		t.DialContext = func(ctx context.Context, network, _ string) (net.Conn, error) {
			return d.DialContext(ctx, network, addr)
		}
	}
	return &http.Client{Timeout: 15 * time.Second, Transport: t}
}

func (a *App) helinkCall(ctx context.Context, path string, body any) (*helinkAnswer, error) {
	js, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	ts := strconv.FormatInt(time.Now().Unix(), 10)
	mac := hmac.New(sha256.New, []byte(a.cfg.HelinkSecret))
	mac.Write([]byte("pixelhop-api|" + ts + "|"))
	mac.Write(js)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, a.cfg.HelinkURL+path, bytes.NewReader(js))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "PixelHop/"+Version)
	req.Header.Set("X-PixelHop-Timestamp", ts)
	req.Header.Set("X-PixelHop-Signature", base64.RawURLEncoding.EncodeToString(mac.Sum(nil)))
	resp, err := a.helinkClient().Do(req)
	if err != nil {
		a.log.Warn("helink api", "path", path, "err", err)
		return nil, errHelinkDown
	}
	defer resp.Body.Close()
	var out struct {
		User    *helinkAccount `json:"user"`
		Error   string         `json:"error"`
		Message string         `json:"message"`
	}
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
	if json.Unmarshal(raw, &out) != nil {
		a.log.Warn("helink api", "path", path, "status", resp.StatusCode, "body", trim(string(raw), 200))
		return nil, errHelinkDown
	}
	ans := &helinkAnswer{Status: resp.StatusCode, Account: out.User, Code: out.Error, Message: out.Message}
	if resp.StatusCode/100 == 2 && (out.User == nil || out.User.Sub == "" || out.User.Email == "") {
		return nil, errHelinkDown
	}
	return ans, nil
}

// helinkLogin checks email and password with HEL.ink. It returns the
// PixelHop user to sign in, or a message for the form, or redirect=true when
// the person must use "Continue with HEL.ink" (two-factor codes).
func (a *App) helinkLogin(r *http.Request, email, password string) (u *db.User, msg string, redirect bool) {
	ans, err := a.helinkCall(r.Context(), "/api/pixelhop/login", map[string]string{"email": email, "password": password, "ip": clientIP(r)})
	if err != nil {
		return nil, "We couldn't reach HEL.ink to check your password. Try “Continue with HEL.ink”, or again in a minute.", false
	}
	switch {
	case ans.Status == http.StatusOK:
	case ans.Code == "two_factor":
		return nil, "", true
	case ans.Code == "suspended":
		return nil, "This account is suspended.", false
	case ans.Code == "too_many_attempts" || ans.Status == http.StatusTooManyRequests:
		return nil, "Too many attempts. Please wait a few minutes and try again.", false
	default:
		return nil, "That email and password don't match.", false
	}
	u, err = a.accountFromHelink(r.Context(), ans.Account)
	if err != nil {
		if errors.Is(err, errNeedsVerifiedEmail) {
			return nil, "Confirm your email first: open the link HEL.ink sent to " + ans.Account.Email + ", then sign in again.", false
		}
		a.log.Error("helink login", "err", err)
		return nil, "Sign-in failed on our side. Please try again.", false
	}
	return u, "", false
}

var errNeedsVerifiedEmail = errors.New("email not confirmed")

// accountFromHelink links a HEL.ink account to its PixelHop profile. An
// unconfirmed address may only reach a profile that is already linked to
// that HEL.ink account, or start a new one; it never takes over an existing
// profile with the same email.
func (a *App) accountFromHelink(ctx context.Context, acc *helinkAccount) (*db.User, error) {
	t := &helinkTicket{Sub: acc.Sub, Email: acc.Email, Name: acc.Name, Avatar: acc.Avatar, Admin: acc.Admin}
	if !acc.Verified {
		if _, err := a.db.UserByHelink(ctx, acc.Sub); errors.Is(err, db.ErrNotFound) {
			if _, err := a.db.UserByEmail(ctx, acc.Email); err == nil {
				return nil, errNeedsVerifiedEmail
			}
		}
		t.Admin = false // HEL.ink's owner always has a confirmed address
	}
	return a.linkHelinkUser(ctx, t)
}

// helinkRegister creates the account on HEL.ink (which emails a
// confirmation link) and its PixelHop profile, already signed in here.
func (a *App) helinkRegister(r *http.Request, name, email, password string) (*db.User, string) {
	if _, err := a.db.UserByEmail(r.Context(), email); err == nil {
		return nil, "An account with this email already exists. Sign in instead."
	}
	if name == "" {
		name, _, _ = strings.Cut(email, "@")
	}
	ans, err := a.helinkCall(r.Context(), "/api/pixelhop/register", map[string]any{
		"name": name, "email": email, "password": password, "tos_accepted": true, "ip": clientIP(r)})
	if err != nil {
		return nil, "We couldn't reach HEL.ink to create your account. Please try again in a minute."
	}
	if ans.Status != http.StatusCreated {
		if ans.Message != "" {
			return nil, ans.Message
		}
		return nil, "We couldn't create that account. Please check the details and try again."
	}
	u, err := a.accountFromHelink(r.Context(), ans.Account)
	if err != nil {
		a.log.Error("helink register", "err", err)
		return nil, "Your account was made, but signing in failed. Please sign in."
	}
	return u, ""
}

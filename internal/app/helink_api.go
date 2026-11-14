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
// Nobody gets in (or can upload) before confirming their email on HEL.ink.

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
		return nil, a.tr(r, "We couldn't reach HEL.ink to check your password. Try “Continue with HEL.ink”, or again in a minute."), false
	}
	switch {
	case ans.Status == http.StatusOK:
	case ans.Code == "two_factor":
		return nil, "", true
	case ans.Code == "suspended":
		return nil, a.tr(r, "This account is suspended."), false
	case ans.Code == "unverified":
		return nil, a.confirmEmailMsg(r, email), false
	case ans.Code == "banned":
		return nil, a.tr(r, "Sign-in is blocked from your network."), false
	case ans.Code == "too_many_attempts" || ans.Status == http.StatusTooManyRequests:
		return nil, a.tr(r, "Too many attempts. Please wait a few minutes and try again."), false
	default:
		return nil, a.tr(r, "That email and password don't match."), false
	}
	u, err = a.accountFromHelink(r.Context(), ans.Account)
	if err != nil {
		if errors.Is(err, errNeedsVerifiedEmail) {
			return nil, a.confirmEmailMsg(r, ans.Account.Email), false
		}
		if errors.Is(err, errLinkConflict) {
			return nil, a.tr(r, linkConflictMsg), false
		}
		a.log.Error("helink login", "err", err)
		return nil, a.tr(r, "Sign-in failed on our side. Please try again."), false
	}
	return u, "", false
}

var errNeedsVerifiedEmail = errors.New("email not confirmed")

func (a *App) confirmEmailMsg(r *http.Request, email string) string {
	return a.tr(r, "Confirm your email first: open the link HEL.ink sent to {email}, then sign in again.", "email", email)
}

// accountFromHelink links a confirmed HEL.ink account to its PixelHop
// profile. HEL.ink already refuses unconfirmed accounts; this is the second lock.
func (a *App) accountFromHelink(ctx context.Context, acc *helinkAccount) (*db.User, error) {
	if !acc.Verified {
		return nil, errNeedsVerifiedEmail
	}
	return a.linkHelinkUser(ctx, &helinkTicket{Sub: acc.Sub, Email: acc.Email, Name: acc.Name, Avatar: acc.Avatar, Admin: acc.Admin})
}

// helinkRegister creates the account on HEL.ink, which emails a
// confirmation link. The PixelHop profile is made at the first sign-in after
// the address is confirmed. It returns a notice on success, else an error.
func (a *App) helinkRegister(r *http.Request, name, email, password string) (notice, errMsg string) {
	if _, err := a.db.UserByEmail(r.Context(), email); err == nil {
		return "", a.tr(r, "An account with this email already exists. Sign in instead.")
	}
	if name == "" {
		name, _, _ = strings.Cut(email, "@")
	}
	ans, err := a.helinkCall(r.Context(), "/api/pixelhop/register", map[string]any{
		"name": name, "email": email, "password": password, "tos_accepted": true, "ip": clientIP(r)})
	if err != nil {
		return "", a.tr(r, "We couldn't reach HEL.ink to create your account. Please try again in a minute.")
	}
	if ans.Status != http.StatusCreated {
		if ans.Message != "" {
			return "", ans.Message
		}
		return "", a.tr(r, "We couldn't create that account. Please check the details and try again.")
	}
	return a.tr(r, "Account created. Open the link HEL.ink sent to {email} to confirm your email, then sign in.", "email", email), ""
}

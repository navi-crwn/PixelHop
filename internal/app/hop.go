package app

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/netip"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/navi-crwn/pixelhop/internal/db"
)

// The HOP App Protocol: a signed server-to-server API under /hop/* that lets
// a control center read and moderate this PixelHop, and lets HOP ID push
// account events (profile changes, suspensions, deletions). Every call is
// signed with the shared SSO secret by one of the configured client apps;
// there are no cookies, no CSRF and no sessions involved. Every change made
// through it is written to the hop_audit table.

const (
	hopMaxAge  = 120 * time.Second
	hopMaxBody = 2 << 20
)

type hopCaller struct {
	App, Actor string
	Body       []byte
}

const keyHop ctxKey = 100

func hopOf(r *http.Request) *hopCaller {
	c, _ := r.Context().Value(keyHop).(*hopCaller)
	return c
}

func (a *App) hopRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /hop/health", a.hopAuth(a.hopHealth))
	mux.HandleFunc("GET /hop/manifest", a.hopAuth(a.hopManifest))
	mux.HandleFunc("GET /hop/stats", a.hopAuth(a.hopStats))
	mux.HandleFunc("GET /hop/users/{hop_id}", a.hopAuth(a.hopUser))
	mux.HandleFunc("POST /hop/users/{hop_id}/actions", a.hopAuth(a.hopUserAction))
	mux.HandleFunc("POST /hop/profile", a.hopAuth(a.hopProfile))
	mux.HandleFunc("POST /hop/events", a.hopAuth(a.hopEvent))
	mux.HandleFunc("GET /hop/content", a.hopAuth(a.hopContent))
	mux.HandleFunc("GET /hop/content/{type}/{id}", a.hopAuth(a.hopContentDetail))
	mux.HandleFunc("POST /hop/content/{type}/{id}/actions", a.hopAuth(a.hopContentAction))
	mux.HandleFunc("GET /hop/reports", a.hopAuth(a.hopReports))
	mux.HandleFunc("POST /hop/reports/{id}/actions", a.hopAuth(a.hopReportAction))
	mux.HandleFunc("GET /hop/settings", a.hopAuth(a.hopSettings))
	mux.HandleFunc("PUT /hop/settings", a.hopAuth(a.hopPutSettings))
	mux.HandleFunc("GET /hop/search", a.hopAuth(a.hopSearch))
	mux.HandleFunc("GET /hop/metrics", a.hopAuth(a.writeMetrics))
	mux.HandleFunc("GET /hop/x/ip", a.hopAuth(a.hopIPBans))
	mux.HandleFunc("POST /hop/x/ip/actions", a.hopAuth(a.hopIPBanAction))
	mux.HandleFunc("GET /hop/audit", a.hopAuth(a.hopAudit))
	mux.HandleFunc("/hop/", a.hopAuth(func(w http.ResponseWriter, r *http.Request) {
		hopFail(w, http.StatusNotFound, "not_found", "No such endpoint.")
	}))
}

// hopAuth checks the signature of a call and hands the caller to the handler.
func (a *App) hopAuth(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !a.cfg.SSOEnabled() {
			hopFail(w, http.StatusNotFound, "not_configured", "The HOP App Protocol needs HELINK_URL and HELINK_SSO_SECRET.")
			return
		}
		body, err := io.ReadAll(io.LimitReader(r.Body, hopMaxBody+1))
		if err != nil || len(body) > hopMaxBody {
			hopFail(w, http.StatusRequestEntityTooLarge, "too_large", "The request body is too large.")
			return
		}
		app := r.Header.Get("X-HOP-App")
		allowed := false
		for _, s := range a.cfg.HopAdminClients {
			if s == app {
				allowed = true
			}
		}
		if !allowed {
			hopFail(w, http.StatusUnauthorized, "bad_signature", "Unknown calling app.")
			return
		}
		if err := a.hopNonces.Verify(r, body, app, a.cfg.HelinkSecrets(), hopMaxAge); err != nil {
			hopFail(w, http.StatusUnauthorized, err.Error(), "The request signature was not accepted.")
			return
		}
		actor := trim(r.Header.Get("X-HOP-Actor"), 100)
		if actor == "" {
			actor = "system"
		}
		c := &hopCaller{App: app, Actor: actor, Body: body}
		h(w, r.WithContext(context.WithValue(r.Context(), keyHop, c)))
	}
}

// ---- envelope ----

func hopOK(w http.ResponseWriter, data any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"ok": true, "data": data})
}

func hopPage(w http.ResponseWriter, data any, next string, total int) {
	w.Header().Set("Content-Type", "application/json")
	page := map[string]any{"total": total}
	if next != "" {
		page["next"] = next
	}
	json.NewEncoder(w).Encode(map[string]any{"ok": true, "data": data, "page": page})
}

func hopFail(w http.ResponseWriter, status int, code, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]any{"ok": false, "error": code, "message": msg})
}

// hopJSON decodes the signed body into v; false means an answer was sent.
func hopJSON(w http.ResponseWriter, r *http.Request, v any) bool {
	body := hopOf(r).Body
	if len(body) == 0 {
		body = []byte("{}")
	}
	if err := json.Unmarshal(body, v); err != nil {
		hopFail(w, http.StatusBadRequest, "bad_json", "The body is not valid JSON: "+err.Error())
		return false
	}
	return true
}

func isoTime(t int64) any {
	if t == 0 {
		return nil
	}
	return time.Unix(t, 0).UTC().Format(time.RFC3339)
}

// hopCursor reads the offset-style cursor and page size of a list call.
func hopCursor(r *http.Request, def, max int) (limit, offset int) {
	q := r.URL.Query()
	limit, _ = strconv.Atoi(q.Get("limit"))
	if limit <= 0 || limit > max {
		limit = def
	}
	offset, _ = strconv.Atoi(q.Get("cursor"))
	if offset < 0 {
		offset = 0
	}
	return
}

func nextCursor(offset, n, total int) string {
	if offset+n >= total || n == 0 {
		return ""
	}
	return strconv.Itoa(offset + n)
}

// audit records one change made through the protocol.
func (a *App) audit(r *http.Request, action, target string, detail map[string]any) {
	c := hopOf(r)
	js, _ := json.Marshal(detail)
	if detail == nil {
		js = []byte("{}")
	}
	if err := a.db.InsertAudit(r.Context(), &db.AuditEntry{Actor: c.Actor, App: c.App, Action: action, Target: target, Detail: js}); err != nil {
		a.log.Error("hop audit", "action", action, "err", err)
	}
}

// hopActionRequest is the body of every ".../actions" call.
type hopActionRequest struct {
	Action string         `json:"action"`
	Reason string         `json:"reason"`
	Params map[string]any `json:"params"`
}

func (q *hopActionRequest) str(key string) string {
	switch v := q.Params[key].(type) {
	case string:
		return v
	case float64:
		return strconv.FormatInt(int64(v), 10)
	}
	return ""
}

func (q *hopActionRequest) num(key string) (int64, bool) {
	switch v := q.Params[key].(type) {
	case float64:
		return int64(v), true
	case string:
		n, err := strconv.ParseInt(strings.TrimSpace(v), 10, 64)
		return n, err == nil
	case json.Number:
		n, err := v.Int64()
		return n, err == nil
	}
	return 0, false
}

// ---- health, manifest, stats ----

type hopCheck struct {
	Name   string `json:"name"`
	OK     bool   `json:"ok"`
	Detail string `json:"detail"`
}

func (a *App) hopHealth(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var checks []hopCheck
	dbOK := "ok"
	var one int
	if err := a.db.QueryRowContext(ctx, `SELECT 1`).Scan(&one); err != nil || one != 1 {
		dbOK = "error"
		checks = append(checks, hopCheck{"db", false, fmt.Sprint(err)})
	} else {
		checks = append(checks, hopCheck{"db", true, "sqlite"})
	}
	storageOK := "ok"
	if a.store.Name() == "local" {
		if st, err := os.Stat(a.cfg.DataDir + "/images"); err != nil || !st.IsDir() {
			storageOK = "error"
			checks = append(checks, hopCheck{"storage", false, "images directory missing"})
		} else {
			free, total := diskSpace(a.cfg.DataDir)
			checks = append(checks, hopCheck{"storage", true, fmt.Sprintf("local, %s free of %s", humanBytes(free), humanBytes(total))})
		}
	} else {
		checks = append(checks, hopCheck{"storage", true, a.store.Name()})
	}
	sso := "off"
	if a.cfg.SSOEnabled() {
		sso = "ok"
	}
	checks = append(checks, hopCheck{"sso", sso == "ok", sso})
	hopOK(w, map[string]any{
		"app": "pixelhop", "version": Version, "commit": "", "uptime_s": int64(time.Since(a.started).Seconds()),
		"db": dbOK, "storage": storageOK, "sso": sso, "checks": checks,
	})
}

// hopSettingSchema describes the runtime settings the control center may edit.
var hopSettingSchema = []map[string]any{
	{"key": "guest_uploads", "type": "bool", "description": "Let visitors upload without an account", "restart_required": false},
	{"key": "registration", "type": "bool", "description": "Let people create accounts", "restart_required": false},
	{"key": "announcement", "type": "string", "description": "Banner shown on every page (empty = none)", "restart_required": false},
	{"key": "seo_title", "type": "string", "description": "Title of the home page", "restart_required": false},
	{"key": "seo_description", "type": "string", "description": "Description of the home page for search engines", "restart_required": false},
}

func (a *App) hopManifest(w http.ResponseWriter, r *http.Request) {
	hopOK(w, map[string]any{
		"app": "pixelhop", "name": a.cfg.SiteName, "icon": "image", "base_url": a.cfg.BaseURL, "version": Version,
		"capabilities": []string{"admin_api", "events", "account_api", "metrics"},
		"content_types": []map[string]any{
			{"type": "image", "label": "Images", "actions": []string{"hide", "unhide", "delete", "ban_ip", "transfer"}},
			{"type": "album", "label": "Albums", "actions": []string{"delete", "transfer"}},
		},
		"settings_schema": hopSettingSchema,
		"extensions":      []string{"ip"},
		"actions": map[string]any{
			"user":    []string{"suspend", "lock", "activate", "revoke_sessions", "revoke_keys", "set_quota", "warn", "purge", "export"},
			"report":  []string{"dismiss", "resolve", "delete"},
			"content": []string{"hide", "unhide", "delete", "ban_ip", "transfer"},
		},
	})
}

func (a *App) hopStats(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	s := a.db.Stats(ctx)
	images, videos, pdfs := a.db.KindCounts(ctx)
	byDay := map[string]db.DayCount{}
	for _, d := range s.Daily {
		byDay[d.Day] = d
	}
	today := time.Now().UTC()
	days := 14
	if since, err := time.Parse("2006-01-02", strings.TrimSpace(r.URL.Query().Get("since"))); err == nil {
		if n := int(today.Sub(since).Hours()/24) + 1; n >= 1 && n <= 30 {
			days = n
		}
	}
	uploads, bytes := make([][2]any, 0, days), make([][2]any, 0, days)
	for i := days - 1; i >= 0; i-- {
		day := today.AddDate(0, 0, -i).Format("2006-01-02")
		dc := byDay[day]
		uploads = append(uploads, [2]any{day, dc.Count})
		bytes = append(bytes, [2]any{day, dc.Bytes})
	}
	guest := 0
	if a.db.Setting(ctx, "guest_uploads", fmt.Sprint(a.cfg.GuestUploads)) == "true" {
		guest = 1
	}
	kpi := map[string]any{
		"users": s.Users, "images": images, "videos": videos, "pdfs": pdfs, "content": s.Images,
		"albums": s.Albums, "bytes": s.Bytes, "views": s.Views,
		"uploads_today": byDay[today.Format("2006-01-02")].Count,
		"open_reports":  s.OpenReports, "bans": s.Bans, "guest_uploads": guest,
	}
	if a.store.Name() == "local" {
		if free, total := diskSpace(a.cfg.DataDir); total > 0 {
			kpi["disk_free_bytes"], kpi["disk_total_bytes"] = free, total
		}
	}
	hopOK(w, map[string]any{
		"kpi":    kpi,
		"series": map[string]any{"uploads": uploads, "bytes": bytes},
		"step":   "day",
	})
}

// ---- users ----

// hopFindUser resolves a HOP id to the linked local account.
func (a *App) hopFindUser(w http.ResponseWriter, r *http.Request, hopID string) *db.User {
	hopID = strings.TrimSpace(hopID)
	if hopID == "" {
		hopFail(w, http.StatusBadRequest, "bad_request", "A HOP user id is needed.")
		return nil
	}
	u, err := a.db.UserByHelink(r.Context(), hopID)
	if errors.Is(err, db.ErrNotFound) {
		hopFail(w, http.StatusNotFound, "not_found", "No account on this app is linked to that HOP id.")
		return nil
	}
	if err != nil {
		hopFail(w, http.StatusInternalServerError, "db_error", err.Error())
		return nil
	}
	return u
}

func (a *App) hopUserJSON(ctx context.Context, u *db.User) map[string]any {
	bytes, _ := a.db.UserStorage(ctx, u.ID)
	sum := a.db.UserSummary(ctx, u.ID)
	return map[string]any{
		"sub": u.HelinkID.String, "local_id": u.ID, "email": u.Email, "name": u.Name, "username": u.Username.String,
		"status": u.Status, "status_note": u.StatusNote, "role": u.Role, "warning": u.Warning,
		"quota_bytes": u.QuotaBytes, "storage_bytes": bytes,
		"images": sum.Images + sum.Videos + sum.PDFs, "albums": sum.Albums,
		"api_keys": a.db.CountAPIKeys(ctx, u.ID), "webhooks": a.db.CountWebhooks(ctx, u.ID), "sessions": a.db.CountSessions(ctx, u.ID),
		"created_at": isoTime(u.CreatedAt), "last_login": isoTime(u.LastLogin), "last_seen": isoTime(u.LastLogin),
		"helink_linked": u.HelinkID.Valid, "bio": u.Bio, "website": u.Website, "show_owner": u.ShowOwner,
		"profile_url": a.cfg.BaseURL + "/u/" + u.Username.String,
	}
}

func (a *App) hopUser(w http.ResponseWriter, r *http.Request) {
	u := a.hopFindUser(w, r, r.PathValue("hop_id"))
	if u == nil {
		return
	}
	hopOK(w, a.hopUserJSON(r.Context(), u))
}

func (a *App) hopUserAction(w http.ResponseWriter, r *http.Request) {
	var q hopActionRequest
	if !hopJSON(w, r, &q) {
		return
	}
	u := a.hopFindUser(w, r, r.PathValue("hop_id"))
	if u == nil {
		return
	}
	ctx := r.Context()
	reason := trim(strings.TrimSpace(q.Reason), 300)
	detail := map[string]any{"reason": reason}
	extra := map[string]any{}
	var err error
	switch q.Action {
	case "suspend", "lock", "activate":
		err = a.setUserStatus(ctx, u, map[string]string{"suspend": "suspended", "lock": "locked", "activate": "active"}[q.Action], reason)
	case "revoke_sessions":
		a.db.DeleteUserSessions(ctx, u.ID)
	case "revoke_keys":
		extra["revoked"] = a.db.DeleteUserAPIKeys(ctx, u.ID)
	case "set_quota":
		n, ok := q.num("quota_bytes")
		if !ok || n < 0 {
			hopFail(w, http.StatusBadRequest, "bad_params", "params.quota_bytes must be a number of bytes, 0 or more.")
			return
		}
		u.QuotaBytes = n
		detail["quota_bytes"] = n
		err = a.db.UpdateUser(ctx, u)
	case "warn":
		u.Warning = trim(strings.TrimSpace(q.str("text")), 1000)
		detail["text"] = u.Warning
		err = a.db.UpdateUser(ctx, u)
	case "purge":
		counts := map[string]any{"images": a.purgeUser(ctx, u)}
		detail["counts"] = counts
		a.audit(r, "user.purge", u.HelinkID.String, detail)
		hopOK(w, map[string]any{"purged": true, "counts": counts, "local_id": u.ID})
		return
	case "export":
		hopOK(w, map[string]any{"url": nil, "message": "not implemented"})
		return
	default:
		hopFail(w, http.StatusBadRequest, "unknown_action", "Unknown user action "+strconv.Quote(q.Action)+".")
		return
	}
	if err != nil {
		hopFail(w, http.StatusInternalServerError, "db_error", err.Error())
		return
	}
	a.audit(r, "user."+q.Action, u.HelinkID.String, detail)
	u, _ = a.db.UserByID(ctx, u.ID)
	out := a.hopUserJSON(ctx, u)
	for k, v := range extra {
		out[k] = v
	}
	hopOK(w, out)
}

// setUserStatus applies a status the way the admin panel does: suspended
// and locked accounts are signed out everywhere, suspended accounts' images
// go offline, and activating brings back only what the suspension hid.
func (a *App) setUserStatus(ctx context.Context, u *db.User, status, note string) error {
	if status != "active" && status != "locked" && status != "suspended" {
		return fmt.Errorf("unknown status %q", status)
	}
	u.Status = status
	u.StatusNote = trim(note, 300)
	if err := a.db.UpdateUser(ctx, u); err != nil {
		return err
	}
	if status != "active" {
		a.db.DeleteUserSessions(ctx, u.ID)
	}
	if status == "suspended" {
		return a.db.SuspendUserImages(ctx, u.ID)
	}
	return a.db.RestoreUserImages(ctx, u.ID)
}

// hopProfilePush is what HOP ID sends when a profile changes.
type hopProfilePush struct {
	HopID          string   `json:"hop_id"`
	Sub            string   `json:"sub"`
	Username       string   `json:"username"`
	Name           *string  `json:"name"`
	Email          string   `json:"email"`
	AvatarURL      *string  `json:"avatar_url"`
	Bio            *string  `json:"bio"`
	Website        *string  `json:"website"`
	ShowOwner      *bool    `json:"show_owner"`
	Roles          []string `json:"roles"`
	Status         string   `json:"status"`
	ProfileVersion int      `json:"profile_version"`
}

func (p *hopProfilePush) id() string {
	if p.HopID != "" {
		return p.HopID
	}
	return p.Sub
}

// applyProfilePush updates the local account linked to a pushed profile.
func (a *App) applyProfilePush(ctx context.Context, p *hopProfilePush) (*db.User, error) {
	u, err := a.db.UserByHelink(ctx, p.id())
	if err != nil {
		return nil, err
	}
	if p.Name != nil && strings.TrimSpace(*p.Name) != "" {
		u.Name = trim(strings.TrimSpace(*p.Name), 60)
	}
	if e := strings.TrimSpace(p.Email); e != "" && !a.db.EmailTaken(ctx, e, u.ID) {
		u.Email = e
	}
	if p.AvatarURL != nil {
		u.AvatarURL = trim(*p.AvatarURL, 500)
	}
	a.applyHopProfile(ctx, u, p.Username, p.Bio, p.Website, p.ShowOwner)
	if role, ok := roleFromHop(p.Roles); ok {
		u.Role = role
	}
	if err := a.db.UpdateUser(ctx, u); err != nil {
		return nil, err
	}
	switch st := hopStatus(p.Status); {
	case st != "" && st != u.Status:
		if err := a.setUserStatus(ctx, u, st, u.StatusNote); err != nil {
			return nil, err
		}
	}
	return u, nil
}

// hopStatus maps HOP ID account states onto the three local ones.
func hopStatus(s string) string {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "active":
		return "active"
	case "locked":
		return "locked"
	case "suspended", "banned", "deleted":
		return "suspended"
	}
	return ""
}

func (a *App) hopProfile(w http.ResponseWriter, r *http.Request) {
	var p hopProfilePush
	if !hopJSON(w, r, &p) {
		return
	}
	if p.id() == "" {
		hopFail(w, http.StatusBadRequest, "bad_request", "hop_id is needed.")
		return
	}
	u, err := a.applyProfilePush(r.Context(), &p)
	if errors.Is(err, db.ErrNotFound) {
		hopFail(w, http.StatusNotFound, "not_found", "No account on this app is linked to that HOP id.")
		return
	}
	if err != nil {
		hopFail(w, http.StatusInternalServerError, "db_error", err.Error())
		return
	}
	a.audit(r, "profile.updated", u.HelinkID.String, map[string]any{"profile_version": p.ProfileVersion, "role": u.Role, "status": u.Status})
	hopOK(w, a.hopUserJSON(r.Context(), u))
}

// ---- events from HOP ID ----

type hopEvent struct {
	ID   string          `json:"id"`
	Type string          `json:"type"`
	At   string          `json:"at"`
	Data json.RawMessage `json:"data"`
}

func (a *App) hopEvent(w http.ResponseWriter, r *http.Request) {
	var ev hopEvent
	if !hopJSON(w, r, &ev) {
		return
	}
	if ev.ID == "" || ev.Type == "" {
		hopFail(w, http.StatusBadRequest, "bad_request", "An event needs an id and a type.")
		return
	}
	ctx := r.Context()
	if !a.db.FirstEvent(ctx, trim(ev.ID, 200)) {
		hopOK(w, map[string]any{"handled": false, "duplicate": true})
		return
	}
	var data struct {
		hopProfilePush
		User string `json:"user"`
	}
	if len(ev.Data) > 0 {
		if err := json.Unmarshal(ev.Data, &data); err != nil {
			hopFail(w, http.StatusBadRequest, "bad_json", "data is not an object: "+err.Error())
			return
		}
	}
	id := data.id()
	if id == "" {
		id = data.User
	}
	handled := false
	switch ev.Type {
	case "profile.updated":
		data.HopID = id
		u, err := a.applyProfilePush(ctx, &data.hopProfilePush)
		if err != nil && !errors.Is(err, db.ErrNotFound) {
			hopFail(w, http.StatusInternalServerError, "db_error", err.Error())
			return
		}
		if handled = err == nil; handled {
			a.audit(r, "event.profile.updated", id, map[string]any{"event": ev.ID, "role": u.Role, "status": u.Status})
		}
	case "account.status":
		u, err := a.db.UserByHelink(ctx, id)
		if st := hopStatus(data.Status); err == nil && st != "" {
			if err := a.setUserStatus(ctx, u, st, u.StatusNote); err != nil {
				hopFail(w, http.StatusInternalServerError, "db_error", err.Error())
				return
			}
			handled = true
			a.audit(r, "event.account.status", id, map[string]any{"event": ev.ID, "status": st})
		}
	case "account.deleted":
		if u, err := a.db.UserByHelink(ctx, id); err == nil {
			counts := map[string]any{"images": a.purgeUser(ctx, u)}
			handled = true
			a.audit(r, "event.account.deleted", id, map[string]any{"event": ev.ID, "counts": counts})
			hopOK(w, map[string]any{"handled": true, "purged": true, "counts": counts})
			return
		}
	case "session.revoked":
		if u, err := a.db.UserByHelink(ctx, id); err == nil {
			a.db.DeleteUserSessions(ctx, u.ID)
			handled = true
			a.audit(r, "event.session.revoked", id, map[string]any{"event": ev.ID})
		}
	case "app.secret_rotated":
		// The secret lives in the environment; an operator must update it.
	}
	hopOK(w, map[string]any{"handled": handled})
}

// ---- content ----

func (a *App) hopOwner(ctx context.Context, userID sql.NullInt64) map[string]any {
	if !userID.Valid {
		return nil
	}
	u, err := a.db.UserByID(ctx, userID.Int64)
	if err != nil {
		return nil
	}
	return map[string]any{"sub": u.HelinkID.String, "local_id": u.ID, "username": u.Username.String, "name": u.Name, "status": u.Status}
}

func (a *App) hopImageRow(ctx context.Context, im *db.Image) map[string]any {
	status := "visible"
	switch {
	case im.Hidden:
		status = "hidden"
	case im.Locked():
		status = "private"
	}
	var flags []string
	for f, on := range map[string]bool{"hidden": im.Hidden, "private": im.Private, "password": im.PassHash != "",
		"listed": im.Listed, "expires": im.ExpiresAt > 0} {
		if on {
			flags = append(flags, f)
		}
	}
	if im.IsAdult() {
		flags = append(flags, "adult", "nsfw")
	} else if im.IsSensitive() {
		flags = append(flags, "sensitive")
	}
	if flags == nil {
		flags = []string{}
	}
	title := im.Title
	if title == "" {
		title = im.OrigName
	}
	preview := ""
	if im.IsImage() || im.KeyThumb != "" {
		preview = a.thumbURL(im)
	}
	return map[string]any{
		"id": im.ID, "type": "image", "kind": im.Kind(), "title": title, "owner": a.hopOwner(ctx, im.UserID),
		"status": status, "hidden_reason": im.HiddenReason, "created": isoTime(im.CreatedAt), "size": im.Size,
		"url": a.pageURL(im.ID), "direct_url": a.directURL(im), "flags": flags, "preview": preview,
		"ip": im.IP, "mime": im.Mime, "width": im.Width, "height": im.Height, "views": im.Views,
		"expires": isoTime(im.ExpiresAt), "album_id": im.AlbumID.String,
		"content_warning": im.ContentWarning, "warning_reason": im.WarningReason,
	}
}

func (a *App) hopAlbumRow(ctx context.Context, al db.HopAlbum) map[string]any {
	var flags []string
	if al.IsAdult() {
		flags = append(flags, "adult", "nsfw")
	} else if al.IsSensitive() {
		flags = append(flags, "sensitive")
	}
	if flags == nil {
		flags = []string{}
	}
	return map[string]any{
		"id": al.ID, "type": "album", "title": al.Title, "owner": a.hopOwner(ctx, al.UserID), "status": "visible",
		"created": isoTime(al.CreatedAt), "size": al.Bytes, "count": al.Count, "url": a.albumURL(al.ID),
		"direct_url": "", "flags": flags, "preview": "", "ip": "", "description": al.Description,
		"content_warning": al.ContentWarning, "warning_reason": al.WarningReason,
	}
}

func (a *App) hopContent(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	q := r.URL.Query()
	limit, offset := hopCursor(r, 50, 200)
	cq := db.ContentQuery{Search: q.Get("q"), Status: q.Get("status"), Limit: limit, Offset: offset}
	if owner := strings.TrimSpace(q.Get("owner")); owner != "" {
		u, err := a.db.UserByHelink(ctx, owner)
		if err != nil {
			hopPage(w, []any{}, "", 0)
			return
		}
		cq.UserID = u.ID
	}
	switch q.Get("type") {
	case "", "image":
		ims, total, err := a.db.HopImages(ctx, cq)
		if err != nil {
			hopFail(w, http.StatusInternalServerError, "db_error", err.Error())
			return
		}
		rows := make([]map[string]any, 0, len(ims))
		for _, im := range ims {
			rows = append(rows, a.hopImageRow(ctx, im))
		}
		hopPage(w, rows, nextCursor(offset, len(rows), total), total)
	case "album":
		als, total, err := a.db.HopAlbums(ctx, cq)
		if err != nil {
			hopFail(w, http.StatusInternalServerError, "db_error", err.Error())
			return
		}
		rows := make([]map[string]any, 0, len(als))
		for _, al := range als {
			rows = append(rows, a.hopAlbumRow(ctx, al))
		}
		hopPage(w, rows, nextCursor(offset, len(rows), total), total)
	default:
		hopFail(w, http.StatusBadRequest, "bad_type", "type must be image or album.")
	}
}

func (a *App) hopContentDetail(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id := r.PathValue("id")
	switch r.PathValue("type") {
	case "image":
		im, err := a.db.Image(ctx, id)
		if err != nil {
			hopFail(w, http.StatusNotFound, "not_found", "No such image.")
			return
		}
		row := a.hopImageRow(ctx, im)
		reps, _ := a.db.ImageReports(ctx, im.ID)
		list := make([]map[string]any, 0, len(reps))
		for _, rp := range reps {
			list = append(list, map[string]any{"id": rp.ID, "reason": rp.Reason, "note": rp.Note, "ip": rp.IP, "status": rp.Status, "created": isoTime(rp.CreatedAt)})
		}
		days, hosts := a.db.ImageStats(ctx, im.ID)
		if days == nil {
			days = []db.DayViews{}
		}
		if hosts == nil {
			hosts = []db.HostViews{}
		}
		row["reports"] = list
		row["views_by_day"] = days
		row["referrers"] = hosts
		row["alt"] = im.Alt
		row["orig_name"] = im.OrigName
		row["sha256"] = im.SHA256
		row["backend"] = im.Backend
		row["stripped"] = im.Stripped
		row["last_viewed"] = isoTime(im.LastViewed)
		row["actions"] = []string{"hide", "unhide", "delete", "ban_ip", "transfer", "mark_nsfw", "unmark_nsfw", "mark_sensitive"}
		hopOK(w, row)
	case "album":
		al, err := a.db.Album(ctx, id)
		if err != nil {
			hopFail(w, http.StatusNotFound, "not_found", "No such album.")
			return
		}
		ims, total, _ := a.db.Images(ctx, db.ImageQuery{AlbumID: al.ID, Sort: "album", Limit: 200})
		var bytes int64
		list := make([]map[string]any, 0, len(ims))
		for _, im := range ims {
			bytes += im.Size
			list = append(list, a.hopImageRow(ctx, im))
		}
		row := a.hopAlbumRow(ctx, db.HopAlbum{Album: *al, Count: total, Bytes: bytes})
		row["images"] = list
		row["actions"] = []string{"delete", "transfer", "mark_nsfw", "unmark_nsfw", "mark_sensitive"}
		hopOK(w, row)
	default:
		hopFail(w, http.StatusBadRequest, "bad_type", "type must be image or album.")
	}
}

func (a *App) hopContentAction(w http.ResponseWriter, r *http.Request) {
	var q hopActionRequest
	if !hopJSON(w, r, &q) {
		return
	}
	ctx := r.Context()
	typ, id := r.PathValue("type"), r.PathValue("id")
	detail := map[string]any{"reason": trim(strings.TrimSpace(q.Reason), 300), "type": typ}
	var err error
	switch typ {
	case "image":
		im, e := a.db.Image(ctx, id)
		if e != nil {
			hopFail(w, http.StatusNotFound, "not_found", "No such image.")
			return
		}
		switch q.Action {
		case "hide":
			err = a.db.SetImageHidden(ctx, im.ID, true)
		case "unhide":
			err = a.db.SetImageHidden(ctx, im.ID, false)
		case "mark_nsfw":
			err = a.db.SetImageWarning(ctx, im.ID, "adult", q.Reason)
		case "unmark_nsfw":
			err = a.db.SetImageWarning(ctx, im.ID, "none", "")
		case "mark_sensitive":
			err = a.db.SetImageWarning(ctx, im.ID, "sensitive", q.Reason)
		case "delete", "takedown":
			err = a.removeImage(ctx, im)
			a.db.DeleteEmptyAlbums(ctx)
		case "ban_ip":
			if im.IP == "" {
				hopFail(w, http.StatusConflict, "no_ip", "This image has no uploader address recorded.")
				return
			}
			if err = a.db.Ban(ctx, im.IP, "uploaded "+im.ID); err == nil {
				a.invalidateBans()
				detail["ip"] = im.IP
				err = a.removeImage(ctx, im)
				a.db.DeleteEmptyAlbums(ctx)
			}
		case "transfer":
			to := a.hopFindUser(w, r, q.str("to"))
			if to == nil {
				return
			}
			detail["to"] = to.HelinkID.String
			err = a.db.SetImageOwner(ctx, im.ID, to.ID)
		default:
			hopFail(w, http.StatusBadRequest, "unknown_action", "Unknown image action "+strconv.Quote(q.Action)+".")
			return
		}
	case "album":
		al, e := a.db.Album(ctx, id)
		if e != nil {
			hopFail(w, http.StatusNotFound, "not_found", "No such album.")
			return
		}
		switch q.Action {
		case "mark_nsfw":
			err = a.db.SetAlbumWarning(ctx, al.ID, "adult", q.Reason)
		case "unmark_nsfw":
			err = a.db.SetAlbumWarning(ctx, al.ID, "none", "")
		case "mark_sensitive":
			err = a.db.SetAlbumWarning(ctx, al.ID, "sensitive", q.Reason)
		case "delete", "takedown":
			ims, _, _ := a.db.Images(ctx, db.ImageQuery{AlbumID: al.ID, Limit: 200})
			for _, im := range ims {
				if err = a.removeImage(ctx, im); err != nil {
					break
				}
			}
			if err == nil {
				err = a.db.DeleteAlbum(ctx, al.ID)
			}
		case "transfer":
			to := a.hopFindUser(w, r, q.str("to"))
			if to == nil {
				return
			}
			detail["to"] = to.HelinkID.String
			err = a.db.SetAlbumOwner(ctx, al.ID, to.ID)
		default:
			hopFail(w, http.StatusBadRequest, "unknown_action", "Unknown album action "+strconv.Quote(q.Action)+".")
			return
		}
	default:
		hopFail(w, http.StatusBadRequest, "bad_type", "type must be image or album.")
		return
	}
	if err != nil {
		hopFail(w, http.StatusInternalServerError, "db_error", err.Error())
		return
	}
	a.audit(r, "content."+q.Action, typ+":"+id, detail)
	hopOK(w, map[string]any{"id": id, "type": typ, "action": q.Action, "done": true})
}

// ---- IP bans (extension "ip") ----

// GET /hop/x/ip: the banned addresses, newest first.
func (a *App) hopIPBans(w http.ResponseWriter, r *http.Request) {
	bans, err := a.db.Bans(r.Context())
	if err != nil {
		hopFail(w, http.StatusInternalServerError, "db_error", err.Error())
		return
	}
	rows := make([]map[string]any, 0, len(bans))
	for _, b := range bans {
		rows = append(rows, map[string]any{"ip": b.IP, "reason": b.Reason, "created_at": isoTime(b.CreatedAt)})
	}
	hopOK(w, map[string]any{"bans": rows})
}

// POST /hop/x/ip/actions {action: ban|unban, reason, params: {ip, reason}}
func (a *App) hopIPBanAction(w http.ResponseWriter, r *http.Request) {
	var q hopActionRequest
	if !hopJSON(w, r, &q) {
		return
	}
	ip, err := netip.ParseAddr(strings.TrimSpace(q.str("ip")))
	if err != nil {
		hopFail(w, http.StatusBadRequest, "bad_params", "params.ip must be an IP address.")
		return
	}
	reason := trim(strings.TrimSpace(q.str("reason")), 300)
	if reason == "" {
		reason = trim(strings.TrimSpace(q.Reason), 300)
	}
	switch q.Action {
	case "ban":
		err = a.db.Ban(r.Context(), ip.String(), reason)
	case "unban":
		err = a.db.Unban(r.Context(), ip.String())
	default:
		hopFail(w, http.StatusBadRequest, "unknown_action", "ban or unban.")
		return
	}
	if err != nil {
		hopFail(w, http.StatusInternalServerError, "db_error", err.Error())
		return
	}
	a.invalidateBans()
	a.audit(r, "ip."+q.Action, "ip:"+ip.String(), map[string]any{"reason": reason})
	hopOK(w, map[string]any{"ip": ip.String(), "action": q.Action, "done": true})
}

// ---- reports ----

func (a *App) hopReportRow(ctx context.Context, rp db.ReportGroup) map[string]any {
	content := map[string]any{"type": "image", "id": rp.ImageID, "url": a.pageURL(rp.ImageID), "preview": ""}
	if im, err := a.db.Image(ctx, rp.ImageID); err == nil {
		content["preview"] = a.thumbURL(im)
		content["title"] = im.Title
		content["hidden"] = im.Hidden
	}
	return map[string]any{"id": rp.ID, "content": content, "reason": rp.Reason, "note": rp.Note, "ip": rp.IP,
		"status": rp.Status, "created": isoTime(rp.CreatedAt), "count": rp.Count}
}

func (a *App) hopReports(w http.ResponseWriter, r *http.Request) {
	status := r.URL.Query().Get("status")
	if status != "dismissed" && status != "actioned" {
		status = "open"
	}
	limit, offset := hopCursor(r, 50, 200)
	groups, total, err := a.db.ReportGroups(r.Context(), status, limit, offset)
	if err != nil {
		hopFail(w, http.StatusInternalServerError, "db_error", err.Error())
		return
	}
	rows := make([]map[string]any, 0, len(groups))
	for _, g := range groups {
		rows = append(rows, a.hopReportRow(r.Context(), g))
	}
	hopPage(w, rows, nextCursor(offset, len(rows), total), total)
}

func (a *App) hopReportAction(w http.ResponseWriter, r *http.Request) {
	var q hopActionRequest
	if !hopJSON(w, r, &q) {
		return
	}
	ctx := r.Context()
	id, _ := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if _, err := a.db.Report(ctx, id); err != nil {
		hopFail(w, http.StatusNotFound, "not_found", "No such report.")
		return
	}
	status := "dismissed"
	switch q.Action {
	case "dismiss":
	case "resolve", "delete":
		status = "actioned"
	case "escalate":
		hopOK(w, map[string]any{"id": id, "action": q.Action, "done": false, "message": "not implemented"})
		return
	default:
		hopFail(w, http.StatusBadRequest, "unknown_action", "Unknown report action "+strconv.Quote(q.Action)+".")
		return
	}
	imageID, err := a.db.SetReportStatus(ctx, id, status)
	if err != nil {
		hopFail(w, http.StatusInternalServerError, "db_error", err.Error())
		return
	}
	if im, err := a.db.Image(ctx, imageID); err == nil {
		switch q.Action {
		case "resolve":
			a.db.SetImageHidden(ctx, im.ID, true)
		case "delete":
			a.removeImage(ctx, im)
			a.db.DeleteEmptyAlbums(ctx)
		}
	}
	a.audit(r, "report."+q.Action, strconv.FormatInt(id, 10), map[string]any{"reason": trim(q.Reason, 300), "image": imageID})
	hopOK(w, map[string]any{"id": id, "action": q.Action, "status": status, "image": imageID, "done": true})
}

// ---- settings ----

func (a *App) hopSettingRows(ctx context.Context) []map[string]any {
	defaults := map[string]any{"guest_uploads": a.cfg.GuestUploads, "registration": a.cfg.Registration,
		"announcement": "", "seo_title": "", "seo_description": ""}
	var out []map[string]any
	for _, s := range hopSettingSchema {
		key := s["key"].(string)
		row := map[string]any{"readonly": false}
		for k, v := range s {
			row[k] = v
		}
		row["default"] = defaults[key]
		raw := a.db.Setting(ctx, key, fmt.Sprint(defaults[key]))
		if s["type"] == "bool" {
			row["value"] = raw == "true"
		} else {
			row["value"] = raw
		}
		out = append(out, row)
	}
	for _, f := range []struct {
		key  string
		typ  string
		val  any
		desc string
	}{
		{"max_upload_mb", "int", a.cfg.MaxUploadBytes >> 20, "Largest picture, in MB (PIXELHOP_MAX_UPLOAD_MB)"},
		{"max_media_mb", "int", a.cfg.MaxMediaBytes >> 20, "Largest video or PDF for members, in MB (PIXELHOP_MAX_MEDIA_MB)"},
		{"guest_max_mb", "int", a.cfg.GuestMaxBytes >> 20, "Largest file for guests, in MB (PIXELHOP_GUEST_MAX_MB)"},
		{"max_batch", "int", a.cfg.MaxBatch, "Files per upload request (PIXELHOP_MAX_BATCH)"},
		{"default_quota_mb", "int", a.cfg.DefaultQuota >> 20, "Storage for new accounts, in MB (PIXELHOP_DEFAULT_QUOTA_MB)"},
		{"inactive_days", "int", a.cfg.InactiveDays, "Delete files nobody viewed for N days, 0 = never (PIXELHOP_INACTIVE_DAYS)"},
		{"uploads_per_minute", "int", a.cfg.UploadsPerMinute, "Upload rate limit per address (PIXELHOP_UPLOADS_PER_MINUTE)"},
		{"storage", "string", a.store.Name(), "Where new uploads go (PIXELHOP_STORAGE)"},
		{"google_sign_in", "bool", a.cfg.GoogleEnabled(), "Sign in with Google configured"},
		{"turnstile", "bool", a.cfg.TurnstileEnabled(), "Cloudflare Turnstile configured"},
		{"ai_tools", "bool", a.cfg.RembgURL != "", "Server AI tools configured (PIXELHOP_REMBG_URL)"},
	} {
		out = append(out, map[string]any{"key": f.key, "type": f.typ, "value": f.val, "default": f.val, "description": f.desc,
			"restart_required": true, "readonly": true})
	}
	return out
}

func (a *App) hopSettings(w http.ResponseWriter, r *http.Request) {
	hopOK(w, a.hopSettingRows(r.Context()))
}

func (a *App) hopPutSettings(w http.ResponseWriter, r *http.Request) {
	var in map[string]any
	if !hopJSON(w, r, &in) {
		return
	}
	ctx := r.Context()
	limits := map[string]int{"announcement": 300, "seo_title": 120, "seo_description": 300}
	changed := map[string]any{}
	for key, v := range in {
		var typ string
		for _, s := range hopSettingSchema {
			if s["key"] == key {
				typ = s["type"].(string)
			}
		}
		var val string
		switch {
		case typ == "":
			hopFail(w, http.StatusBadRequest, "unknown_setting", "No editable setting named "+strconv.Quote(key)+".")
			return
		case typ == "bool":
			switch b := v.(type) {
			case bool:
				val = fmt.Sprint(b)
			case string:
				p, err := strconv.ParseBool(strings.TrimSpace(b))
				if err != nil {
					hopFail(w, http.StatusBadRequest, "bad_value", key+" must be true or false.")
					return
				}
				val = fmt.Sprint(p)
			default:
				hopFail(w, http.StatusBadRequest, "bad_value", key+" must be true or false.")
				return
			}
		default:
			s, ok := v.(string)
			if !ok {
				if v == nil {
					s = ""
				} else {
					hopFail(w, http.StatusBadRequest, "bad_value", key+" must be a string.")
					return
				}
			}
			val = trim(strings.TrimSpace(s), limits[key])
		}
		if err := a.db.SetSetting(ctx, key, val); err != nil {
			hopFail(w, http.StatusInternalServerError, "db_error", err.Error())
			return
		}
		changed[key] = val
	}
	if len(changed) > 0 {
		a.audit(r, "settings.update", "settings", changed)
	}
	hopOK(w, map[string]any{"settings": a.hopSettingRows(ctx), "restart_required": false})
}

// ---- search, audit ----

func (a *App) hopSearch(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	out := []map[string]any{}
	if q == "" {
		hopOK(w, out)
		return
	}
	users, _ := a.db.SearchUsers(ctx, q, 10)
	for _, u := range users {
		label := u.Email
		if u.Username.Valid {
			label = "@" + u.Username.String + " · " + u.Email
		}
		link := ""
		if u.Username.Valid {
			link = a.cfg.BaseURL + "/u/" + u.Username.String
		}
		out = append(out, map[string]any{"kind": "user", "id": u.HelinkID.String, "local_id": u.ID, "label": label,
			"url": link, "status": u.Status, "linked": u.HelinkID.Valid})
	}
	ims, _, _ := a.db.HopImages(ctx, db.ContentQuery{Search: q, Limit: 10})
	for _, im := range ims {
		label := im.ID
		if im.Title != "" {
			label = im.Title + " (" + im.ID + ")"
		}
		out = append(out, map[string]any{"kind": "content", "type": "image", "id": im.ID, "label": label, "url": a.pageURL(im.ID)})
	}
	als, _, _ := a.db.HopAlbums(ctx, db.ContentQuery{Search: q, Limit: 10})
	for _, al := range als {
		label := al.ID
		if al.Title != "" {
			label = al.Title + " (" + al.ID + ")"
		}
		out = append(out, map[string]any{"kind": "content", "type": "album", "id": al.ID, "label": label, "url": a.albumURL(al.ID)})
	}
	hopOK(w, out)
}

func (a *App) hopAudit(w http.ResponseWriter, r *http.Request) {
	limit, offset := hopCursor(r, 50, 200)
	var since int64
	if s := strings.TrimSpace(r.URL.Query().Get("since")); s != "" {
		if t, err := time.Parse(time.RFC3339, s); err == nil {
			since = t.Unix()
		} else if t, err := time.Parse("2006-01-02", s); err == nil {
			since = t.Unix()
		} else if n, err := strconv.ParseInt(s, 10, 64); err == nil {
			since = n
		}
	}
	entries, total, err := a.db.Audit(r.Context(), since, limit, offset)
	if err != nil {
		hopFail(w, http.StatusInternalServerError, "db_error", err.Error())
		return
	}
	rows := make([]map[string]any, 0, len(entries))
	for _, e := range entries {
		rows = append(rows, map[string]any{"id": e.ID, "at": isoTime(e.At), "actor": e.Actor, "app": e.App,
			"action": e.Action, "target": e.Target, "detail": e.Detail})
	}
	hopPage(w, rows, nextCursor(offset, len(rows), total), total)
}

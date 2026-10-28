package app

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/navi-crwn/pixelhop/internal/db"
	"github.com/navi-crwn/pixelhop/internal/hop"
)

type hopResp struct {
	OK      bool            `json:"ok"`
	Error   string          `json:"error"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
	Page    *struct {
		Next  string `json:"next"`
		Total int    `json:"total"`
	} `json:"page"`
}

// hopCall makes a signed call as app (default "imperium") and decodes the envelope.
func hopCall(t *testing.T, h http.Handler, app, method, path string, body any) (*httptest.ResponseRecorder, hopResp) {
	t.Helper()
	var raw []byte
	if body != nil {
		raw, _ = json.Marshal(body)
	}
	req := httptest.NewRequest(method, path, bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	for k, v := range hop.Headers(app, testSecret, "admin-1", method, path, raw) {
		req.Header[k] = v
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	var out hopResp
	json.Unmarshal(rec.Body.Bytes(), &out)
	return rec, out
}

func hopGet(t *testing.T, h http.Handler, path string) hopResp {
	t.Helper()
	rec, out := hopCall(t, h, "imperium", "GET", path, nil)
	if rec.Code != 200 || !out.OK {
		t.Fatalf("GET %s: %d %s", path, rec.Code, rec.Body.String())
	}
	return out
}

func hopPost(t *testing.T, h http.Handler, path string, body any) hopResp {
	t.Helper()
	rec, out := hopCall(t, h, "imperium", "POST", path, body)
	if rec.Code != 200 || !out.OK {
		t.Fatalf("POST %s: %d %s", path, rec.Code, rec.Body.String())
	}
	return out
}

func decode(t *testing.T, raw json.RawMessage, v any) {
	t.Helper()
	if err := json.Unmarshal(raw, v); err != nil {
		t.Fatalf("decode %s: %v", raw, err)
	}
}

// linked makes a HOP-linked member with a few images.
func linked(t *testing.T, a *App, sub, email string) *db.User {
	t.Helper()
	ctx := context.Background()
	u := &db.User{Email: email, Name: "Linked " + sub, QuotaBytes: 1 << 30, HelinkID: sql.NullString{String: sub, Valid: true}}
	if err := a.db.InsertUser(ctx, u); err != nil {
		t.Fatal(err)
	}
	u.Username = sql.NullString{String: "user" + sub, Valid: true}
	a.db.UpdateUser(ctx, u)
	return u
}

func addImage(t *testing.T, a *App, u *db.User, id string) *db.Image {
	t.Helper()
	im := &db.Image{ID: id, Title: "Pic " + id, Mime: "image/jpeg", Ext: "jpg", Size: 100, Width: 10, Height: 10,
		SHA256: id, Backend: "local", KeyFull: id + ".jpg", KeyThumb: id + "_t.jpg", IP: "203.0.113.9"}
	if u != nil {
		im.UserID = sql.NullInt64{Int64: u.ID, Valid: true}
	}
	if err := a.db.InsertImage(context.Background(), im); err != nil {
		t.Fatal(err)
	}
	return im
}

func TestHopSignatureGating(t *testing.T) {
	a, h := ssoApp(t)
	_ = a
	path := "/hop/health"
	// Unsigned.
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", path, nil))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("unsigned: %d", rec.Code)
	}
	// Unknown app.
	if rec, out := hopCall(t, h, "stranger", "GET", path, nil); rec.Code != 401 || out.Error != "bad_signature" {
		t.Fatalf("other app: %d %s", rec.Code, rec.Body.String())
	}
	// Wrong secret.
	req := httptest.NewRequest("GET", path, nil)
	for k, v := range hop.Headers("imperium", "another-secret-that-is-long-enough-000000", "x", "GET", path, nil) {
		req.Header[k] = v
	}
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != 401 || !bytes.Contains(rec.Body.Bytes(), []byte("bad_signature")) {
		t.Fatalf("bad secret: %d %s", rec.Code, rec.Body.String())
	}
	// Stale timestamp.
	req = httptest.NewRequest("GET", path, nil)
	for k, v := range hop.Headers("imperium", testSecret, "x", "GET", path, nil) {
		req.Header[k] = v
	}
	req.Header.Set("X-HOP-Timestamp", strconv.FormatInt(time.Now().Add(-10*time.Minute).Unix(), 10))
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != 401 || !bytes.Contains(rec.Body.Bytes(), []byte("stale_request")) {
		t.Fatalf("stale: %d %s", rec.Code, rec.Body.String())
	}
	// Path (with query) is part of the signature.
	req = httptest.NewRequest("GET", path+"?x=1", nil)
	for k, v := range hop.Headers("imperium", testSecret, "x", "GET", path, nil) {
		req.Header[k] = v
	}
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != 401 {
		t.Fatalf("query tamper: %d", rec.Code)
	}
	// Replay: the same headers twice.
	req = httptest.NewRequest("GET", path, nil)
	hdr := hop.Headers("imperium", testSecret, "x", "GET", path, nil)
	for k, v := range hdr {
		req.Header[k] = v
	}
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("first use: %d %s", rec.Code, rec.Body.String())
	}
	req = httptest.NewRequest("GET", path, nil)
	for k, v := range hdr {
		req.Header[k] = v
	}
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != 401 || !bytes.Contains(rec.Body.Bytes(), []byte("replay")) {
		t.Fatalf("replay: %d %s", rec.Code, rec.Body.String())
	}
	// helink is accepted too; a signed POST needs no CSRF headers.
	if rec, out := hopCall(t, h, "helink", "POST", "/hop/events", map[string]any{"id": "e1", "type": "noop"}); rec.Code != 200 || !out.OK {
		t.Fatalf("helink: %d %s", rec.Code, rec.Body.String())
	}
	// Unknown endpoint under /hop/ answers in the envelope.
	if rec, out := hopCall(t, h, "imperium", "GET", "/hop/nothing", nil); rec.Code != 404 || out.Error != "not_found" {
		t.Fatalf("unknown: %d %s", rec.Code, rec.Body.String())
	}
}

func TestHopNotConfigured(t *testing.T) {
	t.Setenv("HELINK_URL", "")
	t.Setenv("HELINK_SSO_SECRET", "")
	a, h := ssoApp(t)
	a.cfg.HelinkURL, a.cfg.HelinkSecret = "", ""
	h = a.Handler()
	rec, out := hopCall(t, h, "imperium", "GET", "/hop/health", nil)
	if rec.Code != 404 || out.Error != "not_configured" {
		t.Fatalf("%d %s", rec.Code, rec.Body.String())
	}
}

func TestHopHealthManifestStats(t *testing.T) {
	a, h := ssoApp(t)
	var health struct {
		App, Version, DB, Storage, SSO string
		UptimeS                        int64 `json:"uptime_s"`
		Checks                         []struct{ Name string }
	}
	decode(t, hopGet(t, h, "/hop/health").Data, &health)
	if health.App != "pixelhop" || health.Version != Version || health.DB != "ok" || health.Storage != "ok" || health.SSO != "ok" || len(health.Checks) != 3 {
		t.Fatalf("health: %+v", health)
	}
	var man struct {
		App            string `json:"app"`
		Base           string `json:"base_url"`
		Capabilities   []string
		ContentTypes   []struct{ Type string } `json:"content_types"`
		SettingsSchema []struct{ Key string }  `json:"settings_schema"`
	}
	decode(t, hopGet(t, h, "/hop/manifest").Data, &man)
	if man.App != "pixelhop" || man.Base != "https://p.example.com" || len(man.Capabilities) != 4 || len(man.ContentTypes) != 2 || len(man.SettingsSchema) != 5 {
		t.Fatalf("manifest: %+v", man)
	}

	u := linked(t, a, "7", "seven@example.com")
	addImage(t, a, u, "one")
	addImage(t, a, u, "two")
	var stats struct {
		KPI    map[string]int64
		Series map[string][][2]any
	}
	decode(t, hopGet(t, h, "/hop/stats?step=day").Data, &stats)
	if stats.KPI["users"] != 1 || stats.KPI["images"] != 2 || stats.KPI["bytes"] != 200 || stats.KPI["uploads_today"] != 2 || stats.KPI["guest_uploads"] != 1 {
		t.Fatalf("kpi: %v", stats.KPI)
	}
	if len(stats.Series["uploads"]) != 14 || len(stats.Series["bytes"]) != 14 {
		t.Fatalf("series: %v", stats.Series)
	}
	if last := stats.Series["uploads"][13]; last[1].(float64) != 2 {
		t.Fatalf("today's point: %v", last)
	}
}

func TestHopUserAndActions(t *testing.T) {
	a, h := ssoApp(t)
	ctx := context.Background()
	u := linked(t, a, "42", "ivan@example.com")
	keep := addImage(t, a, u, "keep")
	bad := addImage(t, a, u, "bad")
	a.db.SetImageHidden(ctx, bad.ID, true) // removed by moderation before the suspension
	key := "ph_" + randomToken(24)
	a.db.InsertAPIKey(ctx, u.ID, "k", key[:9], hashToken(key))
	a.db.InsertSession(ctx, hashToken(randomToken(32)), u.ID, time.Hour, "", "")

	if rec, out := hopCall(t, h, "imperium", "GET", "/hop/users/nobody", nil); rec.Code != 404 || out.Error != "not_found" {
		t.Fatalf("unknown user: %d %s", rec.Code, rec.Body.String())
	}
	var sum struct {
		Sub      string
		LocalID  int64 `json:"local_id"`
		Email    string
		Status   string
		Images   int
		APIKeys  int `json:"api_keys"`
		Sessions int
		Linked   bool   `json:"helink_linked"`
		Created  string `json:"created_at"`
		Quota    int64  `json:"quota_bytes"`
		Warning  string
		Role     string
	}
	decode(t, hopGet(t, h, "/hop/users/42").Data, &sum)
	if sum.Sub != "42" || sum.LocalID != u.ID || sum.Images != 2 || sum.APIKeys != 1 || sum.Sessions != 1 || !sum.Linked || sum.Created == "" {
		t.Fatalf("summary: %+v", sum)
	}
	act := func(action string, params map[string]any) hopResp {
		return hopPost(t, h, "/hop/users/42/actions", map[string]any{"action": action, "reason": "test " + action, "params": params})
	}

	act("suspend", nil)
	decode(t, hopGet(t, h, "/hop/users/42").Data, &sum)
	if sum.Status != "suspended" || sum.Sessions != 0 {
		t.Fatalf("after suspend: %+v", sum)
	}
	if im, _ := a.db.Image(ctx, keep.ID); !im.Hidden || im.HiddenReason != "suspended" {
		t.Fatalf("keep not hidden by suspension: %+v", im)
	}
	if im, _ := a.db.Image(ctx, bad.ID); !im.Hidden || im.HiddenReason != "moderation" {
		t.Fatalf("moderation reason lost: %+v", im)
	}

	act("activate", nil)
	if im, _ := a.db.Image(ctx, keep.ID); im.Hidden {
		t.Fatal("keep should be visible again")
	}
	if im, _ := a.db.Image(ctx, bad.ID); !im.Hidden || im.HiddenReason != "moderation" {
		t.Fatal("activate must not bring back what moderation removed")
	}
	decode(t, hopGet(t, h, "/hop/users/42").Data, &sum)
	if sum.Status != "active" {
		t.Fatalf("after activate: %+v", sum)
	}

	a.db.InsertSession(ctx, hashToken(randomToken(32)), u.ID, time.Hour, "", "")
	act("lock", nil)
	decode(t, hopGet(t, h, "/hop/users/42").Data, &sum)
	if sum.Status != "locked" || sum.Sessions != 0 {
		t.Fatalf("after lock: %+v", sum)
	}
	if im, _ := a.db.Image(ctx, keep.ID); im.Hidden {
		t.Fatal("lock must not hide images")
	}
	act("activate", nil)

	a.db.InsertSession(ctx, hashToken(randomToken(32)), u.ID, time.Hour, "", "")
	act("revoke_sessions", nil)
	if n := a.db.CountSessions(ctx, u.ID); n != 0 {
		t.Fatalf("sessions left: %d", n)
	}
	act("revoke_keys", nil)
	if n := a.db.CountAPIKeys(ctx, u.ID); n != 0 {
		t.Fatalf("keys left: %d", n)
	}
	act("set_quota", map[string]any{"quota_bytes": 5 << 30})
	act("warn", map[string]any{"text": "Please stop."})
	decode(t, hopGet(t, h, "/hop/users/42").Data, &sum)
	if sum.Quota != 5<<30 || sum.Warning != "Please stop." {
		t.Fatalf("quota/warn: %+v", sum)
	}
	act("warn", map[string]any{"text": ""})
	decode(t, hopGet(t, h, "/hop/users/42").Data, &sum)
	if sum.Warning != "" {
		t.Fatal("warning not cleared")
	}
	if rec, out := hopCall(t, h, "imperium", "POST", "/hop/users/42/actions", map[string]any{"action": "set_quota", "params": map[string]any{"quota_bytes": "lots"}}); rec.Code != 400 || out.Error != "bad_params" {
		t.Fatalf("bad quota: %d %s", rec.Code, rec.Body.String())
	}
	if rec, out := hopCall(t, h, "imperium", "POST", "/hop/users/42/actions", map[string]any{"action": "fly"}); rec.Code != 400 || out.Error != "unknown_action" {
		t.Fatalf("unknown action: %d %s", rec.Code, rec.Body.String())
	}
	var exp struct {
		URL     *string
		Message string
	}
	decode(t, act("export", nil).Data, &exp)
	if exp.URL != nil || exp.Message != "not implemented" {
		t.Fatalf("export: %+v", exp)
	}

	var purged struct {
		Purged bool
		Counts map[string]int
	}
	decode(t, act("purge", nil).Data, &purged)
	if !purged.Purged || purged.Counts["images"] != 2 {
		t.Fatalf("purge: %+v", purged)
	}
	if _, err := a.db.UserByHelink(ctx, "42"); err == nil {
		t.Fatal("user still there after purge")
	}
	if _, err := a.db.Image(ctx, keep.ID); err == nil {
		t.Fatal("image still there after purge")
	}

	// Everything that changed something was recorded.
	var audit []struct{ Action, Target, Actor, App string }
	out := hopGet(t, h, "/hop/audit")
	decode(t, out.Data, &audit)
	want := map[string]bool{"user.suspend": true, "user.activate": true, "user.lock": true, "user.revoke_sessions": true,
		"user.revoke_keys": true, "user.set_quota": true, "user.warn": true, "user.purge": true}
	for _, e := range audit {
		if e.Target != "42" || e.Actor != "admin-1" || e.App != "imperium" {
			t.Fatalf("audit row: %+v", e)
		}
		delete(want, e.Action)
	}
	if len(want) != 0 || out.Page == nil || out.Page.Total != len(audit) {
		t.Fatalf("audit missing %v (page %+v)", want, out.Page)
	}
}

func TestHopProfilePush(t *testing.T) {
	a, h := ssoApp(t)
	ctx := context.Background()
	u := linked(t, a, "9", "nine@example.com")
	other := linked(t, a, "10", "ten@example.com")

	push := func(body map[string]any) hopResp { return hopPost(t, h, "/hop/profile", body) }
	var sum struct {
		Role, Status, Username, Name, Email, Bio, Website string
		ShowOwner                                         bool `json:"show_owner"`
	}
	// Promotion with a profile.
	decode(t, push(map[string]any{"hop_id": "9", "username": "Nina", "name": "Nina N", "bio": " hi ", "website": "https://nina.example",
		"show_owner": false, "roles": []string{"admin:pixelhop"}, "status": "active", "profile_version": 3}).Data, &sum)
	if sum.Role != "admin" || sum.Username != "nina" || sum.Name != "Nina N" || sum.Bio != "hi" || sum.Website != "https://nina.example" || sum.ShowOwner {
		t.Fatalf("after push: %+v", sum)
	}
	// Demotion, and a username someone else has is not taken over.
	decode(t, push(map[string]any{"sub": "9", "username": other.Username.String, "roles": []string{"moderator"}}).Data, &sum)
	if sum.Role != "user" || sum.Username != "nina" {
		t.Fatalf("after demotion: %+v", sum)
	}
	// Owner role promotes; suspension via status hides images; activate restores.
	im := addImage(t, a, u, "nine-pic")
	decode(t, push(map[string]any{"hop_id": "9", "roles": []string{"owner"}, "status": "suspended"}).Data, &sum)
	if sum.Role != "admin" || sum.Status != "suspended" {
		t.Fatalf("owner+suspend: %+v", sum)
	}
	if got, _ := a.db.Image(ctx, im.ID); !got.Hidden {
		t.Fatal("suspension did not hide the image")
	}
	push(map[string]any{"hop_id": "9", "status": "active"})
	if got, _ := a.db.Image(ctx, im.ID); got.Hidden {
		t.Fatal("activation did not restore the image")
	}
	// No roles at all: the role is left alone.
	decode(t, push(map[string]any{"hop_id": "9", "name": "Still Nina"}).Data, &sum)
	if sum.Role != "admin" || sum.Name != "Still Nina" {
		t.Fatalf("no roles: %+v", sum)
	}
	// Email changes follow HOP ID, but never collide with another account.
	decode(t, push(map[string]any{"hop_id": "9", "email": other.Email}).Data, &sum)
	if sum.Email != "nine@example.com" {
		t.Fatalf("email collided: %+v", sum)
	}
	decode(t, push(map[string]any{"hop_id": "9", "email": "nina@example.com"}).Data, &sum)
	if sum.Email != "nina@example.com" {
		t.Fatalf("email not updated: %+v", sum)
	}
	// Nothing is created for an unknown id.
	if rec, out := hopCall(t, h, "helink", "POST", "/hop/profile", map[string]any{"hop_id": "404", "username": "ghost"}); rec.Code != 404 || out.Error != "not_found" {
		t.Fatalf("unknown: %d %s", rec.Code, rec.Body.String())
	}
	if a.db.CountUsers(ctx) != 2 {
		t.Fatal("a profile push created an account")
	}
}

func TestHopEvents(t *testing.T) {
	a, h := ssoApp(t)
	ctx := context.Background()
	u := linked(t, a, "5", "five@example.com")
	im := addImage(t, a, u, "five-pic")
	a.db.InsertSession(ctx, hashToken(randomToken(32)), u.ID, time.Hour, "", "")

	send := func(id, typ string, data map[string]any) (bool, hopResp) {
		rec, out := hopCall(t, h, "helink", "POST", "/hop/events", map[string]any{"id": id, "type": typ, "at": time.Now().UTC().Format(time.RFC3339), "data": data})
		if rec.Code != 200 || !out.OK {
			t.Fatalf("event %s: %d %s", typ, rec.Code, rec.Body.String())
		}
		var d struct{ Handled bool }
		decode(t, out.Data, &d)
		return d.Handled, out
	}
	if ok, _ := send("ev-1", "session.revoked", map[string]any{"hop_id": "5"}); !ok || a.db.CountSessions(ctx, u.ID) != 0 {
		t.Fatal("session.revoked not handled")
	}
	// Same id again: idempotent, not handled twice.
	a.db.InsertSession(ctx, hashToken(randomToken(32)), u.ID, time.Hour, "", "")
	if ok, _ := send("ev-1", "session.revoked", map[string]any{"hop_id": "5"}); ok || a.db.CountSessions(ctx, u.ID) != 1 {
		t.Fatal("duplicate event handled again")
	}
	if ok, _ := send("ev-2", "profile.updated", map[string]any{"sub": "5", "roles": []string{"admin"}, "username": "fiver"}); !ok {
		t.Fatal("profile.updated not handled")
	}
	if got, _ := a.db.UserByID(ctx, u.ID); got.Role != "admin" || got.Username.String != "fiver" {
		t.Fatalf("profile.updated: %+v", got)
	}
	if ok, _ := send("ev-3", "account.status", map[string]any{"hop_id": "5", "status": "banned"}); !ok {
		t.Fatal("account.status not handled")
	}
	if got, _ := a.db.UserByID(ctx, u.ID); got.Status != "suspended" {
		t.Fatalf("status: %s", got.Status)
	}
	if got, _ := a.db.Image(ctx, im.ID); !got.Hidden {
		t.Fatal("banned account's image still visible")
	}
	// Unknown user: accepted, not handled.
	if ok, _ := send("ev-4", "account.status", map[string]any{"hop_id": "77", "status": "locked"}); ok {
		t.Fatal("unknown user handled")
	}
	ok, out := send("ev-5", "account.deleted", map[string]any{"hop_id": "5"})
	var del struct {
		Purged bool
		Counts map[string]int
	}
	decode(t, out.Data, &del)
	if !ok || !del.Purged || del.Counts["images"] != 1 {
		t.Fatalf("account.deleted: %s", out.Data)
	}
	if _, err := a.db.UserByHelink(ctx, "5"); err == nil {
		t.Fatal("account not purged")
	}
	if rec, out := hopCall(t, h, "helink", "POST", "/hop/events", map[string]any{"type": "x"}); rec.Code != 400 || out.Error != "bad_request" {
		t.Fatalf("missing id: %d", rec.Code)
	}
}

func TestHopContent(t *testing.T) {
	a, h := ssoApp(t)
	ctx := context.Background()
	u := linked(t, a, "1", "one@example.com")
	v := linked(t, a, "2", "two@example.com")
	p1 := addImage(t, a, u, "sunset-aaa")
	p2 := addImage(t, a, u, "beach-bbb")
	p3 := addImage(t, a, v, "cat-ccc")
	guest := addImage(t, a, nil, "guest-ddd")
	a.db.SetImageHidden(ctx, p2.ID, true)
	p3.Private = true
	a.db.UpdateImageMeta(ctx, p3)
	al := &db.Album{ID: "holiday-xyz", UserID: sql.NullInt64{Int64: u.ID, Valid: true}, Title: "Holiday"}
	a.db.InsertAlbum(ctx, al)
	p1.AlbumID = sql.NullString{String: al.ID, Valid: true}
	a.db.UpdateImageMeta(ctx, p1)
	p4 := addImage(t, a, u, "in-album-eee")
	p4.AlbumID = sql.NullString{String: al.ID, Valid: true}
	a.db.UpdateImageMeta(ctx, p4)

	type row struct {
		ID, Type, Title, Status, URL, Preview, IP, Created string
		DirectURL                                          string `json:"direct_url"`
		Owner                                              *struct{ Sub, Username string }
		Flags                                              []string
		Size                                               int64
	}
	list := func(q string) ([]row, hopResp) {
		out := hopGet(t, h, "/hop/content"+q)
		var rows []row
		decode(t, out.Data, &rows)
		return rows, out
	}
	rows, out := list("?type=image")
	if len(rows) != 5 || out.Page == nil || out.Page.Total != 5 {
		t.Fatalf("all: %d %+v", len(rows), out.Page)
	}
	for _, r := range rows {
		if r.Type != "image" || r.URL != "https://p.example.com/"+r.ID || r.DirectURL == "" || r.Preview == "" || r.Created == "" || r.IP != "203.0.113.9" {
			t.Fatalf("row: %+v", r)
		}
		if r.ID == guest.ID && r.Owner != nil {
			t.Fatal("guest image has an owner")
		}
		if r.ID == p1.ID && (r.Owner == nil || r.Owner.Sub != "1" || r.Owner.Username != "user1") {
			t.Fatalf("owner: %+v", r.Owner)
		}
	}
	if rows, _ = list("?type=image&owner=1"); len(rows) != 3 {
		t.Fatalf("owner filter: %d", len(rows))
	}
	if rows, _ = list("?type=image&owner=999"); len(rows) != 0 {
		t.Fatalf("unknown owner: %d", len(rows))
	}
	if rows, _ = list("?type=image&status=hidden"); len(rows) != 1 || rows[0].ID != p2.ID || rows[0].Status != "hidden" {
		t.Fatalf("hidden filter: %+v", rows)
	}
	if rows, _ = list("?type=image&status=private"); len(rows) != 1 || rows[0].ID != p3.ID || rows[0].Status != "private" {
		t.Fatalf("private filter: %+v", rows)
	}
	if rows, _ = list("?type=image&status=visible"); len(rows) != 3 {
		t.Fatalf("visible filter: %d", len(rows))
	}
	if rows, _ = list("?type=image&q=sunset"); len(rows) != 1 || rows[0].ID != p1.ID {
		t.Fatalf("search: %+v", rows)
	}
	rows, out = list("?type=image&limit=3")
	if len(rows) != 3 || out.Page.Next != "3" {
		t.Fatalf("page 1: %d next=%q", len(rows), out.Page.Next)
	}
	if rows, out = list("?type=image&limit=3&cursor=3"); len(rows) != 2 || out.Page.Next != "" {
		t.Fatalf("page 2: %d next=%q", len(rows), out.Page.Next)
	}
	if rows, _ = list("?type=album"); len(rows) != 1 || rows[0].ID != al.ID || rows[0].Title != "Holiday" || rows[0].Size != 200 {
		t.Fatalf("albums: %+v", rows)
	}
	if rec, out := hopCall(t, h, "imperium", "GET", "/hop/content?type=note", nil); rec.Code != 400 || out.Error != "bad_type" {
		t.Fatalf("bad type: %d", rec.Code)
	}

	// Detail.
	a.db.InsertReport(ctx, &db.Report{ImageID: p1.ID, Reason: "spam", IP: "198.51.100.1"})
	a.db.CountView(ctx, p1.ID, "example.org")
	var det struct {
		ID      string
		Reports []struct{ Reason string }
		Views   int64
		Actions []string
	}
	decode(t, hopGet(t, h, "/hop/content/image/"+p1.ID).Data, &det)
	if det.ID != p1.ID || len(det.Reports) != 1 || det.Reports[0].Reason != "spam" || det.Views != 1 || len(det.Actions) != 5 {
		t.Fatalf("detail: %+v", det)
	}
	var adet struct {
		ID     string
		Images []row
		Count  int
	}
	decode(t, hopGet(t, h, "/hop/content/album/"+al.ID).Data, &adet)
	if adet.ID != al.ID || len(adet.Images) != 2 || adet.Count != 2 {
		t.Fatalf("album detail: %+v", adet)
	}
	if rec, _ := hopCall(t, h, "imperium", "GET", "/hop/content/image/nope", nil); rec.Code != 404 {
		t.Fatalf("missing: %d", rec.Code)
	}

	// Actions.
	act := func(typ, id, action string, params map[string]any) {
		hopPost(t, h, "/hop/content/"+typ+"/"+id+"/actions", map[string]any{"action": action, "reason": "r", "params": params})
	}
	act("image", p1.ID, "hide", nil)
	if im, _ := a.db.Image(ctx, p1.ID); !im.Hidden || im.HiddenReason != "moderation" {
		t.Fatalf("hide: %+v", im)
	}
	act("image", p1.ID, "unhide", nil)
	if im, _ := a.db.Image(ctx, p1.ID); im.Hidden || im.HiddenReason != "" {
		t.Fatalf("unhide: %+v", im)
	}
	act("image", p1.ID, "transfer", map[string]any{"to": "2"})
	if im, _ := a.db.Image(ctx, p1.ID); im.UserID.Int64 != v.ID {
		t.Fatal("transfer failed")
	}
	if rec, out := hopCall(t, h, "imperium", "POST", "/hop/content/image/"+p1.ID+"/actions", map[string]any{"action": "transfer", "params": map[string]any{"to": "999"}}); rec.Code != 404 || out.Error != "not_found" {
		t.Fatalf("transfer to nobody: %d", rec.Code)
	}
	act("image", p2.ID, "delete", nil)
	if _, err := a.db.Image(ctx, p2.ID); err == nil {
		t.Fatal("delete failed")
	}
	act("image", guest.ID, "ban_ip", nil)
	if _, err := a.db.Image(ctx, guest.ID); err == nil {
		t.Fatal("ban_ip kept the image")
	}
	if bans, _ := a.db.Bans(ctx); len(bans) != 1 || bans[0].IP != "203.0.113.9" {
		t.Fatalf("ban missing: %+v", bans)
	}
	act("album", al.ID, "transfer", map[string]any{"to": "2"})
	if got, _ := a.db.Album(ctx, al.ID); got.UserID.Int64 != v.ID {
		t.Fatal("album transfer failed")
	}
	act("album", al.ID, "delete", nil)
	if _, err := a.db.Album(ctx, al.ID); err == nil {
		t.Fatal("album delete failed")
	}
	if _, err := a.db.Image(ctx, p4.ID); err == nil {
		t.Fatal("album delete kept its image")
	}
	if im, err := a.db.Image(ctx, p1.ID); err != nil || im.AlbumID.Valid {
		t.Fatal("a transferred image must leave the album and survive its deletion")
	}
	if rec, out := hopCall(t, h, "imperium", "POST", "/hop/content/image/"+p3.ID+"/actions", map[string]any{"action": "explode"}); rec.Code != 400 || out.Error != "unknown_action" {
		t.Fatalf("unknown action: %d", rec.Code)
	}
	var audit []struct{ Action, Target string }
	decode(t, hopGet(t, h, "/hop/audit").Data, &audit)
	if len(audit) != 7 || audit[0].Action != "content.delete" || audit[0].Target != "album:"+al.ID {
		t.Fatalf("audit: %+v", audit)
	}
}

func TestHopReports(t *testing.T) {
	a, h := ssoApp(t)
	ctx := context.Background()
	u := linked(t, a, "3", "three@example.com")
	p1 := addImage(t, a, u, "rep-one")
	p2 := addImage(t, a, u, "rep-two")
	p3 := addImage(t, a, u, "rep-three")
	for _, r := range []db.Report{{ImageID: p1.ID, Reason: "spam", IP: "1.1.1.1"}, {ImageID: p1.ID, Reason: "spam", IP: "2.2.2.2", Note: "again"},
		{ImageID: p2.ID, Reason: "abuse", IP: "3.3.3.3"}, {ImageID: p3.ID, Reason: "copyright", IP: "4.4.4.4"}} {
		a.db.InsertReport(ctx, &r)
	}
	type row struct {
		ID                      int64
		Content                 struct{ Type, ID, URL, Preview string }
		Reason, Status, Created string
		Count                   int
	}
	list := func(q string) []row {
		var rows []row
		decode(t, hopGet(t, h, "/hop/reports"+q).Data, &rows)
		return rows
	}
	rows := list("")
	if len(rows) != 3 {
		t.Fatalf("open: %+v", rows)
	}
	var byImage = map[string]row{}
	for _, r := range rows {
		if r.Content.Type != "image" || r.Content.URL == "" || r.Content.Preview == "" || r.Created == "" || r.Status != "open" {
			t.Fatalf("row: %+v", r)
		}
		byImage[r.Content.ID] = r
	}
	if byImage[p1.ID].Count != 2 || byImage[p2.ID].Count != 1 {
		t.Fatalf("counts: %+v", byImage)
	}
	act := func(id int64, action string) {
		hopPost(t, h, "/hop/reports/"+strconv.FormatInt(id, 10)+"/actions", map[string]any{"action": action, "reason": "r"})
	}
	act(byImage[p1.ID].ID, "dismiss")
	act(byImage[p2.ID].ID, "resolve")
	act(byImage[p3.ID].ID, "delete")
	if len(list("")) != 0 {
		t.Fatal("reports still open")
	}
	if d := list("?status=dismissed"); len(d) != 1 || d[0].Content.ID != p1.ID || d[0].Count != 2 {
		t.Fatalf("dismissed: %+v", d)
	}
	// Deleting the image takes its reports with it (as in the admin panel).
	if ac := list("?status=actioned"); len(ac) != 1 || ac[0].Content.ID != p2.ID {
		t.Fatalf("actioned: %+v", ac)
	}
	if im, _ := a.db.Image(ctx, p1.ID); im.Hidden {
		t.Fatal("dismiss hid the image")
	}
	if im, _ := a.db.Image(ctx, p2.ID); !im.Hidden {
		t.Fatal("resolve did not hide the image")
	}
	if _, err := a.db.Image(ctx, p3.ID); err == nil {
		t.Fatal("delete kept the image")
	}
	if rec, _ := hopCall(t, h, "imperium", "POST", "/hop/reports/999/actions", map[string]any{"action": "dismiss"}); rec.Code != 404 {
		t.Fatalf("missing report: %d", rec.Code)
	}
	if rec, out := hopCall(t, h, "imperium", "POST", "/hop/reports/"+strconv.FormatInt(byImage[p1.ID].ID, 10)+"/actions", map[string]any{"action": "nuke"}); rec.Code != 400 || out.Error != "unknown_action" {
		t.Fatalf("unknown: %d", rec.Code)
	}
}

func TestHopSettings(t *testing.T) {
	a, h := ssoApp(t)
	type row struct {
		Key, Type string
		Value     any `json:"value"`
		Readonly  bool
		Restart   bool `json:"restart_required"`
	}
	get := func() map[string]row {
		var rows []row
		decode(t, hopGet(t, h, "/hop/settings").Data, &rows)
		m := map[string]row{}
		for _, r := range rows {
			m[r.Key] = r
		}
		return m
	}
	m := get()
	if m["guest_uploads"].Value != true || m["guest_uploads"].Type != "bool" || m["guest_uploads"].Readonly || m["announcement"].Value != "" {
		t.Fatalf("defaults: %+v", m)
	}
	if ro := m["max_upload_mb"]; !ro.Readonly || ro.Value.(float64) != 20 || !ro.Restart {
		t.Fatalf("readonly: %+v", ro)
	}
	rec, out := hopCall(t, h, "imperium", "PUT", "/hop/settings", map[string]any{"guest_uploads": false, "registration": "false", "announcement": "  Maintenance tonight  ", "seo_title": "My pics"})
	if rec.Code != 200 || !out.OK {
		t.Fatalf("put: %d %s", rec.Code, rec.Body.String())
	}
	m = get()
	if m["guest_uploads"].Value != false || m["registration"].Value != false || m["announcement"].Value != "Maintenance tonight" || m["seo_title"].Value != "My pics" {
		t.Fatalf("after put: %+v", m)
	}
	if a.db.Setting(context.Background(), "guest_uploads", "") != "false" || a.registrationOpen(context.Background()) {
		t.Fatal("settings not stored")
	}
	if rec, out := hopCall(t, h, "imperium", "PUT", "/hop/settings", map[string]any{"max_upload_mb": 50}); rec.Code != 400 || out.Error != "unknown_setting" {
		t.Fatalf("readonly write: %d %s", rec.Code, rec.Body.String())
	}
	if rec, out := hopCall(t, h, "imperium", "PUT", "/hop/settings", map[string]any{"guest_uploads": "maybe"}); rec.Code != 400 || out.Error != "bad_value" {
		t.Fatalf("bad bool: %d %s", rec.Code, rec.Body.String())
	}
	var audit []struct {
		Action string
		Detail map[string]any
	}
	decode(t, hopGet(t, h, "/hop/audit").Data, &audit)
	if len(audit) != 1 || audit[0].Action != "settings.update" || audit[0].Detail["announcement"] != "Maintenance tonight" {
		t.Fatalf("audit: %+v", audit)
	}
}

func TestHopSearchAndMetrics(t *testing.T) {
	a, h := ssoApp(t)
	ctx := context.Background()
	u := linked(t, a, "8", "finder@example.com")
	addImage(t, a, u, "needle-pic")
	a.db.InsertAlbum(ctx, &db.Album{ID: "needle-album", UserID: sql.NullInt64{Int64: u.ID, Valid: true}, Title: "Needles"})
	type hit struct{ Kind, Type, ID, Label, URL string }
	search := func(q string) []hit {
		var hits []hit
		decode(t, hopGet(t, h, "/hop/search?q="+q).Data, &hits)
		return hits
	}
	hits := search("needle")
	if len(hits) != 2 || hits[0].Kind != "content" || hits[0].Type != "image" || hits[1].Type != "album" || hits[1].URL != "https://p.example.com/a/needle-album" {
		t.Fatalf("content hits: %+v", hits)
	}
	if hits = search("finder"); len(hits) != 1 || hits[0].Kind != "user" || hits[0].ID != "8" {
		t.Fatalf("user hit: %+v", hits)
	}
	if hits = search("user8"); len(hits) != 1 || hits[0].Kind != "user" {
		t.Fatalf("username hit: %+v", hits)
	}
	if hits = search(""); len(hits) != 0 {
		t.Fatalf("empty: %+v", hits)
	}

	rec, _ := hopCall(t, h, "imperium", "GET", "/hop/metrics", nil)
	if rec.Code != 200 || !bytes.Contains(rec.Body.Bytes(), []byte("pixelhop_images 1")) {
		t.Fatalf("metrics: %d %s", rec.Code, rec.Body.String())
	}
	// The plain /metrics stays loopback-only.
	req := httptest.NewRequest("GET", "/metrics", nil)
	req.RemoteAddr = "203.0.113.5:4444"
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != 404 {
		t.Fatalf("/metrics from outside: %d", rec.Code)
	}
}

// Ticket v2: roles promote and demote; profile fields come along.
func TestHelinkTicketV2Roles(t *testing.T) {
	a, h := ssoApp(t)
	ctx := context.Background()
	u := &db.User{Email: "ivan@example.com", Name: "Ivan", Role: "admin", QuotaBytes: 1 << 20, HelinkID: sql.NullString{String: "42", Valid: true}}
	a.db.InsertUser(ctx, u)
	// A second account so "first user is admin" never applies.
	a.db.InsertUser(ctx, &db.User{Email: "other@example.com", QuotaBytes: 1})

	signIn := func(mod func(*helinkTicket)) {
		t.Helper()
		c, state := start(t, h)
		tk := ticketFor(state)
		mod(&tk)
		if rec := callback(h, c, signTicket(tk, testSecret)); rec.Code != http.StatusSeeOther {
			t.Fatalf("callback %d %s", rec.Code, rec.Body.String())
		}
	}
	no := false
	signIn(func(tk *helinkTicket) {
		tk.V, tk.Aud, tk.Admin, tk.Roles, tk.Username = 2, "pixelhop", true, []string{"moderator"}, "ivan.v2"
		bio := "from HOP ID"
		tk.Bio, tk.ShowOwner = &bio, &no
	})
	got, _ := a.db.UserByID(ctx, u.ID)
	if got.Role != "user" || got.Username.String != "ivan.v2" || got.Bio != "from HOP ID" || got.ShowOwner {
		t.Fatalf("demotion by roles: role=%s username=%s bio=%q show=%v", got.Role, got.Username.String, got.Bio, got.ShowOwner)
	}
	signIn(func(tk *helinkTicket) { tk.V, tk.Aud, tk.Roles = 2, "pixelhop", []string{"owner"} })
	if got, _ = a.db.UserByID(ctx, u.ID); got.Role != "admin" {
		t.Fatalf("promotion by roles: %s", got.Role)
	}
	// Empty roles list demotes; absent roles keep the v1 behaviour.
	signIn(func(tk *helinkTicket) { tk.V, tk.Aud, tk.Roles = 2, "pixelhop", []string{} })
	if got, _ = a.db.UserByID(ctx, u.ID); got.Role != "user" {
		t.Fatalf("empty roles: %s", got.Role)
	}
	signIn(func(tk *helinkTicket) { tk.Admin = true })
	if got, _ = a.db.UserByID(ctx, u.ID); got.Role != "admin" {
		t.Fatalf("v1 admin flag: %s", got.Role)
	}
	signIn(func(tk *helinkTicket) {})
	if got, _ = a.db.UserByID(ctx, u.ID); got.Role != "admin" || got.Username.String != "ivan.v2" {
		t.Fatalf("v1 without admin must not demote: %s %s", got.Role, got.Username.String)
	}
	// A ticket for another app is refused.
	c, state := start(t, h)
	tk := ticketFor(state)
	tk.V, tk.Aud = 2, "notehop"
	if rec := callback(h, c, signTicket(tk, testSecret)); rec.Code != http.StatusBadRequest {
		t.Fatalf("wrong aud accepted: %d", rec.Code)
	}
}

// The admin panel's status change uses the same reason bookkeeping.
func TestAdminStatusKeepsModerationHidden(t *testing.T) {
	a, _ := ssoApp(t)
	ctx := context.Background()
	u := linked(t, a, "11", "eleven@example.com")
	ok := addImage(t, a, u, "fine")
	bad := addImage(t, a, u, "nasty")
	a.db.SetImageHidden(ctx, bad.ID, true)
	if err := a.setUserStatus(ctx, u, "suspended", "spam"); err != nil {
		t.Fatal(err)
	}
	if err := a.setUserStatus(ctx, u, "active", ""); err != nil {
		t.Fatal(err)
	}
	if im, _ := a.db.Image(ctx, ok.ID); im.Hidden {
		t.Fatal("fine should be back")
	}
	if im, _ := a.db.Image(ctx, bad.ID); !im.Hidden {
		t.Fatal("nasty should stay hidden")
	}
}

// The "ip" extension lists the banned addresses and bans/unbans one, so
// Imperium can take over the retired admin "IP bans" page.
func TestHopIPBans(t *testing.T) {
	a, h := ssoApp(t)
	ctx := context.Background()
	var man struct {
		Extensions []string `json:"extensions"`
	}
	decode(t, hopGet(t, h, "/hop/manifest").Data, &man)
	if len(man.Extensions) != 1 || man.Extensions[0] != "ip" {
		t.Fatalf("manifest extensions: %v", man.Extensions)
	}
	type ban struct {
		IP, Reason string
		CreatedAt  string `json:"created_at"`
	}
	list := func() []ban {
		var out struct{ Bans []ban }
		decode(t, hopGet(t, h, "/hop/x/ip").Data, &out)
		return out.Bans
	}
	if len(list()) != 0 {
		t.Fatal("bans should start empty")
	}
	hopPost(t, h, "/hop/x/ip/actions", map[string]any{"action": "ban", "reason": "spam burst", "params": map[string]any{"ip": " 203.0.113.9 "}})
	bans := list()
	if len(bans) != 1 || bans[0].IP != "203.0.113.9" || bans[0].Reason != "spam burst" || bans[0].CreatedAt == "" {
		t.Fatalf("after ban: %+v", bans)
	}
	if all, _ := a.db.Bans(ctx); len(all) != 1 {
		t.Fatalf("db bans: %+v", all)
	}
	if rec, out := hopCall(t, h, "imperium", "POST", "/hop/x/ip/actions", map[string]any{"action": "ban", "params": map[string]any{"ip": "not-an-ip"}}); rec.Code != 400 || out.Error != "bad_params" {
		t.Fatalf("bad ip: %d %s", rec.Code, out.Error)
	}
	if rec, out := hopCall(t, h, "imperium", "POST", "/hop/x/ip/actions", map[string]any{"action": "nuke", "params": map[string]any{"ip": "203.0.113.9"}}); rec.Code != 400 || out.Error != "unknown_action" {
		t.Fatalf("unknown action: %d %s", rec.Code, out.Error)
	}
	hopPost(t, h, "/hop/x/ip/actions", map[string]any{"action": "unban", "params": map[string]any{"ip": "203.0.113.9"}})
	if len(list()) != 0 {
		t.Fatal("unban left the address banned")
	}
	var audit []struct{ Action, Target string }
	decode(t, hopGet(t, h, "/hop/audit").Data, &audit)
	var seen []string
	for _, e := range audit {
		seen = append(seen, e.Action+" "+e.Target)
	}
	for _, want := range []string{"ip.ban ip:203.0.113.9", "ip.unban ip:203.0.113.9"} {
		found := false
		for _, s := range seen {
			found = found || s == want
		}
		if !found {
			t.Errorf("audit missing %q in %v", want, seen)
		}
	}
	var stats struct {
		KPI map[string]float64 `json:"kpi"`
	}
	decode(t, hopGet(t, h, "/hop/stats").Data, &stats)
	if _, ok := stats.KPI["disk_total_bytes"]; !ok {
		t.Errorf("stats should report the disk: %v", stats.KPI)
	}
}

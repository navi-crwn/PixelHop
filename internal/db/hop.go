package db

import (
	"context"
	"encoding/json"
	"strings"
)

// Queries behind the HOP App Protocol endpoints (/hop/*): the audit trail,
// webhook idempotency, content listings with filters and a few counts.

// AuditEntry is one admin action recorded by the signed admin API.
type AuditEntry struct {
	ID     int64           `json:"id"`
	At     int64           `json:"-"`
	Actor  string          `json:"actor"`
	App    string          `json:"app"`
	Action string          `json:"action"`
	Target string          `json:"target"`
	Detail json.RawMessage `json:"detail"`
}

func (d *DB) InsertAudit(ctx context.Context, e *AuditEntry) error {
	if len(e.Detail) == 0 {
		e.Detail = json.RawMessage("{}")
	}
	e.At = now()
	res, err := d.ExecContext(ctx, `INSERT INTO hop_audit (at,actor,app,action,target,detail) VALUES (?,?,?,?,?,?)`,
		e.At, e.Actor, e.App, e.Action, e.Target, string(e.Detail))
	if err == nil {
		e.ID, _ = res.LastInsertId()
	}
	return err
}

// Audit lists recorded actions, newest first, from since (unix, 0 = all).
func (d *DB) Audit(ctx context.Context, since int64, limit, offset int) ([]AuditEntry, int, error) {
	var total int
	d.QueryRowContext(ctx, `SELECT COUNT(*) FROM hop_audit WHERE at>=?`, since).Scan(&total)
	rows, err := d.QueryContext(ctx, `SELECT id,at,actor,app,action,target,detail FROM hop_audit WHERE at>=?
		ORDER BY id DESC LIMIT ? OFFSET ?`, since, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var out []AuditEntry
	for rows.Next() {
		var e AuditEntry
		var detail string
		if err := rows.Scan(&e.ID, &e.At, &e.Actor, &e.App, &e.Action, &e.Target, &detail); err != nil {
			return nil, 0, err
		}
		e.Detail = json.RawMessage(detail)
		if !json.Valid(e.Detail) {
			e.Detail = json.RawMessage("{}")
		}
		out = append(out, e)
	}
	return out, total, rows.Err()
}

// FirstEvent remembers a webhook id and reports whether it is new, so a
// retried delivery is handled once.
func (d *DB) FirstEvent(ctx context.Context, id string) bool {
	res, err := d.ExecContext(ctx, `INSERT OR IGNORE INTO hop_events_seen (id,at) VALUES (?,?)`, id, now())
	if err != nil {
		return false
	}
	n, _ := res.RowsAffected()
	return n == 1
}

// ---- per-user counts for the user summary ----

func (d *DB) CountSessions(ctx context.Context, userID int64) (n int) {
	d.QueryRowContext(ctx, `SELECT COUNT(*) FROM sessions WHERE user_id=? AND expires_at>?`, userID, now()).Scan(&n)
	return
}

func (d *DB) CountAPIKeys(ctx context.Context, userID int64) (n int) {
	d.QueryRowContext(ctx, `SELECT COUNT(*) FROM api_keys WHERE user_id=?`, userID).Scan(&n)
	return
}

func (d *DB) CountWebhooks(ctx context.Context, userID int64) (n int) {
	d.QueryRowContext(ctx, `SELECT COUNT(*) FROM webhooks WHERE user_id=?`, userID).Scan(&n)
	return
}

func (d *DB) DeleteUserAPIKeys(ctx context.Context, userID int64) (n int64) {
	res, err := d.ExecContext(ctx, `DELETE FROM api_keys WHERE user_id=?`, userID)
	if err == nil {
		n, _ = res.RowsAffected()
	}
	return
}

// ---- site-wide counts ----

// KindCounts reports how many files are pictures, videos and PDFs.
func (d *DB) KindCounts(ctx context.Context) (images, videos, pdfs int) {
	d.QueryRowContext(ctx, `SELECT
		COALESCE(SUM(mime NOT LIKE 'video/%' AND mime<>'application/pdf'),0),
		COALESCE(SUM(mime LIKE 'video/%'),0),
		COALESCE(SUM(mime='application/pdf'),0) FROM images`).Scan(&images, &videos, &pdfs)
	return
}

// ---- content listings ----

// ContentQuery filters the generic content list of the admin API.
type ContentQuery struct {
	UserID int64  // 0 = any
	Search string // id or title
	Status string // "" | hidden | visible | private | expiring
	Limit  int
	Offset int
}

func likeOf(s string) string {
	return "%" + strings.NewReplacer("%", `\%`, "_", `\_`).Replace(strings.TrimSpace(s)) + "%"
}

// HopImages lists images for the admin API, newest first.
func (d *DB) HopImages(ctx context.Context, q ContentQuery) ([]*Image, int, error) {
	where, args := []string{"1=1"}, []any{}
	if q.UserID > 0 {
		where = append(where, "user_id=?")
		args = append(args, q.UserID)
	}
	if strings.TrimSpace(q.Search) != "" {
		where = append(where, "(id LIKE ? ESCAPE '\\' OR title LIKE ? ESCAPE '\\' OR orig_name LIKE ? ESCAPE '\\')")
		l := likeOf(q.Search)
		args = append(args, l, l, l)
	}
	switch q.Status {
	case "hidden":
		where = append(where, "hidden=1")
	case "visible":
		where = append(where, "hidden=0 AND private=0 AND pass_hash=''")
	case "private":
		where = append(where, "(private=1 OR pass_hash<>'')")
	case "expiring":
		where = append(where, "expires_at>0")
	}
	w := " WHERE " + strings.Join(where, " AND ")
	var total int
	if err := d.QueryRowContext(ctx, `SELECT COUNT(*) FROM images`+w, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := d.QueryContext(ctx, `SELECT `+imageCols+` FROM images`+w+` ORDER BY created_at DESC, id LIMIT ? OFFSET ?`,
		append(args, q.Limit, q.Offset)...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var out []*Image
	for rows.Next() {
		im, err := scanImage(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, im)
	}
	return out, total, rows.Err()
}

// HopAlbum is an album row of the admin API, with how many images it holds.
type HopAlbum struct {
	Album
	Count int
	Bytes int64
}

// HopAlbums lists albums for the admin API, newest first.
func (d *DB) HopAlbums(ctx context.Context, q ContentQuery) ([]HopAlbum, int, error) {
	where, args := []string{"1=1"}, []any{}
	if q.UserID > 0 {
		where = append(where, "a.user_id=?")
		args = append(args, q.UserID)
	}
	if strings.TrimSpace(q.Search) != "" {
		where = append(where, "(a.id LIKE ? ESCAPE '\\' OR a.title LIKE ? ESCAPE '\\')")
		l := likeOf(q.Search)
		args = append(args, l, l)
	}
	w := " WHERE " + strings.Join(where, " AND ")
	var total int
	if err := d.QueryRowContext(ctx, `SELECT COUNT(*) FROM albums a`+w, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := d.QueryContext(ctx, `SELECT a.id,a.user_id,a.title,a.owner_token,a.created_at,a.cover_id,a.description,
		COUNT(i.id), COALESCE(SUM(i.size),0) FROM albums a LEFT JOIN images i ON i.album_id=a.id`+w+`
		GROUP BY a.id ORDER BY a.created_at DESC, a.id LIMIT ? OFFSET ?`, append(args, q.Limit, q.Offset)...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var out []HopAlbum
	for rows.Next() {
		var a HopAlbum
		if err := rows.Scan(&a.ID, &a.UserID, &a.Title, &a.OwnerToken, &a.CreatedAt, &a.CoverID, &a.Description, &a.Count, &a.Bytes); err != nil {
			return nil, 0, err
		}
		out = append(out, a)
	}
	return out, total, rows.Err()
}

// ImageReports lists every report filed against one image, newest first.
func (d *DB) ImageReports(ctx context.Context, imageID string) ([]Report, error) {
	rows, err := d.QueryContext(ctx, `SELECT id,image_id,reason,note,ip,status,created_at FROM reports WHERE image_id=? ORDER BY id DESC LIMIT 100`, imageID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Report
	for rows.Next() {
		var r Report
		if err := rows.Scan(&r.ID, &r.ImageID, &r.Reason, &r.Note, &r.IP, &r.Status, &r.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// ReportGroup is one reported image: its newest report and how many there are.
type ReportGroup struct {
	Report
	Count int
}

// ReportGroups lists reports with a status, one row per image, newest first.
func (d *DB) ReportGroups(ctx context.Context, status string, limit, offset int) ([]ReportGroup, int, error) {
	var total int
	d.QueryRowContext(ctx, `SELECT COUNT(DISTINCT image_id) FROM reports WHERE status=?`, status).Scan(&total)
	rows, err := d.QueryContext(ctx, `SELECT MAX(id),image_id,reason,note,ip,status,MAX(created_at),COUNT(*) FROM reports
		WHERE status=? GROUP BY image_id ORDER BY MAX(id) DESC LIMIT ? OFFSET ?`, status, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var out []ReportGroup
	for rows.Next() {
		var r ReportGroup
		if err := rows.Scan(&r.ID, &r.ImageID, &r.Reason, &r.Note, &r.IP, &r.Status, &r.CreatedAt, &r.Count); err != nil {
			return nil, 0, err
		}
		out = append(out, r)
	}
	return out, total, rows.Err()
}

// Report fetches one report by id.
func (d *DB) Report(ctx context.Context, id int64) (*Report, error) {
	var r Report
	err := d.QueryRowContext(ctx, `SELECT id,image_id,reason,note,ip,status,created_at FROM reports WHERE id=?`, id).
		Scan(&r.ID, &r.ImageID, &r.Reason, &r.Note, &r.IP, &r.Status, &r.CreatedAt)
	if err != nil {
		return nil, ErrNotFound
	}
	return &r, nil
}

// SearchUsers finds accounts by email, name or username.
func (d *DB) SearchUsers(ctx context.Context, q string, limit int) ([]*User, error) {
	l := likeOf(q)
	rows, err := d.QueryContext(ctx, `SELECT `+userCols+` FROM users WHERE email LIKE ? ESCAPE '\' OR name LIKE ? ESCAPE '\'
		OR username LIKE ? ESCAPE '\' OR helink_id=? ORDER BY id DESC LIMIT ?`, l, l, l, strings.TrimSpace(q), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*User
	for rows.Next() {
		u, err := scanUser(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, rows.Err()
}

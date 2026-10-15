package db

import "context"

type Report struct {
	ID        int64
	ImageID   string
	Reason    string
	Note      string
	IP        string
	Status    string
	CreatedAt int64
}

func (d *DB) InsertReport(ctx context.Context, r *Report) error {
	_, err := d.ExecContext(ctx, `INSERT INTO reports (image_id,reason,note,ip,created_at) VALUES (?,?,?,?,?)`,
		r.ImageID, r.Reason, r.Note, r.IP, now())
	return err
}

// RecentReport reports whether this IP already reported this image, to keep
// one person from flooding the queue.
func (d *DB) RecentReport(ctx context.Context, imageID, ip string) bool {
	var n int
	d.QueryRowContext(ctx, `SELECT COUNT(*) FROM reports WHERE image_id=? AND ip=?`, imageID, ip).Scan(&n)
	return n > 0
}

func (d *DB) Reports(ctx context.Context, status string) ([]Report, error) {
	rows, err := d.QueryContext(ctx, `SELECT id,image_id,reason,note,ip,status,created_at FROM reports
		WHERE status=? ORDER BY id DESC LIMIT 200`, status)
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

func (d *DB) SetReportStatus(ctx context.Context, id int64, status string) (imageID string, err error) {
	if err = d.QueryRowContext(ctx, `SELECT image_id FROM reports WHERE id=?`, id).Scan(&imageID); err != nil {
		return
	}
	_, err = d.ExecContext(ctx, `UPDATE reports SET status=? WHERE image_id=? AND status='open'`, status, imageID)
	return
}

// ---- IP bans ----

func (d *DB) Ban(ctx context.Context, ip, reason string) error {
	_, err := d.ExecContext(ctx, `INSERT INTO bans (ip,reason,created_at) VALUES (?,?,?)
		ON CONFLICT(ip) DO UPDATE SET reason=excluded.reason`, ip, reason, now())
	return err
}

func (d *DB) Unban(ctx context.Context, ip string) error {
	_, err := d.ExecContext(ctx, `DELETE FROM bans WHERE ip=?`, ip)
	return err
}

type Ban struct {
	IP, Reason string
	CreatedAt  int64
}

func (d *DB) Bans(ctx context.Context) ([]Ban, error) {
	rows, err := d.QueryContext(ctx, `SELECT ip,reason,created_at FROM bans ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Ban
	for rows.Next() {
		var b Ban
		if err := rows.Scan(&b.IP, &b.Reason, &b.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

// ---- stats ----

type Stats struct {
	Images, Users, Albums, OpenReports, Bans int
	Bytes, Views                             int64
	Daily                                    []DayCount // last 30 days of uploads
}

type DayCount struct {
	Day   string
	Count int
	Bytes int64
}

func (d *DB) Stats(ctx context.Context) Stats {
	var s Stats
	d.QueryRowContext(ctx, `SELECT COUNT(*), COALESCE(SUM(size),0), COALESCE(SUM(views),0) FROM images`).Scan(&s.Images, &s.Bytes, &s.Views)
	d.QueryRowContext(ctx, `SELECT COUNT(*) FROM users`).Scan(&s.Users)
	d.QueryRowContext(ctx, `SELECT COUNT(*) FROM albums`).Scan(&s.Albums)
	d.QueryRowContext(ctx, `SELECT COUNT(DISTINCT image_id) FROM reports WHERE status='open'`).Scan(&s.OpenReports)
	d.QueryRowContext(ctx, `SELECT COUNT(*) FROM bans`).Scan(&s.Bans)
	rows, err := d.QueryContext(ctx, `SELECT date(created_at,'unixepoch') AS day, COUNT(*), SUM(size) FROM images
		WHERE created_at > ? GROUP BY day ORDER BY day`, now()-30*86400)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var dc DayCount
			rows.Scan(&dc.Day, &dc.Count, &dc.Bytes)
			s.Daily = append(s.Daily, dc)
		}
	}
	return s
}

// PublicStats is the small set of numbers shown on the home page.
func (d *DB) PublicStats(ctx context.Context) (images int, views int64) {
	d.QueryRowContext(ctx, `SELECT COUNT(*), COALESCE(SUM(views),0) FROM images WHERE hidden=0`).Scan(&images, &views)
	return
}

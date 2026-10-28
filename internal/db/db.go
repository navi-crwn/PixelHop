// Package db owns PixelHop's SQLite database: schema, migrations and queries.
// SQLite runs in-process (pure Go, no CGO), so there is no database server to run.
package db

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

type DB struct{ *sql.DB }

// Open opens (or creates) the database in dir and applies pending migrations.
func Open(dir string) (*DB, error) {
	dsn := "file:" + filepath.Join(dir, "pixelhop.db") +
		"?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)&_pragma=synchronous(NORMAL)"
	sqlDB, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	// One writer at a time is what SQLite wants; WAL lets readers run alongside.
	sqlDB.SetMaxOpenConns(8)
	d := &DB{sqlDB}
	if err := d.migrate(); err != nil {
		return nil, fmt.Errorf("migrate: %w", err)
	}
	return d, nil
}

var migrations = []string{
	// 1: initial schema
	`
CREATE TABLE users (
  id           INTEGER PRIMARY KEY,
  email        TEXT NOT NULL UNIQUE COLLATE NOCASE,
  name         TEXT NOT NULL DEFAULT '',
  pass_hash    TEXT NOT NULL DEFAULT '',
  google_id    TEXT UNIQUE,
  avatar_url   TEXT NOT NULL DEFAULT '',
  role         TEXT NOT NULL DEFAULT 'user',      -- user | admin
  status       TEXT NOT NULL DEFAULT 'active',    -- active | locked | suspended
  status_note  TEXT NOT NULL DEFAULT '',
  quota_bytes  INTEGER NOT NULL,
  created_at   INTEGER NOT NULL,
  last_login   INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE sessions (
  token_hash TEXT PRIMARY KEY,
  user_id    INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  created_at INTEGER NOT NULL,
  expires_at INTEGER NOT NULL,
  ip         TEXT NOT NULL DEFAULT '',
  user_agent TEXT NOT NULL DEFAULT ''
);
CREATE INDEX sessions_user ON sessions(user_id);

CREATE TABLE api_keys (
  id         INTEGER PRIMARY KEY,
  user_id    INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  name       TEXT NOT NULL,
  prefix     TEXT NOT NULL,
  key_hash   TEXT NOT NULL UNIQUE,
  created_at INTEGER NOT NULL,
  last_used  INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE albums (
  id          TEXT PRIMARY KEY,
  user_id     INTEGER REFERENCES users(id) ON DELETE CASCADE,
  title       TEXT NOT NULL DEFAULT '',
  owner_token TEXT NOT NULL DEFAULT '',
  created_at  INTEGER NOT NULL
);

CREATE TABLE images (
  id          TEXT PRIMARY KEY,
  user_id     INTEGER REFERENCES users(id) ON DELETE SET NULL,
  album_id    TEXT REFERENCES albums(id) ON DELETE SET NULL,
  owner_token TEXT NOT NULL DEFAULT '',    -- sha256 of the guest delete token
  title       TEXT NOT NULL DEFAULT '',
  orig_name   TEXT NOT NULL DEFAULT '',
  mime        TEXT NOT NULL,
  ext         TEXT NOT NULL,
  size        INTEGER NOT NULL,
  width       INTEGER NOT NULL,
  height      INTEGER NOT NULL,
  sha256      TEXT NOT NULL,
  color       TEXT NOT NULL DEFAULT '',     -- dominant color, used as placeholder
  backend     TEXT NOT NULL,               -- local | s3 | remote
  key_full    TEXT NOT NULL,
  key_thumb   TEXT NOT NULL,
  views       INTEGER NOT NULL DEFAULT 0,
  last_viewed INTEGER NOT NULL DEFAULT 0,
  ip          TEXT NOT NULL DEFAULT '',
  hidden      INTEGER NOT NULL DEFAULT 0,  -- removed by moderation, kept for appeal
  created_at  INTEGER NOT NULL,
  expires_at  INTEGER NOT NULL DEFAULT 0
);
CREATE INDEX images_user    ON images(user_id, created_at DESC);
CREATE INDEX images_album   ON images(album_id);
CREATE INDEX images_sha     ON images(sha256);
CREATE INDEX images_expires ON images(expires_at) WHERE expires_at > 0;
CREATE INDEX images_created ON images(created_at DESC);

CREATE TABLE reports (
  id         INTEGER PRIMARY KEY,
  image_id   TEXT NOT NULL REFERENCES images(id) ON DELETE CASCADE,
  reason     TEXT NOT NULL,
  note       TEXT NOT NULL DEFAULT '',
  ip         TEXT NOT NULL DEFAULT '',
  status     TEXT NOT NULL DEFAULT 'open',  -- open | dismissed | actioned
  created_at INTEGER NOT NULL
);

CREATE TABLE bans (
  ip         TEXT PRIMARY KEY,
  reason     TEXT NOT NULL DEFAULT '',
  created_at INTEGER NOT NULL
);

CREATE TABLE settings (
  key   TEXT PRIMARY KEY,
  value TEXT NOT NULL
);
`,
	// 2: admin warnings shown to a user until they acknowledge them
	`ALTER TABLE users ADD COLUMN warning TEXT NOT NULL DEFAULT '';`,
	// 3: link to the person's HEL.ink account (one account for all HEL.ink apps)
	`ALTER TABLE users ADD COLUMN helink_id TEXT;
CREATE UNIQUE INDEX users_helink ON users(helink_id) WHERE helink_id IS NOT NULL;`,
	// 4: public profiles: who owns an image, and which images they show off
	`ALTER TABLE users ADD COLUMN username TEXT;
CREATE UNIQUE INDEX users_username ON users(username COLLATE NOCASE) WHERE username IS NOT NULL;
ALTER TABLE users ADD COLUMN bio TEXT NOT NULL DEFAULT '';
ALTER TABLE users ADD COLUMN website TEXT NOT NULL DEFAULT '';
ALTER TABLE users ADD COLUMN avatar_key TEXT NOT NULL DEFAULT '';
ALTER TABLE users ADD COLUMN show_owner INTEGER NOT NULL DEFAULT 1;
ALTER TABLE images ADD COLUMN listed INTEGER NOT NULL DEFAULT 0;
CREATE INDEX images_listed ON images(user_id, created_at DESC) WHERE listed = 1;`,
	// 5: alt text, what metadata was stripped, private and password-protected
	// images, album order and covers, per-image daily views and referrers,
	// webhooks
	`ALTER TABLE images ADD COLUMN alt TEXT NOT NULL DEFAULT '';
ALTER TABLE images ADD COLUMN stripped TEXT NOT NULL DEFAULT '';
ALTER TABLE images ADD COLUMN private INTEGER NOT NULL DEFAULT 0;
ALTER TABLE images ADD COLUMN pass_hash TEXT NOT NULL DEFAULT '';
ALTER TABLE images ADD COLUMN position INTEGER NOT NULL DEFAULT 0;
ALTER TABLE albums ADD COLUMN cover_id TEXT NOT NULL DEFAULT '';
ALTER TABLE albums ADD COLUMN description TEXT NOT NULL DEFAULT '';
CREATE TABLE image_views (
  image_id TEXT NOT NULL REFERENCES images(id) ON DELETE CASCADE,
  day      TEXT NOT NULL,
  views    INTEGER NOT NULL DEFAULT 0,
  PRIMARY KEY (image_id, day)
);
CREATE TABLE image_referrers (
  image_id TEXT NOT NULL REFERENCES images(id) ON DELETE CASCADE,
  host     TEXT NOT NULL,
  views    INTEGER NOT NULL DEFAULT 0,
  PRIMARY KEY (image_id, host)
);
CREATE TABLE webhooks (
  id         INTEGER PRIMARY KEY,
  user_id    INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  url        TEXT NOT NULL,
  secret     TEXT NOT NULL,
  events     TEXT NOT NULL DEFAULT 'image.uploaded,image.deleted',
  created_at INTEGER NOT NULL,
  last_status TEXT NOT NULL DEFAULT ''
);
CREATE TABLE secrets (
  name  TEXT PRIMARY KEY,
  value TEXT NOT NULL
);
CREATE TABLE variants (
  image_id TEXT NOT NULL REFERENCES images(id) ON DELETE CASCADE,
  key      TEXT NOT NULL,
  size     INTEGER NOT NULL,
  created_at INTEGER NOT NULL,
  PRIMARY KEY (image_id, key)
);`,
	// 6: video and PDF optimisation that runs after upload
	`CREATE TABLE media_jobs (image_id TEXT PRIMARY KEY REFERENCES images(id) ON DELETE CASCADE, created_at INTEGER NOT NULL);`,
	// 7: HOP App Protocol: why an image is hidden ('' | suspended | moderation),
	// so lifting a suspension never brings back images moderation removed;
	// the audit trail of admin actions and the webhook ids already handled.
	`ALTER TABLE images ADD COLUMN hidden_reason TEXT NOT NULL DEFAULT '';
UPDATE images SET hidden_reason='suspended' WHERE hidden=1 AND user_id IN (SELECT id FROM users WHERE status='suspended');
UPDATE images SET hidden_reason='moderation' WHERE hidden=1 AND hidden_reason='';
CREATE TABLE hop_audit (
  id     INTEGER PRIMARY KEY,
  at     INTEGER NOT NULL,
  actor  TEXT NOT NULL DEFAULT '',
  app    TEXT NOT NULL DEFAULT '',
  action TEXT NOT NULL,
  target TEXT NOT NULL DEFAULT '',
  detail TEXT NOT NULL DEFAULT '{}'
);
CREATE TABLE hop_events_seen (
  id TEXT PRIMARY KEY,
  at INTEGER NOT NULL
);`,
}

func (d *DB) migrate() error {
	if _, err := d.Exec(`CREATE TABLE IF NOT EXISTS schema_version (v INTEGER NOT NULL)`); err != nil {
		return err
	}
	var v int
	if err := d.QueryRow(`SELECT COALESCE(MAX(v),0) FROM schema_version`).Scan(&v); err != nil {
		return err
	}
	for i := v; i < len(migrations); i++ {
		tx, err := d.Begin()
		if err != nil {
			return err
		}
		if _, err := tx.Exec(migrations[i]); err != nil {
			tx.Rollback()
			return fmt.Errorf("migration %d: %w", i+1, err)
		}
		if _, err := tx.Exec(`INSERT INTO schema_version (v) VALUES (?)`, i+1); err != nil {
			tx.Rollback()
			return err
		}
		if err := tx.Commit(); err != nil {
			return err
		}
	}
	return nil
}

func now() int64 { return time.Now().Unix() }

// Setting reads a runtime setting stored by the admin panel.
func (d *DB) Setting(ctx context.Context, key, def string) string {
	var v string
	if err := d.QueryRowContext(ctx, `SELECT value FROM settings WHERE key=?`, key).Scan(&v); err != nil {
		return def
	}
	return v
}

// Secret returns a long-lived random value stored in the database (made on
// first use), e.g. the key that signs "unlocked" cookies for password images.
func (d *DB) Secret(ctx context.Context, name string, make func() string) string {
	var v string
	if d.QueryRowContext(ctx, `SELECT value FROM secrets WHERE name=?`, name).Scan(&v) == nil {
		return v
	}
	v = make()
	d.ExecContext(ctx, `INSERT OR IGNORE INTO secrets (name,value) VALUES (?,?)`, name, v)
	d.QueryRowContext(ctx, `SELECT value FROM secrets WHERE name=?`, name).Scan(&v)
	return v
}

// CountView records one view of an image for today, and where it came from.
func (d *DB) CountView(ctx context.Context, id, refHost string) {
	day := time.Now().UTC().Format("2006-01-02")
	d.ExecContext(ctx, `UPDATE images SET views=views+1, last_viewed=? WHERE id=?`, now(), id)
	d.ExecContext(ctx, `INSERT INTO image_views (image_id,day,views) VALUES (?,?,1)
		ON CONFLICT(image_id,day) DO UPDATE SET views=views+1`, id, day)
	if refHost != "" {
		d.ExecContext(ctx, `INSERT INTO image_referrers (image_id,host,views) VALUES (?,?,1)
			ON CONFLICT(image_id,host) DO UPDATE SET views=views+1`, id, refHost)
	}
}

type DayViews struct {
	Day   string `json:"day"`
	Views int    `json:"views"`
}

type HostViews struct {
	Host  string `json:"host"`
	Views int    `json:"views"`
}

// ImageStats returns daily views for the last 30 days and the top referrers.
func (d *DB) ImageStats(ctx context.Context, id string) ([]DayViews, []HostViews) {
	var days []DayViews
	rows, err := d.QueryContext(ctx, `SELECT day, views FROM image_views WHERE image_id=? AND day>=? ORDER BY day`,
		id, time.Now().UTC().AddDate(0, 0, -29).Format("2006-01-02"))
	if err == nil {
		for rows.Next() {
			var v DayViews
			rows.Scan(&v.Day, &v.Views)
			days = append(days, v)
		}
		rows.Close()
	}
	var hosts []HostViews
	rows, err = d.QueryContext(ctx, `SELECT host, views FROM image_referrers WHERE image_id=? ORDER BY views DESC LIMIT 10`, id)
	if err == nil {
		for rows.Next() {
			var v HostViews
			rows.Scan(&v.Host, &v.Views)
			hosts = append(hosts, v)
		}
		rows.Close()
	}
	return days, hosts
}

// Variants lists the resized copies made for an image.
func (d *DB) Variants(ctx context.Context, imageID string) []string {
	var out []string
	rows, err := d.QueryContext(ctx, `SELECT key FROM variants WHERE image_id=?`, imageID)
	if err != nil {
		return nil
	}
	defer rows.Close()
	for rows.Next() {
		var k string
		rows.Scan(&k)
		out = append(out, k)
	}
	return out
}

func (d *DB) AddVariant(ctx context.Context, imageID, key string, size int) {
	d.ExecContext(ctx, `INSERT OR IGNORE INTO variants (image_id,key,size,created_at) VALUES (?,?,?,?)`, imageID, key, size, now())
}

// CountUsage adds one to today's counter for an action (e.g. an AI tool).
func (d *DB) CountUsage(ctx context.Context, action string) {
	key := "usage:" + time.Now().UTC().Format("2006-01-02") + ":" + action
	d.ExecContext(ctx, `INSERT INTO settings (key,value) VALUES (?, '1')
		ON CONFLICT(key) DO UPDATE SET value=CAST(value AS INTEGER)+1`, key)
}

// Usage sums the counters of the last n days, per action.
func (d *DB) Usage(ctx context.Context, days int) map[string]int {
	out := map[string]int{}
	from := "usage:" + time.Now().UTC().AddDate(0, 0, -days+1).Format("2006-01-02")
	rows, err := d.QueryContext(ctx, `SELECT key, CAST(value AS INTEGER) FROM settings WHERE key >= ? AND key < 'usage;'`, from)
	if err != nil {
		return out
	}
	defer rows.Close()
	for rows.Next() {
		var k string
		var n int
		if rows.Scan(&k, &n) == nil {
			out[k[strings.LastIndexByte(k, ':')+1:]] += n
		}
	}
	return out
}

func (d *DB) SetSetting(ctx context.Context, key, value string) error {
	_, err := d.ExecContext(ctx, `INSERT INTO settings (key,value) VALUES (?,?)
		ON CONFLICT(key) DO UPDATE SET value=excluded.value`, key, value)
	return err
}

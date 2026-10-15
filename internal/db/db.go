// Package db owns PixelHop's SQLite database: schema, migrations and queries.
// SQLite runs in-process (pure Go, no CGO), so there is no database server to run.
package db

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
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

func (d *DB) SetSetting(ctx context.Context, key, value string) error {
	_, err := d.ExecContext(ctx, `INSERT INTO settings (key,value) VALUES (?,?)
		ON CONFLICT(key) DO UPDATE SET value=excluded.value`, key, value)
	return err
}

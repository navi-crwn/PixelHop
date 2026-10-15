package db

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

type User struct {
	ID         int64
	Email      string
	Name       string
	PassHash   string
	GoogleID   sql.NullString
	AvatarURL  string
	Role       string
	Status     string
	StatusNote string
	Warning    string // message from an admin, shown until acknowledged
	HelinkID   sql.NullString
	QuotaBytes int64
	CreatedAt  int64
	LastLogin  int64
}

func (u *User) IsAdmin() bool { return u.Role == "admin" }

const userCols = `id,email,name,pass_hash,google_id,avatar_url,role,status,status_note,quota_bytes,created_at,last_login,warning,helink_id`

func scanUser(row interface{ Scan(...any) error }) (*User, error) {
	var u User
	err := row.Scan(&u.ID, &u.Email, &u.Name, &u.PassHash, &u.GoogleID, &u.AvatarURL, &u.Role, &u.Status,
		&u.StatusNote, &u.QuotaBytes, &u.CreatedAt, &u.LastLogin, &u.Warning, &u.HelinkID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return &u, err
}

func (d *DB) InsertUser(ctx context.Context, u *User) error {
	if u.CreatedAt == 0 {
		u.CreatedAt = now()
	}
	if u.Role == "" {
		u.Role = "user"
	}
	if u.Status == "" {
		u.Status = "active"
	}
	q := `INSERT INTO users (email,name,pass_hash,google_id,avatar_url,role,status,quota_bytes,created_at,helink_id) VALUES (?,?,?,?,?,?,?,?,?,?)`
	args := []any{u.Email, u.Name, u.PassHash, u.GoogleID, u.AvatarURL, u.Role, u.Status, u.QuotaBytes, u.CreatedAt, u.HelinkID}
	if u.ID > 0 { // keep ids when importing from the PHP version
		q = `INSERT INTO users (id,email,name,pass_hash,google_id,avatar_url,role,status,quota_bytes,created_at,helink_id) VALUES (?,?,?,?,?,?,?,?,?,?,?)`
		args = append([]any{u.ID}, args...)
	}
	res, err := d.ExecContext(ctx, q, args...)
	if err != nil {
		return err
	}
	u.ID, _ = res.LastInsertId()
	return nil
}

func (d *DB) UserByID(ctx context.Context, id int64) (*User, error) {
	return scanUser(d.QueryRowContext(ctx, `SELECT `+userCols+` FROM users WHERE id=?`, id))
}

func (d *DB) UserByEmail(ctx context.Context, email string) (*User, error) {
	return scanUser(d.QueryRowContext(ctx, `SELECT `+userCols+` FROM users WHERE email=?`, email))
}

func (d *DB) UserByGoogle(ctx context.Context, gid string) (*User, error) {
	return scanUser(d.QueryRowContext(ctx, `SELECT `+userCols+` FROM users WHERE google_id=?`, gid))
}

func (d *DB) UserByHelink(ctx context.Context, id string) (*User, error) {
	return scanUser(d.QueryRowContext(ctx, `SELECT `+userCols+` FROM users WHERE helink_id=?`, id))
}

// MergeUsers moves everything from one account into another (images, albums,
// API keys) and deletes the emptied account. Used to fold an old PixelHop-only
// account into the person's HEL.ink-linked one.
func (d *DB) MergeUsers(ctx context.Context, fromID, intoID int64) error {
	tx, err := d.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, q := range []string{
		`UPDATE images SET user_id=? WHERE user_id=?`,
		`UPDATE albums SET user_id=? WHERE user_id=?`,
		`UPDATE api_keys SET user_id=? WHERE user_id=?`,
	} {
		if _, err := tx.ExecContext(ctx, q, intoID, fromID); err != nil {
			return err
		}
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM users WHERE id=?`, fromID); err != nil {
		return err
	}
	return tx.Commit()
}

func (d *DB) Users(ctx context.Context, search string, limit, offset int) ([]*User, int, error) {
	like := "%" + search + "%"
	var total int
	d.QueryRowContext(ctx, `SELECT COUNT(*) FROM users WHERE email LIKE ? OR name LIKE ?`, like, like).Scan(&total)
	rows, err := d.QueryContext(ctx, `SELECT `+userCols+` FROM users WHERE email LIKE ? OR name LIKE ?
		ORDER BY id DESC LIMIT ? OFFSET ?`, like, like, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var out []*User
	for rows.Next() {
		u, err := scanUser(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, u)
	}
	return out, total, rows.Err()
}

func (d *DB) CountUsers(ctx context.Context) (n int) {
	d.QueryRowContext(ctx, `SELECT COUNT(*) FROM users`).Scan(&n)
	return
}

func (d *DB) UpdateUser(ctx context.Context, u *User) error {
	_, err := d.ExecContext(ctx, `UPDATE users SET email=?,name=?,pass_hash=?,google_id=?,avatar_url=?,role=?,status=?,
		status_note=?,quota_bytes=?,last_login=?,warning=?,helink_id=? WHERE id=?`, u.Email, u.Name, u.PassHash, u.GoogleID, u.AvatarURL,
		u.Role, u.Status, u.StatusNote, u.QuotaBytes, u.LastLogin, u.Warning, u.HelinkID, u.ID)
	return err
}

func (d *DB) DeleteUser(ctx context.Context, id int64) error {
	_, err := d.ExecContext(ctx, `DELETE FROM users WHERE id=?`, id)
	return err
}

// ---- sessions ----

func (d *DB) InsertSession(ctx context.Context, tokenHash string, userID int64, ttl time.Duration, ip, ua string) error {
	t := now()
	_, err := d.ExecContext(ctx, `INSERT INTO sessions (token_hash,user_id,created_at,expires_at,ip,user_agent) VALUES (?,?,?,?,?,?)`,
		tokenHash, userID, t, t+int64(ttl.Seconds()), ip, ua)
	return err
}

func (d *DB) SessionUser(ctx context.Context, tokenHash string) (*User, error) {
	return scanUser(d.QueryRowContext(ctx, `SELECT `+prefixed("u.", userCols)+` FROM sessions s JOIN users u ON u.id=s.user_id
		WHERE s.token_hash=? AND s.expires_at>?`, tokenHash, now()))
}

func (d *DB) DeleteSession(ctx context.Context, tokenHash string) {
	d.ExecContext(ctx, `DELETE FROM sessions WHERE token_hash=?`, tokenHash)
}

func (d *DB) DeleteUserSessions(ctx context.Context, userID int64) {
	d.ExecContext(ctx, `DELETE FROM sessions WHERE user_id=?`, userID)
}

func (d *DB) PurgeSessions(ctx context.Context) {
	d.ExecContext(ctx, `DELETE FROM sessions WHERE expires_at<=?`, now())
}

// ---- API keys ----

type APIKey struct {
	ID        int64
	UserID    int64
	Name      string
	Prefix    string
	CreatedAt int64
	LastUsed  int64
}

func (d *DB) InsertAPIKey(ctx context.Context, userID int64, name, prefix, keyHash string) error {
	_, err := d.ExecContext(ctx, `INSERT INTO api_keys (user_id,name,prefix,key_hash,created_at) VALUES (?,?,?,?,?)`,
		userID, name, prefix, keyHash, now())
	return err
}

func (d *DB) APIKeys(ctx context.Context, userID int64) ([]APIKey, error) {
	rows, err := d.QueryContext(ctx, `SELECT id,user_id,name,prefix,created_at,last_used FROM api_keys WHERE user_id=? ORDER BY id DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []APIKey
	for rows.Next() {
		var k APIKey
		if err := rows.Scan(&k.ID, &k.UserID, &k.Name, &k.Prefix, &k.CreatedAt, &k.LastUsed); err != nil {
			return nil, err
		}
		out = append(out, k)
	}
	return out, rows.Err()
}

func (d *DB) DeleteAPIKey(ctx context.Context, userID, id int64) error {
	_, err := d.ExecContext(ctx, `DELETE FROM api_keys WHERE id=? AND user_id=?`, id, userID)
	return err
}

func (d *DB) APIKeyUser(ctx context.Context, keyHash string) (*User, error) {
	u, err := scanUser(d.QueryRowContext(ctx, `SELECT `+prefixed("u.", userCols)+` FROM api_keys k JOIN users u ON u.id=k.user_id
		WHERE k.key_hash=?`, keyHash))
	if err == nil {
		d.ExecContext(ctx, `UPDATE api_keys SET last_used=? WHERE key_hash=?`, now(), keyHash)
	}
	return u, err
}

func prefixed(p, cols string) string {
	out := p
	for _, c := range cols {
		out += string(c)
		if c == ',' {
			out += p
		}
	}
	return out
}

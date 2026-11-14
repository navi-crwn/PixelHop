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
	Username   sql.NullString // public handle: /u/{username}
	Bio        string
	Website    string
	AvatarKey  string // uploaded profile photo in storage ("" = none)
	ShowOwner  bool   // show name & photo on their images and profile page
	QuotaBytes int64
	CreatedAt  int64
	LastLogin  int64
}

func (u *User) IsAdmin() bool { return u.Role == "admin" }

const userCols = `id,email,name,pass_hash,google_id,avatar_url,role,status,status_note,quota_bytes,created_at,last_login,warning,helink_id,username,bio,website,avatar_key,show_owner`

func scanUser(row interface{ Scan(...any) error }) (*User, error) {
	var u User
	err := row.Scan(&u.ID, &u.Email, &u.Name, &u.PassHash, &u.GoogleID, &u.AvatarURL, &u.Role, &u.Status,
		&u.StatusNote, &u.QuotaBytes, &u.CreatedAt, &u.LastLogin, &u.Warning, &u.HelinkID,
		&u.Username, &u.Bio, &u.Website, &u.AvatarKey, &u.ShowOwner)
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
	u.ShowOwner = true
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

func (d *DB) UserByUsername(ctx context.Context, name string) (*User, error) {
	return scanUser(d.QueryRowContext(ctx, `SELECT `+userCols+` FROM users WHERE username=? COLLATE NOCASE`, name))
}

// UsernameTaken reports whether another account already uses name.
func (d *DB) UsernameTaken(ctx context.Context, name string, exceptID int64) bool {
	var n int
	d.QueryRowContext(ctx, `SELECT COUNT(*) FROM users WHERE username=? COLLATE NOCASE AND id<>?`, name, exceptID).Scan(&n)
	return n > 0
}

// EmailTaken reports whether another account already uses email.
func (d *DB) EmailTaken(ctx context.Context, email string, exceptID int64) bool {
	var n int
	d.QueryRowContext(ctx, `SELECT COUNT(*) FROM users WHERE email=? AND id<>?`, email, exceptID).Scan(&n)
	return n > 0
}

// UsersWithoutUsername lists accounts that still need a handle.
func (d *DB) UsersWithoutUsername(ctx context.Context) ([]*User, error) {
	rows, err := d.QueryContext(ctx, `SELECT `+userCols+` FROM users WHERE username IS NULL`)
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

func (d *DB) UserByHelink(ctx context.Context, id string) (*User, error) {
	return scanUser(d.QueryRowContext(ctx, `SELECT `+userCols+` FROM users WHERE helink_id=?`, id))
}

// MergePlan is what moving one account into another does (or did): the numbers the owner checks first.
type MergePlan struct {
	Images, Albums, APIKeys, Webhooks, Reminders int
	StorageBytes                                 int64  // what the old account holds
	Avatar                                       string // "moved", "kept: the new account has its own" or ""
	Warning                                      bool   // a content warning on the old account moves along
	OldIsAdmin                                   bool   // the old account is an admin: the role does NOT move
	QuotaBefore, QuotaAfter                      int64  // of the new account
	UsedAfter                                    int64  // storage of the new account once merged
}

func (d *DB) count(ctx context.Context, q string, args ...any) (n int) {
	d.QueryRowContext(ctx, q, args...).Scan(&n)
	return
}

// PlanMerge reads what MergeUsers would move, changing nothing.
func (d *DB) PlanMerge(ctx context.Context, from, into *User) MergePlan {
	p := MergePlan{
		Images:      d.count(ctx, `SELECT COUNT(*) FROM images WHERE user_id=?`, from.ID),
		Albums:      d.count(ctx, `SELECT COUNT(*) FROM albums WHERE user_id=?`, from.ID),
		APIKeys:     d.count(ctx, `SELECT COUNT(*) FROM api_keys WHERE user_id=?`, from.ID),
		Webhooks:    d.count(ctx, `SELECT COUNT(*) FROM webhooks WHERE user_id=?`, from.ID),
		Reminders:   d.count(ctx, `SELECT COUNT(*) FROM reminders WHERE user_id=? AND key NOT IN (SELECT key FROM reminders WHERE user_id=?)`, from.ID, into.ID),
		QuotaBefore: into.QuotaBytes, OldIsAdmin: from.Role == "admin", Warning: from.Warning != "" && into.Warning == "",
	}
	p.StorageBytes, _ = d.UserStorage(ctx, from.ID)
	own, _ := d.UserStorage(ctx, into.ID)
	p.UsedAfter = own + p.StorageBytes
	p.QuotaAfter = max(into.QuotaBytes, from.QuotaBytes, p.UsedAfter)
	switch {
	case from.AvatarKey == "":
	case into.AvatarKey == "":
		p.Avatar = "moved"
	default:
		p.Avatar = "kept: the new account has its own"
	}
	return p
}

// MergeUsers moves everything from one account into another in one step: images, albums, API keys,
// webhooks, hidden reminder cards, the profile photo (when the new account has none), a content warning,
// and the larger of the two quotas (never less than what the merged account now stores). The old account is
// then deleted, which ends its sessions and removes its password and Google link with it. It refuses an
// old account that is linked to a HOP ID of its own.
func (d *DB) MergeUsers(ctx context.Context, fromID, intoID int64) (MergePlan, error) {
	if fromID == intoID {
		return MergePlan{}, errors.New("that is the same account")
	}
	from, err := d.UserByID(ctx, fromID)
	if err != nil {
		return MergePlan{}, err
	}
	into, err := d.UserByID(ctx, intoID)
	if err != nil {
		return MergePlan{}, err
	}
	if from.HelinkID.Valid {
		return MergePlan{}, errors.New("the old account is linked to a HOP ID of its own; nothing was moved")
	}
	plan := d.PlanMerge(ctx, from, into)
	tx, err := d.BeginTx(ctx, nil)
	if err != nil {
		return MergePlan{}, err
	}
	defer tx.Rollback()
	for _, q := range []string{
		`UPDATE images SET user_id=? WHERE user_id=?`,
		`UPDATE albums SET user_id=? WHERE user_id=?`,
		`UPDATE api_keys SET user_id=? WHERE user_id=?`,
		`UPDATE webhooks SET user_id=? WHERE user_id=?`,
		`UPDATE OR IGNORE reminders SET user_id=? WHERE user_id=?`,
	} {
		if _, err := tx.ExecContext(ctx, q, intoID, fromID); err != nil {
			return MergePlan{}, err
		}
	}
	avatar, warning := into.AvatarKey, into.Warning
	if avatar == "" {
		avatar = from.AvatarKey
	}
	if warning == "" {
		warning = from.Warning
	}
	// The role stays as it is: in hub mode HOP ID decides who is an admin.
	if _, err := tx.ExecContext(ctx, `UPDATE users SET avatar_key=?, warning=?, quota_bytes=?, block_hotlinks=MAX(block_hotlinks, (SELECT block_hotlinks FROM users WHERE id=?)) WHERE id=?`, avatar, warning, plan.QuotaAfter, fromID, intoID); err != nil {
		return MergePlan{}, err
	}
	// The old photo now belongs to the new account: clear it so deleting the old row cannot be mistaken for a purge.
	if _, err := tx.ExecContext(ctx, `UPDATE users SET avatar_key='' WHERE id=?`, fromID); err != nil {
		return MergePlan{}, err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM users WHERE id=?`, fromID); err != nil {
		return MergePlan{}, err
	}
	return plan, tx.Commit()
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
		status_note=?,quota_bytes=?,last_login=?,warning=?,helink_id=?,username=?,bio=?,website=?,avatar_key=?,show_owner=?
		WHERE id=?`, u.Email, u.Name, u.PassHash, u.GoogleID, u.AvatarURL,
		u.Role, u.Status, u.StatusNote, u.QuotaBytes, u.LastLogin, u.Warning, u.HelinkID,
		u.Username, u.Bio, u.Website, u.AvatarKey, u.ShowOwner, u.ID)
	return err
}

func (d *DB) DeleteUser(ctx context.Context, id int64) error {
	_, err := d.ExecContext(ctx, `DELETE FROM users WHERE id=?`, id)
	return err
}

// ---- sessions ----

// DetachedSession marks a session that no hub browser session stands behind (password, Google,
// local sign-up): HOP ID's state is never used to end it.
const DetachedSession = -1

func (d *DB) InsertSession(ctx context.Context, tokenHash string, userID int64, ttl time.Duration, ip, ua string) error {
	return d.insertSession(ctx, tokenHash, userID, ttl, ip, ua, DetachedSession)
}

// InsertHubSession is a session the hub has just vouched for (its ticket was accepted).
func (d *DB) InsertHubSession(ctx context.Context, tokenHash string, userID int64, ttl time.Duration, ip, ua string) error {
	return d.insertSession(ctx, tokenHash, userID, ttl, ip, ua, now())
}

func (d *DB) insertSession(ctx context.Context, tokenHash string, userID int64, ttl time.Duration, ip, ua string, verified int64) error {
	t := now()
	_, err := d.ExecContext(ctx, `INSERT INTO sessions (token_hash,user_id,created_at,expires_at,ip,user_agent,verified_at) VALUES (?,?,?,?,?,?,?)`,
		tokenHash, userID, t, t+int64(ttl.Seconds()), ip, ua, verified)
	return err
}

// SessionVerifiedAt is when the hub last vouched for the session: unix seconds, 0 for one older than
// the column, DetachedSession for one the hub has no say over.
func (d *DB) SessionVerifiedAt(ctx context.Context, tokenHash string) (int64, error) {
	var at int64
	err := d.QueryRowContext(ctx, `SELECT verified_at FROM sessions WHERE token_hash=? AND expires_at>?`, tokenHash, now()).Scan(&at)
	return at, err
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
	Scope     string // "full" or "upload" (can only upload)
}

func (d *DB) InsertAPIKey(ctx context.Context, userID int64, name, prefix, keyHash string) error {
	return d.InsertAPIKeyScoped(ctx, userID, name, prefix, keyHash, "full")
}

// InsertAPIKeyScoped stores a key limited to scope ("full" or "upload").
func (d *DB) InsertAPIKeyScoped(ctx context.Context, userID int64, name, prefix, keyHash, scope string) error {
	if scope != "upload" {
		scope = "full"
	}
	_, err := d.ExecContext(ctx, `INSERT INTO api_keys (user_id,name,prefix,key_hash,created_at,scope) VALUES (?,?,?,?,?,?)`,
		userID, name, prefix, keyHash, now(), scope)
	return err
}

func (d *DB) APIKeys(ctx context.Context, userID int64) ([]APIKey, error) {
	rows, err := d.QueryContext(ctx, `SELECT id,user_id,name,prefix,created_at,last_used,scope FROM api_keys WHERE user_id=? ORDER BY id DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []APIKey
	for rows.Next() {
		var k APIKey
		if err := rows.Scan(&k.ID, &k.UserID, &k.Name, &k.Prefix, &k.CreatedAt, &k.LastUsed, &k.Scope); err != nil {
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

// APIKeyScope returns the scope of the key with this hash ("full" when unknown).
func (d *DB) APIKeyScope(ctx context.Context, keyHash string) string {
	var s string
	if d.QueryRowContext(ctx, `SELECT scope FROM api_keys WHERE key_hash=?`, keyHash).Scan(&s) != nil || s == "" {
		return "full"
	}
	return s
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

// BlockHotlinks reports whether this user has turned off embedding of their images on other websites.
func (d *DB) BlockHotlinks(ctx context.Context, userID int64) bool {
	var v int
	d.QueryRowContext(ctx, `SELECT block_hotlinks FROM users WHERE id=?`, userID).Scan(&v)
	return v == 1
}

func (d *DB) SetBlockHotlinks(ctx context.Context, userID int64, on bool) error {
	v := 0
	if on {
		v = 1
	}
	_, err := d.ExecContext(ctx, `UPDATE users SET block_hotlinks=? WHERE id=?`, v, userID)
	return err
}

// SessionRow is one signed-in browser, for the "where am I signed in" list.
type SessionRow struct {
	TokenHash string
	CreatedAt int64
	IP        string
	UserAgent string
}

func (d *DB) UserSessions(ctx context.Context, userID int64) []SessionRow {
	rows, err := d.QueryContext(ctx, `SELECT token_hash,created_at,ip,user_agent FROM sessions WHERE user_id=? AND expires_at>? ORDER BY created_at DESC LIMIT 50`, userID, now())
	if err != nil {
		return nil
	}
	defer rows.Close()
	var out []SessionRow
	for rows.Next() {
		var s SessionRow
		if rows.Scan(&s.TokenHash, &s.CreatedAt, &s.IP, &s.UserAgent) == nil {
			out = append(out, s)
		}
	}
	return out
}

// DeleteOtherSessions signs a user out everywhere except the browser holding keepHash.
func (d *DB) DeleteOtherSessions(ctx context.Context, userID int64, keepHash string) {
	d.ExecContext(ctx, `DELETE FROM sessions WHERE user_id=? AND token_hash<>?`, userID, keepHash)
}

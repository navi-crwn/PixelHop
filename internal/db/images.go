package db

import (
	"context"
	"database/sql"
	"errors"
	"strings"
)

var ErrNotFound = errors.New("not found")

type Image struct {
	ID         string
	UserID     sql.NullInt64
	AlbumID    sql.NullString
	OwnerToken string
	Title      string
	OrigName   string
	Mime       string
	Ext        string
	Size       int64
	Width      int
	Height     int
	SHA256     string
	Color      string
	Backend    string
	KeyFull    string
	KeyThumb   string
	Views      int64
	LastViewed int64
	IP         string
	Hidden     bool
	CreatedAt  int64
	ExpiresAt  int64
}

const imageCols = `id,user_id,album_id,owner_token,title,orig_name,mime,ext,size,width,height,sha256,color,
backend,key_full,key_thumb,views,last_viewed,ip,hidden,created_at,expires_at`

func scanImage(row interface{ Scan(...any) error }) (*Image, error) {
	var im Image
	err := row.Scan(&im.ID, &im.UserID, &im.AlbumID, &im.OwnerToken, &im.Title, &im.OrigName, &im.Mime, &im.Ext,
		&im.Size, &im.Width, &im.Height, &im.SHA256, &im.Color, &im.Backend, &im.KeyFull, &im.KeyThumb,
		&im.Views, &im.LastViewed, &im.IP, &im.Hidden, &im.CreatedAt, &im.ExpiresAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return &im, err
}

func (d *DB) InsertImage(ctx context.Context, im *Image) error {
	if im.CreatedAt == 0 {
		im.CreatedAt = now()
	}
	_, err := d.ExecContext(ctx, `INSERT INTO images (`+imageCols+`) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		im.ID, im.UserID, im.AlbumID, im.OwnerToken, im.Title, im.OrigName, im.Mime, im.Ext, im.Size, im.Width, im.Height,
		im.SHA256, im.Color, im.Backend, im.KeyFull, im.KeyThumb, im.Views, im.LastViewed, im.IP, im.Hidden,
		im.CreatedAt, im.ExpiresAt)
	return err
}

func (d *DB) Image(ctx context.Context, id string) (*Image, error) {
	return scanImage(d.QueryRowContext(ctx, `SELECT `+imageCols+` FROM images WHERE id=?`, id))
}

// ImageBySHA finds an earlier upload of the same bytes by the same owner, so
// re-uploading a file returns the existing link instead of storing it twice.
func (d *DB) ImageBySHA(ctx context.Context, sha string, userID sql.NullInt64) (*Image, error) {
	if !userID.Valid {
		return nil, ErrNotFound // guests get a fresh link (and delete token) every time
	}
	return scanImage(d.QueryRowContext(ctx, `SELECT `+imageCols+` FROM images
		WHERE sha256=? AND user_id=? AND hidden=0 AND expires_at=0 LIMIT 1`, sha, userID.Int64))
}

// CountKeyRefs reports how many images point at a storage key; dedupe across
// users can share one stored object, which must outlive all its references.
func (d *DB) CountKeyRefs(ctx context.Context, key string) int {
	var n int
	d.QueryRowContext(ctx, `SELECT COUNT(*) FROM images WHERE key_full=?`, key).Scan(&n)
	return n
}

type ImageQuery struct {
	UserID  int64  // 0 = any
	AlbumID string // "" = any
	Search  string
	Sort    string // new | old | views | size
	Limit   int
	Offset  int
}

func (d *DB) Images(ctx context.Context, q ImageQuery) ([]*Image, int, error) {
	var where []string
	var args []any
	if q.UserID > 0 {
		where = append(where, "user_id=?")
		args = append(args, q.UserID)
	}
	if q.AlbumID != "" {
		where = append(where, "album_id=?")
		args = append(args, q.AlbumID)
	}
	if s := strings.TrimSpace(q.Search); s != "" {
		where = append(where, "(id LIKE ? OR title LIKE ? OR orig_name LIKE ?)")
		like := "%" + strings.NewReplacer("%", `\%`, "_", `\_`).Replace(s) + "%"
		args = append(args, like, like, like)
	}
	w := ""
	if len(where) > 0 {
		w = " WHERE " + strings.Join(where, " AND ")
	}
	order := map[string]string{"old": "created_at ASC", "views": "views DESC", "size": "size DESC"}[q.Sort]
	if order == "" {
		order = "created_at DESC"
	}
	var total int
	if err := d.QueryRowContext(ctx, `SELECT COUNT(*) FROM images`+w, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	if q.Limit <= 0 || q.Limit > 200 {
		q.Limit = 60
	}
	rows, err := d.QueryContext(ctx, `SELECT `+imageCols+` FROM images`+w+` ORDER BY `+order+` LIMIT ? OFFSET ?`,
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

func (d *DB) DeleteImage(ctx context.Context, id string) error {
	_, err := d.ExecContext(ctx, `DELETE FROM images WHERE id=?`, id)
	return err
}

func (d *DB) SetImageHidden(ctx context.Context, id string, hidden bool) error {
	_, err := d.ExecContext(ctx, `UPDATE images SET hidden=? WHERE id=?`, hidden, id)
	return err
}

func (d *DB) SetImageTitle(ctx context.Context, id, title string) error {
	_, err := d.ExecContext(ctx, `UPDATE images SET title=? WHERE id=?`, title, id)
	return err
}

func (d *DB) AddView(ctx context.Context, id string) {
	d.ExecContext(ctx, `UPDATE images SET views=views+1, last_viewed=? WHERE id=?`, now(), id)
}

// ExpiredImages lists images past their expiry date, or (when inactiveDays>0)
// images nobody has viewed for that many days.
func (d *DB) ExpiredImages(ctx context.Context, inactiveDays int, limit int) ([]*Image, error) {
	t := now()
	q := `SELECT ` + imageCols + ` FROM images WHERE (expires_at > 0 AND expires_at <= ?)`
	args := []any{t}
	if inactiveDays > 0 {
		cut := t - int64(inactiveDays)*86400
		q += ` OR (MAX(last_viewed, created_at) < ?)`
		args = append(args, cut)
	}
	rows, err := d.QueryContext(ctx, q+` LIMIT ?`, append(args, limit)...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*Image
	for rows.Next() {
		im, err := scanImage(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, im)
	}
	return out, rows.Err()
}

func (d *DB) UserStorage(ctx context.Context, userID int64) (bytes int64, count int) {
	d.QueryRowContext(ctx, `SELECT COALESCE(SUM(size),0), COUNT(*) FROM images WHERE user_id=?`, userID).Scan(&bytes, &count)
	return
}

// ---- albums ----

type Album struct {
	ID         string
	UserID     sql.NullInt64
	Title      string
	OwnerToken string
	CreatedAt  int64
}

func (d *DB) InsertAlbum(ctx context.Context, a *Album) error {
	a.CreatedAt = now()
	_, err := d.ExecContext(ctx, `INSERT INTO albums (id,user_id,title,owner_token,created_at) VALUES (?,?,?,?,?)`,
		a.ID, a.UserID, a.Title, a.OwnerToken, a.CreatedAt)
	return err
}

func (d *DB) Album(ctx context.Context, id string) (*Album, error) {
	var a Album
	err := d.QueryRowContext(ctx, `SELECT id,user_id,title,owner_token,created_at FROM albums WHERE id=?`, id).
		Scan(&a.ID, &a.UserID, &a.Title, &a.OwnerToken, &a.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return &a, err
}

func (d *DB) DeleteEmptyAlbums(ctx context.Context) {
	d.ExecContext(ctx, `DELETE FROM albums WHERE id NOT IN (SELECT album_id FROM images WHERE album_id IS NOT NULL)`)
}

package db

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"
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
	Listed     bool   // shown on the owner's public profile
	Alt        string // description for screen readers and embeds
	Stripped   string // kinds of metadata removed on upload: "gps,camera,date"
	Private    bool   // only the owner (and admins) can see it
	PassHash   string // "" or a password visitors must enter
	Position   int    // order inside its album
}

// Kind is "image", "video" or "pdf".
func (im *Image) Kind() string {
	switch {
	case strings.HasPrefix(im.Mime, "video/"):
		return "video"
	case im.Mime == "application/pdf":
		return "pdf"
	}
	return "image"
}

// IsImage reports whether the file is a picture (the transforms, studio and
// embed codes only make sense for those).
func (im *Image) IsImage() bool { return im.Kind() == "image" }

// Locked reports whether visitors need a password (or must be the owner).
func (im *Image) Locked() bool { return im.Private || im.PassHash != "" }

const imageCols = `id,user_id,album_id,owner_token,title,orig_name,mime,ext,size,width,height,sha256,color,
backend,key_full,key_thumb,views,last_viewed,ip,hidden,created_at,expires_at,listed,
alt,stripped,private,pass_hash,position`

func scanImage(row interface{ Scan(...any) error }) (*Image, error) {
	var im Image
	err := row.Scan(&im.ID, &im.UserID, &im.AlbumID, &im.OwnerToken, &im.Title, &im.OrigName, &im.Mime, &im.Ext,
		&im.Size, &im.Width, &im.Height, &im.SHA256, &im.Color, &im.Backend, &im.KeyFull, &im.KeyThumb,
		&im.Views, &im.LastViewed, &im.IP, &im.Hidden, &im.CreatedAt, &im.ExpiresAt, &im.Listed,
		&im.Alt, &im.Stripped, &im.Private, &im.PassHash, &im.Position)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return &im, err
}

func (d *DB) InsertImage(ctx context.Context, im *Image) error {
	if im.CreatedAt == 0 {
		im.CreatedAt = now()
	}
	_, err := d.ExecContext(ctx, `INSERT INTO images (`+imageCols+`) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		im.ID, im.UserID, im.AlbumID, im.OwnerToken, im.Title, im.OrigName, im.Mime, im.Ext, im.Size, im.Width, im.Height,
		im.SHA256, im.Color, im.Backend, im.KeyFull, im.KeyThumb, im.Views, im.LastViewed, im.IP, im.Hidden,
		im.CreatedAt, im.ExpiresAt, im.Listed, im.Alt, im.Stripped, im.Private, im.PassHash, im.Position)
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
	UserID      int64  // 0 = any
	AlbumID     string // "" = any
	Search      string
	Listed      bool   // only images shown on the owner's profile (and still live)
	PublicOwner bool   // only images whose owner shows their profile
	Sort        string // new | old | views | size
	Limit       int
	Offset      int
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
	if q.Listed {
		where = append(where, "listed=1 AND hidden=0 AND private=0 AND pass_hash='' AND (expires_at=0 OR expires_at>?)")
		args = append(args, now())
	}
	if q.PublicOwner {
		where = append(where, "user_id IN (SELECT id FROM users WHERE show_owner=1 AND status='active')")
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
	order := map[string]string{"old": "created_at ASC", "views": "views DESC", "size": "size DESC", "album": "position ASC, created_at ASC"}[q.Sort]
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

// UpdateImageMeta saves the fields an owner can edit after upload.
func (d *DB) UpdateImageMeta(ctx context.Context, im *Image) error {
	_, err := d.ExecContext(ctx, `UPDATE images SET title=?, alt=?, listed=?, private=?, pass_hash=?, album_id=?, position=? WHERE id=?`,
		im.Title, im.Alt, im.Listed, im.Private, im.PassHash, im.AlbumID, im.Position, im.ID)
	return err
}

func (d *DB) SetImageListed(ctx context.Context, id string, listed bool) error {
	_, err := d.ExecContext(ctx, `UPDATE images SET listed=? WHERE id=?`, listed, id)
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
	ID          string
	UserID      sql.NullInt64
	Title       string
	OwnerToken  string
	CreatedAt   int64
	CoverID     string
	Description string
}

func (d *DB) InsertAlbum(ctx context.Context, a *Album) error {
	a.CreatedAt = now()
	_, err := d.ExecContext(ctx, `INSERT INTO albums (id,user_id,title,owner_token,created_at) VALUES (?,?,?,?,?)`,
		a.ID, a.UserID, a.Title, a.OwnerToken, a.CreatedAt)
	return err
}

func (d *DB) Album(ctx context.Context, id string) (*Album, error) {
	var a Album
	err := d.QueryRowContext(ctx, `SELECT id,user_id,title,owner_token,created_at,cover_id,description FROM albums WHERE id=?`, id).
		Scan(&a.ID, &a.UserID, &a.Title, &a.OwnerToken, &a.CreatedAt, &a.CoverID, &a.Description)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return &a, err
}

func (d *DB) UpdateAlbum(ctx context.Context, a *Album) error {
	_, err := d.ExecContext(ctx, `UPDATE albums SET title=?, cover_id=?, description=? WHERE id=?`, a.Title, a.CoverID, a.Description, a.ID)
	return err
}

// Albums lists someone's albums, newest first, with how many images each has.
func (d *DB) Albums(ctx context.Context, userID int64) ([]AlbumRow, error) {
	rows, err := d.QueryContext(ctx, `SELECT a.id, a.title, a.cover_id, a.created_at, COUNT(i.id),
		COALESCE((SELECT id FROM images WHERE album_id=a.id ORDER BY position, created_at LIMIT 1), '')
		FROM albums a LEFT JOIN images i ON i.album_id=a.id WHERE a.user_id=? GROUP BY a.id ORDER BY a.created_at DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []AlbumRow
	for rows.Next() {
		var r AlbumRow
		if err := rows.Scan(&r.ID, &r.Title, &r.CoverID, &r.CreatedAt, &r.Count, &r.FirstID); err != nil {
			return nil, err
		}
		if r.CoverID == "" {
			r.CoverID = r.FirstID
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

type AlbumRow struct {
	ID, Title, CoverID, FirstID string
	CreatedAt                   int64
	Count                       int
}

func (d *DB) DeleteAlbum(ctx context.Context, id string) error {
	_, err := d.ExecContext(ctx, `DELETE FROM albums WHERE id=?`, id) // images stay; album_id is set to NULL
	return err
}

func (d *DB) DeleteEmptyAlbums(ctx context.Context) {
	d.ExecContext(ctx, `DELETE FROM albums WHERE id NOT IN (SELECT album_id FROM images WHERE album_id IS NOT NULL)`)
}

// ---- media optimisation queue ----

func (d *DB) AddMediaJob(ctx context.Context, id string) error {
	_, err := d.ExecContext(ctx, `INSERT OR IGNORE INTO media_jobs (image_id, created_at) VALUES (?, ?)`, id, time.Now().Unix())
	return err
}

func (d *DB) DoneMediaJob(ctx context.Context, id string) {
	d.ExecContext(ctx, `DELETE FROM media_jobs WHERE image_id=?`, id)
}

func (d *DB) MediaJobPending(ctx context.Context, id string) bool {
	var n int
	d.QueryRowContext(ctx, `SELECT COUNT(*) FROM media_jobs WHERE image_id=?`, id).Scan(&n)
	return n > 0
}

func (d *DB) MediaJobs(ctx context.Context) []string {
	rows, err := d.QueryContext(ctx, `SELECT image_id FROM media_jobs ORDER BY created_at`)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		rows.Scan(&id)
		ids = append(ids, id)
	}
	return ids
}

// ReplaceFile points an image at a new stored file (after optimising it).
func (d *DB) ReplaceFile(ctx context.Context, id, key, mime, ext string, size int64, w, h int) error {
	_, err := d.ExecContext(ctx, `UPDATE images SET key_full=?, mime=?, ext=?, size=?, width=?, height=? WHERE id=?`, key, mime, ext, size, w, h, id)
	return err
}

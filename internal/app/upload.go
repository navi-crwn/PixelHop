package app

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"math/big"
	"mime/multipart"
	"net/http"
	"path"
	"strings"
	"time"
	"unicode"

	"github.com/navi-crwn/pixelhop/internal/db"
	"github.com/navi-crwn/pixelhop/internal/imaging"
	"github.com/navi-crwn/pixelhop/internal/netsafe"
)

var expiries = map[string]time.Duration{
	"10m": 10 * time.Minute, "1h": time.Hour, "1d": 24 * time.Hour,
	"7d": 7 * 24 * time.Hour, "30d": 30 * 24 * time.Hour, "90d": 90 * 24 * time.Hour,
}

type uploadInput struct {
	name string
	data []byte
	err  error
}

type uploadResult struct {
	Image       *db.Image `json:"-"`
	DeleteToken string    `json:"-"`
	Name        string    `json:"-"`
	Err         error     `json:"-"`
	Duplicate   bool      `json:"-"`
}

type userError struct{ msg string }

func (e userError) Error() string { return e.msg }

// handleUpload reads every file (and URL) in the request and stores them.
// It returns per-file results, and an album id when several were uploaded.
func (a *App) handleUpload(r *http.Request) ([]uploadResult, string, error) {
	ctx := r.Context()
	u := currentUser(r)
	ip := clientIP(r)

	if u == nil && a.db.Setting(ctx, "guest_uploads", fmt.Sprint(a.cfg.GuestUploads)) != "true" {
		return nil, "", userError{"Please sign in to upload on this server."}
	}

	maxBatch := a.cfg.MaxBatch
	r.Body = http.MaxBytesReader(nil, r.Body, a.cfg.MaxUploadBytes*int64(maxBatch)+(1<<20))
	if err := r.ParseMultipartForm(32 << 20); err != nil && !errors.Is(err, http.ErrNotMultipart) {
		var mbe *http.MaxBytesError
		if errors.As(err, &mbe) {
			return nil, "", userError{fmt.Sprintf("That's too much at once: the limit is %d files of %d MB each.", maxBatch, a.cfg.MaxUploadBytes>>20)}
		}
		if err := r.ParseForm(); err != nil {
			return nil, "", userError{"The upload didn't arrive in one piece. Please try again."}
		}
	}

	var inputs []uploadInput
	if r.MultipartForm != nil {
		for _, field := range []string{"file", "files", "files[]", "image", "images", "images[]"} {
			for _, fh := range r.MultipartForm.File[field] {
				inputs = append(inputs, a.readPart(fh))
			}
		}
	}
	for _, raw := range strings.Fields(r.FormValue("url")) {
		inputs = append(inputs, uploadInput{name: raw})
	}
	if len(inputs) == 0 {
		return nil, "", userError{"Choose an image to upload."}
	}
	if len(inputs) > maxBatch {
		return nil, "", userError{fmt.Sprintf("You can upload up to %d images at once.", maxBatch)}
	}
	perMinute := a.cfg.UploadsPerMinute
	if u != nil {
		perMinute *= 4
	}
	for range inputs {
		if !a.limits.allow("upload:"+ip, perMinute, time.Minute) {
			return nil, "", userError{"You're uploading very fast. Please wait a minute."}
		}
	}

	var expiresAt int64
	if d, ok := expiries[r.FormValue("expire")]; ok {
		expiresAt = time.Now().Add(d).Unix()
	}
	useSlug := r.FormValue("slug") != "0" && r.FormValue("slug") != "false"

	// Several files become an album with its own share link. Signed-in people
	// can keep adding to one of their albums (batch uploads come in chunks).
	albumID := ""
	keepAlbum := false
	if aid := r.FormValue("album_id"); aid != "" && u != nil && validID(aid) {
		if al, err := a.db.Album(ctx, aid); err == nil && al.UserID.Valid && al.UserID.Int64 == u.ID {
			albumID, keepAlbum = aid, true
		}
	}
	if albumID == "" && (len(inputs) > 1 || r.FormValue("album") == "force") && r.FormValue("album") != "0" {
		al := &db.Album{ID: newID(""), Title: trim(r.FormValue("title"), 120)}
		if u != nil {
			al.UserID = sql.NullInt64{Int64: u.ID, Valid: true}
		}
		if err := a.db.InsertAlbum(ctx, al); err == nil {
			albumID = al.ID
		}
	}

	var used int64
	if u != nil {
		used, _ = a.db.UserStorage(ctx, u.ID)
	}

	results := make([]uploadResult, 0, len(inputs))
	for _, in := range inputs {
		res := uploadResult{Name: in.name}
		if in.err == nil && in.data == nil { // a URL to fetch
			fctx, cancel := context.WithTimeout(ctx, 40*time.Second)
			var p string
			in.data, p, in.err = netsafe.Fetch(fctx, in.name, a.cfg.MaxUploadBytes)
			cancel()
			if in.err == nil {
				in.name = path.Base(p)
			} else {
				in.err = userError{in.err.Error()} // netsafe's messages are written for people
			}
		}
		if in.err != nil {
			res.Err = in.err
			results = append(results, res)
			continue
		}
		if u != nil && used+int64(len(in.data)) > u.QuotaBytes {
			res.Err = userError{fmt.Sprintf("Your storage is full (%s of %s). Delete some images to make room.", humanBytes(used), humanBytes(u.QuotaBytes))}
			results = append(results, res)
			continue
		}
		im, tok, dup, err := a.storeImage(ctx, in, u, ip, albumID, expiresAt, useSlug)
		res.Image, res.DeleteToken, res.Duplicate, res.Err = im, tok, dup, err
		if err == nil && !dup {
			used += im.Size
		}
		results = append(results, res)
	}
	if albumID != "" && !keepAlbum && r.FormValue("album") != "force" {
		ok := 0
		for _, res := range results {
			if res.Err == nil && !res.Duplicate {
				ok++
			}
		}
		if ok < 2 { // nothing to group: drop the album again
			for i := range results {
				if results[i].Image != nil && results[i].Image.AlbumID.String == albumID {
					results[i].Image.AlbumID = sql.NullString{}
					a.db.ExecContext(ctx, `UPDATE images SET album_id=NULL WHERE id=?`, results[i].Image.ID)
				}
			}
			a.db.ExecContext(ctx, `DELETE FROM albums WHERE id=?`, albumID)
			albumID = ""
		}
	}
	return results, albumID, nil
}

func (a *App) readPart(fh *multipart.FileHeader) uploadInput {
	in := uploadInput{name: fh.Filename}
	if fh.Size > a.cfg.MaxUploadBytes {
		in.err = userError{fmt.Sprintf("%s is larger than %d MB.", fh.Filename, a.cfg.MaxUploadBytes>>20)}
		return in
	}
	f, err := fh.Open()
	if err != nil {
		in.err = err
		return in
	}
	defer f.Close()
	in.data, in.err = io.ReadAll(io.LimitReader(f, a.cfg.MaxUploadBytes+1))
	if in.err == nil && int64(len(in.data)) > a.cfg.MaxUploadBytes {
		in.err = userError{fmt.Sprintf("%s is larger than %d MB.", fh.Filename, a.cfg.MaxUploadBytes>>20)}
	}
	return in
}

func (a *App) storeImage(ctx context.Context, in uploadInput, u *db.User, ip, albumID string, expiresAt int64, useSlug bool) (*db.Image, string, bool, error) {
	sum := sha256.Sum256(in.data)
	sha := hex.EncodeToString(sum[:])
	var uid sql.NullInt64
	if u != nil {
		uid = sql.NullInt64{Int64: u.ID, Valid: true}
		if old, err := a.db.ImageBySHA(ctx, sha, uid); err == nil {
			return old, "", true, nil
		}
	}

	p, err := imaging.Process(in.data, a.cfg.MaxPixels)
	if err != nil {
		return nil, "", false, userError{err.Error()}
	}

	slug := ""
	if useSlug {
		slug = slugify(strings.TrimSuffix(in.name, path.Ext(in.name)))
	}
	im := &db.Image{
		UserID: uid, Title: trim(strings.TrimSuffix(in.name, path.Ext(in.name)), 120), OrigName: trim(in.name, 200),
		Mime: p.Format.Mime, Ext: p.Format.Ext, Size: int64(len(p.Data)), Width: p.Width, Height: p.Height,
		SHA256: sha, Color: p.Color, Backend: a.store.Name(), IP: ip, ExpiresAt: expiresAt,
	}
	if albumID != "" {
		im.AlbumID = sql.NullString{String: albumID, Valid: true}
	}
	token := ""
	if u == nil {
		token = randomToken(18)
		im.OwnerToken = hashToken(token)
	}

	day := time.Now().UTC().Format("2006/01/02")
	for attempt := 0; ; attempt++ {
		im.ID = newID(slug)
		im.KeyFull = day + "/" + im.ID + "." + im.Ext
		im.KeyThumb = day + "/" + im.ID + "_thumb." + p.ThumbExt
		if err := a.store.Put(ctx, im.KeyFull, bytes.NewReader(p.Data), int64(len(p.Data)), im.Mime); err != nil {
			return nil, "", false, err
		}
		if err := a.store.Put(ctx, im.KeyThumb, bytes.NewReader(p.Thumb), int64(len(p.Thumb)), "image/"+map[string]string{"jpg": "jpeg", "png": "png"}[p.ThumbExt]); err != nil {
			a.store.Delete(ctx, im.KeyFull)
			return nil, "", false, err
		}
		err := a.db.InsertImage(ctx, im)
		if err == nil {
			break
		}
		a.store.Delete(ctx, im.KeyFull)
		a.store.Delete(ctx, im.KeyThumb)
		if attempt >= 3 || !strings.Contains(err.Error(), "UNIQUE") {
			return nil, "", false, err
		}
	}
	return im, token, false, nil
}

// ImportCopy downloads an image from its old public address and stores it
// in this server's storage under the same id, with fresh thumbnails. Used by
// "pixelhop import --copy"; the URL comes from the old database, not a visitor.
func (a *App) ImportCopy(ctx context.Context, im *db.Image, url string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download: %s", resp.Status)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, 200<<20))
	if err != nil {
		return err
	}
	p, err := imaging.Process(data, 1<<40)
	if err != nil {
		return err
	}
	day := time.Unix(im.CreatedAt, 0).UTC().Format("2006/01/02")
	im.Backend, im.Mime, im.Ext = a.store.Name(), p.Format.Mime, p.Format.Ext
	im.Size, im.Width, im.Height, im.Color = int64(len(p.Data)), p.Width, p.Height, p.Color
	im.KeyFull = day + "/" + im.ID + "." + im.Ext
	im.KeyThumb = day + "/" + im.ID + "_thumb." + p.ThumbExt
	if err := a.store.Put(ctx, im.KeyFull, bytes.NewReader(p.Data), int64(len(p.Data)), im.Mime); err != nil {
		return err
	}
	if err := a.store.Put(ctx, im.KeyThumb, bytes.NewReader(p.Thumb), int64(len(p.Thumb)), "image/"+map[string]string{"jpg": "jpeg", "png": "png"}[p.ThumbExt]); err != nil {
		return err
	}
	return a.db.InsertImage(ctx, im)
}

// removeImage deletes an image's row and its stored files.
func (a *App) removeImage(ctx context.Context, im *db.Image) error {
	if err := a.db.DeleteImage(ctx, im.ID); err != nil {
		return err
	}
	if b := a.backends[im.Backend]; b != nil && a.db.CountKeyRefs(ctx, im.KeyFull) == 0 {
		b.Delete(ctx, im.KeyFull)
		if im.KeyThumb != im.KeyFull {
			b.Delete(ctx, im.KeyThumb)
		}
	}
	return nil
}

const idAlphabet = "abcdefghijkmnopqrstuvwxyzABCDEFGHJKLMNPQRSTUVWXYZ23456789"

// newID makes a link id: a readable slug from the file name (when there is
// one) plus 6 random characters, e.g. "beach-sunset-x7Kp2Q". Look-alike
// characters (0/O, 1/l/I) are left out so ids can be read aloud.
func newID(slug string) string {
	var b strings.Builder
	if slug != "" {
		b.WriteString(slug)
		b.WriteByte('-')
	}
	n := big.NewInt(int64(len(idAlphabet)))
	for i := 0; i < 6; i++ {
		k, _ := rand.Int(rand.Reader, n)
		b.WriteByte(idAlphabet[k.Int64()])
	}
	return b.String()
}

// slugify keeps the first few words of a file name, in lower-case ASCII.
func slugify(s string) string {
	var b strings.Builder
	dash := false
	for _, r := range strings.ToLower(s) {
		switch {
		case r < 128 && (unicode.IsLetter(r) || unicode.IsDigit(r)):
			b.WriteRune(r)
			dash = false
		case !dash && b.Len() > 0:
			b.WriteByte('-')
			dash = true
		}
	}
	out := strings.Trim(b.String(), "-")
	// Camera names like "img-20240101-123456" are noise, not a description.
	if strings.HasPrefix(out, "img-") || strings.HasPrefix(out, "dsc") || strings.HasPrefix(out, "screenshot-20") ||
		strings.HasPrefix(out, "whatsapp-image") || strings.HasPrefix(out, "pxl-") || len(strings.Trim(out, "0123456789-")) == 0 {
		return ""
	}
	if len(out) > 24 {
		out = out[:24]
		if i := strings.LastIndexByte(out, '-'); i > 10 {
			out = out[:i]
		}
	}
	return strings.Trim(out, "-")
}

func validID(id string) bool {
	if id == "" || len(id) > 64 {
		return false
	}
	for _, c := range id {
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '-' || c == '_') {
			return false
		}
	}
	return true
}

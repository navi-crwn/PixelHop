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
	"os"
	"path"
	"regexp"
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
	name           string
	data           []byte
	path           string // big videos and PDFs wait on disk instead of in memory
	size           int64
	err            error
	contentWarning string
	warningReason  string
}

type uploadResult struct {
	Image       *db.Image `json:"-"`
	DeleteToken string    `json:"-"`
	Name        string    `json:"-"`
	Err         error     `json:"-"`
	Duplicate   bool      `json:"-"`
}

// errBlockedFile is returned for a file that was taken down before and is not welcome again.
var errBlockedFile = errors.New("blocked file")

type userError struct{ msg string }

func (e userError) Error() string { return e.msg }

// handleUpload reads every file (and URL) in the request and stores them.
// It returns per-file results, and an album id when several were uploaded.
func (a *App) handleUpload(r *http.Request) ([]uploadResult, string, error) {
	ctx := withLang(r)
	u := currentUser(r)
	ip := clientIP(r)

	if u == nil && a.db.Setting(ctx, "guest_uploads", fmt.Sprint(a.cfg.GuestUploads)) != "true" {
		return nil, "", userError{a.tr(r, "Please sign in to upload on this server.")}
	}

	maxBatch := a.cfg.MaxBatch
	r.Body = http.MaxBytesReader(nil, r.Body, a.cfg.MaxUploadBytes*int64(maxBatch)+(1<<20))
	if err := r.ParseMultipartForm(32 << 20); err != nil && !errors.Is(err, http.ErrNotMultipart) {
		var mbe *http.MaxBytesError
		if errors.As(err, &mbe) {
			return nil, "", userError{a.tr(r, "That's too much at once: the limit is {n} files of {mb} MB each.", "n", maxBatch, "mb", a.cfg.MaxUploadBytes>>20)}
		}
		if err := r.ParseForm(); err != nil {
			return nil, "", userError{a.tr(r, "The upload didn't arrive in one piece. Please try again.")}
		}
	}

	var inputs []uploadInput
	if r.MultipartForm != nil {
		for _, field := range []string{"file", "files", "files[]", "image", "images", "images[]"} {
			for _, fh := range r.MultipartForm.File[field] {
				inputs = append(inputs, a.readPart(ctx, fh, u))
			}
		}
	}
	for _, raw := range strings.Fields(r.FormValue("url")) {
		inputs = append(inputs, uploadInput{name: raw})
	}
	if len(inputs) == 0 {
		return nil, "", userError{a.tr(r, "Choose a file to upload.")}
	}
	if len(inputs) > maxBatch {
		return nil, "", userError{a.tr(r, "You can upload up to {n} images at once.", "n", maxBatch)}
	}
	perMinute := a.cfg.UploadsPerMinute
	if u != nil {
		perMinute *= 4
	}
	for range inputs {
		if !a.limits.allow("upload:"+ip, perMinute, time.Minute) {
			return nil, "", userError{a.tr(r, "You're uploading very fast. Please wait a minute.")}
		}
	}
	// Guests: how many more files this address may store today (-1 = no
	// limit). A guest who already used the day's share is refused outright.
	dailyRoom := -1
	if u == nil {
		var err error
		if dailyRoom, err = a.guestDailyRoom(r); err != nil {
			return nil, "", err
		}
	}

	var expiresAt int64
	if d, ok := expiries[r.FormValue("expire")]; ok {
		expiresAt = time.Now().Add(d).Unix()
	}
	expiresAt = a.guestExpiry(ctx, u, expiresAt)
	useSlug := r.FormValue("slug") != "0" && r.FormValue("slug") != "false"
	listed := u != nil && (r.FormValue("listed") == "1" || r.FormValue("listed") == "true")
	alt := trim(strings.TrimSpace(r.FormValue("alt")), 300)
	contentWarning := strings.ToLower(strings.TrimSpace(r.FormValue("content_warning")))
	if contentWarning != "sensitive" && contentWarning != "adult" {
		contentWarning = "none"
	}
	warningReason := trim(strings.TrimSpace(r.FormValue("warning_reason")), 255)

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
		al := &db.Album{
			ID:             newID(""),
			Title:          trim(r.FormValue("title"), 120),
			ContentWarning: contentWarning,
			WarningReason:  warningReason,
		}
		if u != nil {
			al.UserID = sql.NullInt64{Int64: u.ID, Valid: true}
		}
		if err := a.db.InsertAlbum(ctx, al); err == nil {
			albumID = al.ID
			if note := trim(strings.TrimSpace(r.FormValue("note")), 1000); note != "" {
				al.AdminNote = note // for the owner and the moderators; the shared page never shows it
				a.db.UpdateAlbum(ctx, al)
			}
		}
	}

	// An album made by a guest holds only so many images (-1 = no limit).
	albumRoom := -1
	if u == nil && albumID != "" {
		if max := a.guestAlbumMax(ctx); max > 0 {
			albumRoom = max
		}
	}

	results := make([]uploadResult, 0, len(inputs))
	for _, in := range inputs {
		res := uploadResult{Name: in.name}
		if in.err == nil && in.data == nil && in.path == "" { // a URL to fetch
			fctx, cancel := context.WithTimeout(ctx, 40*time.Second)
			var p string
			in.data, p, in.err = netsafe.Fetch(fctx, in.name, a.imageLimit(u))
			cancel()
			if in.err == nil {
				in.name = path.Base(p)
			} else {
				in.err = userError{a.localizedError(ctx, in.err.Error())} // netsafe's messages are written for people
			}
		}
		if in.err != nil {
			res.Err = in.err
			results = append(results, res)
			continue
		}
		if in.size == 0 {
			in.size = int64(len(in.data))
		}
		if dailyRoom == 0 {
			// The day's share ran out during this batch: the rest waits.
			room, err := a.guestDailyRoom(r)
			if err != nil {
				res.Err = err
				results = append(results, res)
				continue
			}
			dailyRoom = room
		}
		if albumRoom == 0 {
			res.Err = a.guestAlbumFull(r, a.guestAlbumMax(ctx))
			results = append(results, res)
			continue
		}
		in.contentWarning = contentWarning
		in.warningReason = warningReason
		done, used, fits := a.reserveQuota(ctx, u, in.size)
		if !fits {
			res.Err = userError{a.tr(r, "Your storage is full ({used} of {quota}). Delete some images to make room.", "used", humanBytes(used), "quota", humanBytes(u.QuotaBytes))}
			results = append(results, res)
			continue
		}
		im, tok, dup, err := func() (*db.Image, string, bool, error) {
			defer done() // also when storing panics: the member's lock must not stay taken
			return a.storeImage(ctx, in, u, ip, albumID, expiresAt, useSlug, listed, alt)
		}()
		res.Image, res.DeleteToken, res.Duplicate, res.Err = im, tok, dup, err
		if errors.Is(err, errBlockedFile) {
			res.Err = userError{a.tr(r, "This file is not allowed on this server.")}
		}
		if u == nil && res.Err == nil {
			a.countGuestUpload(r)
			if dailyRoom > 0 {
				dailyRoom--
			}
			if albumRoom > 0 {
				albumRoom--
			}
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

// imageLimit and mediaLimit are the largest picture and video/PDF one person
// may upload: guests get the smaller guest limit for everything.
func (a *App) imageLimit(u *db.User) int64 {
	if u == nil {
		return min(a.cfg.MaxUploadBytes, a.cfg.GuestMaxBytes)
	}
	return a.cfg.MaxUploadBytes
}

func (a *App) mediaLimit(u *db.User) int64 {
	if u == nil {
		return min(a.cfg.MaxMediaBytes, a.cfg.GuestMaxBytes)
	}
	return a.cfg.MaxMediaBytes
}

// tooLarge says how big a file may be, and how to get more room when signing in would help.
func (a *App) tooLarge(ctx context.Context, name string, u *db.User, limit int64) userError {
	if u == nil && a.cfg.MaxMediaBytes > limit {
		return userError{a.trc(ctx, "{name} is larger than {mb} MB. Sign in to upload videos and PDFs up to {max} MB.", "name", name, "mb", limit>>20, "max", a.cfg.MaxMediaBytes>>20)}
	}
	return userError{a.trc(ctx, "{name} is larger than {mb} MB.", "name", name, "mb", limit>>20)}
}

func (a *App) readPart(ctx context.Context, fh *multipart.FileHeader, u *db.User) uploadInput {
	in := uploadInput{name: fh.Filename}
	imgMax := a.imageLimit(u)
	if fh.Size > imgMax {
		// Videos and PDFs may be bigger: keep them on disk.
		if f, err := fh.Open(); err == nil {
			hb := make([]byte, 64)
			n, _ := io.ReadFull(f, hb)
			f.Close()
			if kind := mediaKind(hb[:n]); kind != "" {
				if fh.Size <= a.mediaLimit(u) {
					in.path, in.size, in.err = a.spool(fh)
					return in
				}
				in.err = a.tooLarge(ctx, fh.Filename, u, a.mediaLimit(u))
				return in
			}
		}
		in.err = a.tooLarge(ctx, fh.Filename, u, imgMax)
		return in
	}
	f, err := fh.Open()
	if err != nil {
		in.err = err
		return in
	}
	defer f.Close()
	in.data, in.err = io.ReadAll(io.LimitReader(f, imgMax+1))
	if in.err == nil && int64(len(in.data)) > imgMax {
		in.err = a.tooLarge(ctx, fh.Filename, u, imgMax)
	}
	return in
}

func (a *App) storeImage(ctx context.Context, in uploadInput, u *db.User, ip, albumID string, expiresAt int64, useSlug, listed bool, alt string) (*db.Image, string, bool, error) {
	if in.contentWarning == "" {
		in.contentWarning = "none"
	}
	if in.path == "" {
		if kind := mediaKind(in.data[:min(len(in.data), 64)]); kind != "" {
			if in.path, in.err = a.spoolBytes(in.data); in.err != nil {
				return nil, "", false, in.err
			}
		}
	}
	if in.path != "" {
		defer os.Remove(in.path)
		kind := mediaKind(head(in.path, 64))
		if kind == "" { // an image that arrived in pieces
			data, err := os.ReadFile(in.path)
			if err != nil {
				return nil, "", false, err
			}
			if limit := a.imageLimit(u); int64(len(data)) > limit {
				return nil, "", false, a.tooLarge(ctx, in.name, u, limit)
			}
			in.data, in.path = data, ""
		} else {
			return a.storeMedia(ctx, in.path, in.name, kind, u, ip, albumID, expiresAt, useSlug, alt, in.contentWarning, in.warningReason)
		}
	}

	sum := sha256.Sum256(in.data)
	sha := hex.EncodeToString(sum[:])
	if a.db.HashBlocked(ctx, sha) {
		return nil, "", false, errBlockedFile
	}
	var uid sql.NullInt64
	if u != nil {
		uid = sql.NullInt64{Int64: u.ID, Valid: true}
		if old, err := a.db.ImageBySHA(ctx, sha, uid); err == nil && old.ContentWarning == in.contentWarning && old.WarningReason == in.warningReason {
			return old, "", true, nil
		}
	}

	if im, token, ok, err := a.shareStoredImage(ctx, in, sha, u, uid, ip, albumID, expiresAt, useSlug, listed, alt); ok || err != nil {
		return im, token, false, err
	}

	stripped := imaging.MetadataFound(in.data)
	if kind := imaging.Container(in.data); kind != "" {
		// iPhone (HEIC) and AVIF photos: the AI service turns them into JPEG/PNG.
		data, removed, err := a.normalize(ctx, in.data)
		if err != nil {
			return nil, "", false, err
		}
		in.data, stripped = data, removed
	}
	p, err := imaging.Process(in.data, a.cfg.MaxPixels)
	if err != nil {
		return nil, "", false, userError{a.localizedError(ctx, err.Error())}
	}

	slug := ""
	if useSlug {
		slug = slugify(strings.TrimSuffix(in.name, path.Ext(in.name)))
	}
	im := &db.Image{
		UserID: uid, Title: trim(strings.TrimSuffix(in.name, path.Ext(in.name)), 120), OrigName: trim(in.name, 200),
		Mime: p.Format.Mime, Ext: p.Format.Ext, Size: int64(len(p.Data)), Width: p.Width, Height: p.Height,
		SHA256: sha, Color: p.Color, Backend: a.store.Name(), IP: ip, ExpiresAt: expiresAt, Listed: listed,
		Stripped: strings.Join(stripped, ","), Alt: alt, ContentWarning: in.contentWarning, WarningReason: in.warningReason,
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
	if u != nil {
		a.fireWebhooks(u.ID, "image.uploaded", a.imageJSON(im))
	}
	return im, token, false, nil
}

// shareStoredImage gives an upload its own row (link, owner, delete token, expiry, warning, album) that
// points at the file somebody else's upload of the same bytes already stored. ok is false when there is
// nothing to share (the setting is off, or no such file) and the upload is stored as usual. The uploader is
// never told the file was already here: the answer looks like any other upload (duplicate stays for the
// same owner only). The quota still counts the whole file for each owner.
func (a *App) shareStoredImage(ctx context.Context, in uploadInput, sha string, u *db.User, uid sql.NullInt64, ip, albumID string, expiresAt int64, useSlug, listed bool, alt string) (im *db.Image, token string, ok bool, err error) {
	if !a.shareStorageOn(ctx) {
		return nil, "", false, nil
	}
	slug := ""
	if useSlug {
		slug = slugify(strings.TrimSuffix(in.name, path.Ext(in.name)))
	}
	a.shareMu.Lock()
	defer a.shareMu.Unlock()
	src, err := a.db.ImageForSharing(ctx, sha, a.store.Name())
	if err != nil {
		return nil, "", false, nil
	}
	im = &db.Image{
		UserID: uid, Title: trim(strings.TrimSuffix(in.name, path.Ext(in.name)), 120), OrigName: trim(in.name, 200),
		Mime: src.Mime, Ext: src.Ext, Size: src.Size, Width: src.Width, Height: src.Height,
		SHA256: sha, Color: src.Color, Backend: src.Backend, KeyFull: src.KeyFull, KeyThumb: src.KeyThumb,
		IP: ip, ExpiresAt: expiresAt, Listed: listed,
		Stripped: src.Stripped, Alt: alt, ContentWarning: in.contentWarning, WarningReason: in.warningReason,
	}
	if albumID != "" {
		im.AlbumID = sql.NullString{String: albumID, Valid: true}
	}
	if u == nil {
		token = randomToken(18)
		im.OwnerToken = hashToken(token)
	}
	for attempt := 0; ; attempt++ {
		im.ID = newID(slug)
		err = a.db.InsertImage(ctx, im)
		if err == nil {
			break
		}
		if attempt >= 3 || !strings.Contains(err.Error(), "UNIQUE") {
			return nil, "", false, err
		}
	}
	if u != nil {
		a.fireWebhooks(u.ID, "image.uploaded", a.imageJSON(im))
	}
	return im, token, true, nil
}

// Messages the imaging and download code writes in English, with the numbers they carry: each is turned
// into a catalog entry, so the person reads it in their own language.
var errorKeys = []struct {
	re    *regexp.Regexp
	key   string
	names []string
}{
	{regexp.MustCompile(`^this file looks damaged: `), "This file looks damaged.", nil},
	{regexp.MustCompile(`^image is too large \((\d+)×(\d+)\); the limit is (\d+) megapixels$`), "This image is too large ({w}×{h}); the limit is {mp} megapixels.", []string{"w", "h", "mp"}},
	{regexp.MustCompile(`^that link answered (.+)$`), "That link answered {status}.", []string{"status"}},
	{regexp.MustCompile(`^that image is larger than (\d+) MB$`), "That image is larger than {mb} MB.", []string{"mb"}},
}

// localizedError words an error from the imaging or download code for the person: a fixed message is
// looked up as it stands, one with numbers by its pattern, anything else is left as it is.
func (a *App) localizedError(ctx context.Context, msg string) string {
	for _, e := range errorKeys {
		if m := e.re.FindStringSubmatch(msg); m != nil {
			kv := make([]any, 0, 2*len(e.names))
			for i, n := range e.names {
				kv = append(kv, n, m[i+1])
			}
			return a.trc(ctx, e.key, kv...)
		}
	}
	return a.trc(ctx, msg)
}

// normalize converts HEIC/AVIF to JPEG/PNG through the AI service and
// reports which kinds of metadata the original carried.
func (a *App) normalize(ctx context.Context, data []byte) ([]byte, []string, error) {
	if a.cfg.RembgURL == "" {
		return nil, nil, userError{a.trc(ctx, "iPhone (HEIC) and AVIF photos aren't supported on this server yet; save them as JPG first.")}
	}
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, a.cfg.RembgURL+"/normalize", bytes.NewReader(data))
	req.Header.Set("Content-Type", "application/octet-stream")
	resp, err := aiClient.Do(req)
	if err != nil {
		return nil, nil, userError{a.trc(ctx, "Couldn't convert this photo right now; please try again.")}
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, nil, userError{a.trc(ctx, "This photo couldn't be read.")}
	}
	out, err := io.ReadAll(io.LimitReader(resp.Body, a.cfg.MaxUploadBytes*3))
	if err != nil {
		return nil, nil, err
	}
	var removed []string
	for _, k := range strings.Split(resp.Header.Get("X-Removed"), ",") {
		if k = strings.TrimSpace(k); k != "" {
			removed = append(removed, k)
		}
	}
	return out, removed, nil
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

// deleteImageObjects removes stored files before their database keys go away.
func (a *App) deleteImageObjects(ctx context.Context, im *db.Image) error {
	for _, k := range a.db.Variants(ctx, im.ID) {
		if err := a.store.Delete(ctx, k); err != nil {
			return fmt.Errorf("delete variant: %w", err)
		}
	}
	b := a.backends[im.Backend]
	refs := 0
	if b != nil {
		var err error
		if refs, err = a.db.CountKeyRefs(ctx, im.KeyFull); err != nil {
			return fmt.Errorf("count references: %w", err) // cannot tell whether others share the file: keep it
		}
	}
	if b != nil && refs <= 1 {
		if err := b.Delete(ctx, im.KeyFull); err != nil {
			return fmt.Errorf("delete original: %w", err)
		}
		if im.KeyThumb != im.KeyFull {
			if err := b.Delete(ctx, im.KeyThumb); err != nil {
				return fmt.Errorf("delete thumbnail: %w", err)
			}
		}
	}
	return nil
}

// removeImage deletes an image's row and its stored files.
func (a *App) removeImage(ctx context.Context, im *db.Image) error {
	// A dropped connection must not cut the removal in the middle (objects gone, row left, or the reverse).
	ctx = context.WithoutCancel(ctx)
	a.shareMu.Lock() // counting who still points at the file, deleting it and dropping the row are one step
	defer a.shareMu.Unlock()
	if err := a.deleteImageObjects(ctx, im); err != nil {
		return err
	}
	if im.UserID.Valid {
		a.fireWebhooks(im.UserID.Int64, "image.deleted", map[string]any{"id": im.ID})
	}
	return a.db.DeleteImage(ctx, im.ID)
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

// spool copies a big uploaded part to our own temporary file.
func (a *App) spool(fh *multipart.FileHeader) (string, int64, error) {
	src, err := fh.Open()
	if err != nil {
		return "", 0, err
	}
	defer src.Close()
	f, err := os.CreateTemp(a.tmpDir(), "up-")
	if err != nil {
		return "", 0, err
	}
	n, err := io.Copy(f, src)
	f.Close()
	if err != nil {
		os.Remove(f.Name())
		return "", 0, err
	}
	return f.Name(), n, nil
}

func (a *App) spoolBytes(b []byte) (string, error) {
	f, err := os.CreateTemp(a.tmpDir(), "up-")
	if err != nil {
		return "", err
	}
	_, err = f.Write(b)
	f.Close()
	if err != nil {
		os.Remove(f.Name())
		return "", err
	}
	return f.Name(), nil
}

package app

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	"io"
	"math"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/navi-crwn/pixelhop/internal/db"
	"github.com/navi-crwn/pixelhop/internal/imaging"
)

// Videos and PDFs (for example proof of a payment: a screen recording, a
// bank statement) are stored as files with a page, a poster and a link like
// images. On upload a video loses its location and other metadata (a quick
// copy, no re-encoding); afterwards a background job shrinks it to 720p
// H.264 so it plays everywhere and loads fast on phones. PDFs are
// compressed with Ghostscript right away. ffmpeg/ffprobe and gs are
// optional: without them files are kept as they are.

// mediaKind sniffs the first bytes of a file: "video", "pdf" or "".
func mediaKind(head []byte) string {
	switch {
	case bytes.HasPrefix(head, []byte("%PDF-")):
		return "pdf"
	case len(head) >= 12 && string(head[4:8]) == "ftyp" && imaging.Container(head) == "":
		return "video" // MP4, MOV, 3GP (HEIC and AVIF are photos)
	case bytes.HasPrefix(head, []byte{0x1A, 0x45, 0xDF, 0xA3}):
		return "video" // WebM, MKV
	}
	return ""
}

func (a *App) tmpDir() string {
	d := filepath.Join(a.cfg.DataDir, "tmp")
	os.MkdirAll(d, 0o700)
	return d
}

func have(tool string) bool { _, err := exec.LookPath(tool); return err == nil }

// run executes a media tool with a time limit and low priority.
func run(ctx context.Context, limit time.Duration, name string, args ...string) error {
	ctx, cancel := context.WithTimeout(ctx, limit)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if len(msg) > 300 {
			msg = msg[len(msg)-300:]
		}
		return fmt.Errorf("%s: %v: %s", name, err, msg)
	}
	return nil
}

type videoInfo struct {
	W, H     int
	Duration float64
	Codec    string
	Format   string
	Tags     map[string]string
}

func probeVideo(ctx context.Context, file string) (*videoInfo, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, "ffprobe", "-v", "error", "-print_format", "json",
		"-show_format", "-show_streams", file).Output()
	if err != nil {
		return nil, err
	}
	var p struct {
		Streams []struct {
			CodecType string            `json:"codec_type"`
			CodecName string            `json:"codec_name"`
			Width     int               `json:"width"`
			Height    int               `json:"height"`
			Tags      map[string]string `json:"tags"`
			SideData  []struct {
				Rotation float64 `json:"rotation"`
			} `json:"side_data_list"`
		} `json:"streams"`
		Format struct {
			Name     string            `json:"format_name"`
			Duration string            `json:"duration"`
			Tags     map[string]string `json:"tags"`
		} `json:"format"`
	}
	if err := json.Unmarshal(out, &p); err != nil {
		return nil, err
	}
	vi := &videoInfo{Format: p.Format.Name, Tags: p.Format.Tags}
	vi.Duration, _ = strconv.ParseFloat(p.Format.Duration, 64)
	for _, s := range p.Streams {
		if s.CodecType != "video" || s.Width == 0 {
			continue
		}
		vi.W, vi.H, vi.Codec = s.Width, s.Height, s.CodecName
		rot := 0.0
		if v, ok := s.Tags["rotate"]; ok {
			rot, _ = strconv.ParseFloat(v, 64)
		}
		for _, sd := range s.SideData {
			if sd.Rotation != 0 {
				rot = sd.Rotation
			}
		}
		if r := math.Mod(math.Abs(rot), 180); r == 90 { // phone videos filmed upright
			vi.W, vi.H = vi.H, vi.W
		}
		break
	}
	if vi.W == 0 {
		return nil, fmt.Errorf("no video stream")
	}
	return vi, nil
}

// videoMetadata lists what a video carried that we strip (in removedWords keys).
func videoMetadata(tags map[string]string) []string {
	var found []string
	has := func(subs ...string) bool {
		for k := range tags {
			lk := strings.ToLower(k)
			for _, s := range subs {
				if strings.Contains(lk, s) {
					return true
				}
			}
		}
		return false
	}
	if has("location", "gps", "iso6709") {
		found = append(found, "gps")
	}
	if has("make", "model") {
		found = append(found, "camera")
	}
	if has("creation_time", "creationdate") {
		found = append(found, "date")
	}
	if has("software", "encoder") {
		found = append(found, "software")
	}
	return found
}

func sha256File(file string) (string, int64, error) {
	f, err := os.Open(file)
	if err != nil {
		return "", 0, err
	}
	defer f.Close()
	h := sha256.New()
	n, err := io.Copy(h, f)
	return hex.EncodeToString(h.Sum(nil)), n, err
}

func fileSize(file string) int64 {
	if st, err := os.Stat(file); err == nil {
		return st.Size()
	}
	return -1
}

// placeholderThumb is used when no poster can be made.
func placeholderThumb(kind string) []byte {
	m := image.NewRGBA(image.Rect(0, 0, 480, 360))
	bg := color.RGBA{0x1b, 0x1f, 0x2a, 255}
	if kind == "pdf" {
		bg = color.RGBA{0x7f, 0x1d, 0x1d, 255}
	}
	draw.Draw(m, m.Bounds(), &image.Uniform{bg}, image.Point{}, draw.Src)
	var b bytes.Buffer
	jpeg.Encode(&b, m, &jpeg.Options{Quality: 80})
	return b.Bytes()
}

// storeMedia keeps a video or PDF (already on disk at src) and makes its page.
func (a *App) storeMedia(ctx context.Context, src, name, kind string, u *db.User, ip, albumID string, expiresAt int64, useSlug bool, alt string) (*db.Image, string, bool, error) {
	sha, _, err := sha256File(src)
	if err != nil {
		return nil, "", false, err
	}
	var uid sql.NullInt64
	if u != nil {
		uid = sql.NullInt64{Int64: u.ID, Valid: true}
		if old, err := a.db.ImageBySHA(ctx, sha, uid); err == nil {
			return old, "", true, nil
		}
	}
	work, err := os.MkdirTemp(a.tmpDir(), "media-")
	if err != nil {
		return nil, "", false, err
	}
	defer os.RemoveAll(work)

	file, mime, ext := src, "", ""
	var w, h int
	var stripped []string
	var thumb []byte
	queue := false

	switch kind {
	case "video":
		mime, ext = "video/mp4", "mp4"
		if !have("ffprobe") {
			if strings.HasPrefix(string(head(src, 4)), "\x1a\x45") {
				mime, ext = "video/webm", "webm"
			}
			break
		}
		vi, err := probeVideo(ctx, src)
		if err != nil {
			return nil, "", false, userError{name + " isn't a video we can read."}
		}
		w, h = vi.W, vi.H
		if strings.Contains(vi.Format, "webm") || strings.Contains(vi.Format, "matroska") {
			mime, ext = "video/webm", "webm"
		}
		// Drop location, dates and the like without re-encoding (seconds, not minutes).
		clean := filepath.Join(work, "clean.mp4")
		if err := run(ctx, 2*time.Minute, "ffmpeg", "-v", "error", "-y", "-i", src, "-map", "0:v:0", "-map", "0:a:0?",
			"-c", "copy", "-map_metadata", "-1", "-map_chapters", "-1", "-movflags", "+faststart", clean); err == nil {
			file, mime, ext = clean, "video/mp4", "mp4"
			stripped = videoMetadata(vi.Tags)
		} else {
			a.log.Warn("video remux", "file", name, "err", err)
		}
		at := "1"
		if vi.Duration > 0 && vi.Duration < 3 {
			at = "0"
		}
		poster := filepath.Join(work, "poster.jpg")
		if err := run(ctx, time.Minute, "ffmpeg", "-v", "error", "-y", "-ss", at, "-i", file, "-frames:v", "1",
			"-vf", "scale='min(960,iw)':-2", "-q:v", "4", poster); err == nil {
			thumb, _ = os.ReadFile(poster)
		}
		queue = have("ffmpeg")
	case "pdf":
		mime, ext = "application/pdf", "pdf"
		if !have("gs") {
			break
		}
		small := filepath.Join(work, "small.pdf")
		if err := run(ctx, 90*time.Second, "gs", "-q", "-dSAFER", "-dBATCH", "-dNOPAUSE", "-sDEVICE=pdfwrite",
			"-dCompatibilityLevel=1.5", "-dPDFSETTINGS=/ebook", "-dDetectDuplicateImages=true",
			"-sOutputFile="+small, src); err == nil {
			if n := fileSize(small); n > 0 && n < fileSize(src)*9/10 {
				file = small
			}
		}
		page := filepath.Join(work, "page1.jpg")
		if err := run(ctx, time.Minute, "gs", "-q", "-dSAFER", "-dBATCH", "-dNOPAUSE", "-sDEVICE=jpeg", "-dJPEGQ=82",
			"-r90", "-dFirstPage=1", "-dLastPage=1", "-sOutputFile="+page, file); err == nil {
			thumb, _ = os.ReadFile(page)
		}
	}

	color := "#1b1f2a"
	thumbBytes := placeholderThumb(kind)
	if len(thumb) > 0 {
		if p, err := imaging.Process(thumb, a.cfg.MaxPixels); err == nil {
			thumbBytes, color = p.Thumb, p.Color
			if w == 0 {
				w, h = p.Width, p.Height
			}
		}
	}

	size := fileSize(file)
	slug := ""
	if useSlug {
		slug = slugify(strings.TrimSuffix(name, path.Ext(name)))
	}
	im := &db.Image{
		UserID: uid, Title: trim(strings.TrimSuffix(name, path.Ext(name)), 120), OrigName: trim(name, 200),
		Mime: mime, Ext: ext, Size: size, Width: w, Height: h, SHA256: sha, Color: color,
		Backend: a.store.Name(), IP: ip, ExpiresAt: expiresAt, Stripped: strings.Join(stripped, ","), Alt: alt,
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
		im.KeyThumb = day + "/" + im.ID + "_thumb.jpg"
		f, err := os.Open(file)
		if err != nil {
			return nil, "", false, err
		}
		err = a.store.Put(ctx, im.KeyFull, f, size, im.Mime)
		f.Close()
		if err != nil {
			return nil, "", false, err
		}
		if err := a.store.Put(ctx, im.KeyThumb, bytes.NewReader(thumbBytes), int64(len(thumbBytes)), "image/jpeg"); err != nil {
			a.store.Delete(ctx, im.KeyFull)
			return nil, "", false, err
		}
		err = a.db.InsertImage(ctx, im)
		if err == nil {
			break
		}
		a.store.Delete(ctx, im.KeyFull)
		a.store.Delete(ctx, im.KeyThumb)
		if attempt >= 3 || !strings.Contains(err.Error(), "UNIQUE") {
			return nil, "", false, err
		}
	}
	if queue {
		a.db.AddMediaJob(ctx, im.ID)
		select {
		case a.mediaQueue <- im.ID:
		default: // the worker picks it up from the database later
		}
	}
	if u != nil {
		a.fireWebhooks(u.ID, "image.uploaded", a.imageJSON(im))
	}
	return im, token, false, nil
}

func head(file string, n int) []byte {
	f, err := os.Open(file)
	if err != nil {
		return nil
	}
	defer f.Close()
	b := make([]byte, n)
	m, _ := io.ReadFull(f, b)
	return b[:m]
}

// mediaWorker shrinks uploaded videos one at a time, in the background.
func (a *App) mediaWorker(ctx context.Context) {
	if !have("ffmpeg") {
		return
	}
	go func() { // jobs left over from before a restart
		for _, id := range a.db.MediaJobs(ctx) {
			select {
			case a.mediaQueue <- id:
			case <-ctx.Done():
				return
			}
		}
	}()
	for {
		select {
		case <-ctx.Done():
			return
		case id := <-a.mediaQueue:
			if err := a.optimizeVideo(ctx, id); err != nil {
				a.log.Warn("video optimise", "id", id, "err", err)
			}
			a.db.DoneMediaJob(context.Background(), id)
		}
	}
}

func (a *App) optimizeVideo(ctx context.Context, id string) error {
	im, err := a.db.Image(ctx, id)
	if err != nil || im.Kind() != "video" {
		return nil
	}
	b := a.backends[im.Backend]
	if b == nil {
		return nil
	}
	work, err := os.MkdirTemp(a.tmpDir(), "opt-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(work)
	in := filepath.Join(work, "in."+im.Ext)
	obj, err := b.Get(ctx, im.KeyFull, "")
	if err != nil {
		return err
	}
	f, err := os.Create(in)
	if err == nil {
		_, err = io.Copy(f, obj.Body)
		f.Close()
	}
	obj.Body.Close()
	if err != nil {
		return err
	}
	vi, err := probeVideo(ctx, in)
	if err != nil {
		return err
	}
	out := filepath.Join(work, "out.mp4")
	// Longest side at most 1280 px, at most 30 fps, H.264 + AAC: plays on any phone.
	scale := "scale='if(gte(iw,ih),min(1280,iw),-2)':'if(gte(iw,ih),-2,min(1280,ih))'"
	args := []string{"-n", "10", "ffmpeg", "-v", "error", "-y", "-i", in, "-map", "0:v:0", "-map", "0:a:0?",
		"-map_metadata", "-1", "-vf", scale, "-fpsmax", "30", "-c:v", "libx264", "-preset", "veryfast", "-crf", "28",
		"-pix_fmt", "yuv420p", "-threads", "2", "-c:a", "aac", "-b:a", "96k", "-ac", "2", "-movflags", "+faststart", out}
	if err := run(ctx, 45*time.Minute, "nice", args...); err != nil {
		return err
	}
	newSize := fileSize(out)
	playable := im.Mime == "video/mp4" && vi.Codec == "h264"
	if newSize <= 0 || (playable && newSize >= im.Size*9/10) {
		return nil // already small and playable: keep it
	}
	nv, err := probeVideo(ctx, out)
	if err != nil {
		return err
	}
	key := strings.TrimSuffix(im.KeyFull, path.Ext(im.KeyFull)) + "_v.mp4"
	of, err := os.Open(out)
	if err != nil {
		return err
	}
	err = b.Put(ctx, key, of, newSize, "video/mp4")
	of.Close()
	if err != nil {
		return err
	}
	if err := a.db.ReplaceFile(ctx, im.ID, key, "video/mp4", "mp4", newSize, nv.W, nv.H); err != nil {
		b.Delete(ctx, key)
		return err
	}
	if a.db.CountKeyRefs(ctx, im.KeyFull) == 0 {
		b.Delete(ctx, im.KeyFull)
	}
	a.log.Info("video optimised", "id", im.ID, "from", im.Size, "to", newSize)
	return nil
}

package app

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	xdraw "golang.org/x/image/draw"

	"github.com/navi-crwn/pixelhop/internal/db"
)

// Image transformations by URL, like a small Cloudinary:
//   /i/{id}.jpg?w=800                  fit inside 800 px wide
//   /i/{id}.jpg?w=400&h=400&fit=cover  crop to fill 400×400
//   /i/{id}.jpg?format=webp&q=75       convert (webp and avif need the AI service)
// Each variant is made once and stored; later requests are served from storage.

const maxVariants = 24 // per image, so nobody can fill the disk with sizes

var transformSlot = make(chan struct{}, 2)

func (a *App) transformRequested(r *http.Request) bool {
	q := r.URL.Query()
	return q.Has("w") || q.Has("h") || q.Has("format") || q.Has("q") || q.Has("fit")
}

type transformSpec struct {
	W, H, Q int
	Fit     string // inside | cover
	Format  string // jpeg | png | webp | avif
}

func parseTransform(r *http.Request, im *db.Image) (transformSpec, error) {
	q := r.URL.Query()
	num := func(k string, lo, hi int) (int, error) {
		if !q.Has(k) {
			return 0, nil
		}
		v, err := strconv.Atoi(q.Get(k))
		if err != nil || v < lo || v > hi {
			return 0, fmt.Errorf("%s must be between %d and %d", k, lo, hi)
		}
		return v, nil
	}
	var s transformSpec
	var err error
	if s.W, err = num("w", 16, 4096); err != nil {
		return s, err
	}
	if s.H, err = num("h", 16, 4096); err != nil {
		return s, err
	}
	if s.Q, err = num("q", 30, 95); err != nil {
		return s, err
	}
	if s.Q == 0 {
		s.Q = 82
	}
	s.Fit = "inside"
	if q.Get("fit") == "cover" {
		if s.W == 0 || s.H == 0 {
			return s, fmt.Errorf("fit=cover needs both w and h")
		}
		s.Fit = "cover"
	}
	s.Format = strings.ToLower(q.Get("format"))
	if s.Format == "jpg" {
		s.Format = "jpeg"
	}
	if s.Format == "" {
		s.Format = map[string]string{"jpg": "jpeg", "png": "png", "webp": "webp"}[im.Ext]
		if s.Format == "" {
			s.Format = "png"
		}
	}
	if s.Format != "jpeg" && s.Format != "png" && s.Format != "webp" && s.Format != "avif" {
		return s, fmt.Errorf("format is jpeg, png, webp or avif")
	}
	// Never make an image bigger than it is.
	if s.W > im.Width {
		s.W = im.Width
	}
	if s.H > im.Height {
		s.H = im.Height
	}
	return s, nil
}

func (s transformSpec) key(id string) string {
	ext := map[string]string{"jpeg": "jpg"}[s.Format]
	if ext == "" {
		ext = s.Format
	}
	return fmt.Sprintf("variants/%s/%dx%d-%s-q%d.%s", id, s.W, s.H, s.Fit, s.Q, ext)
}

func (a *App) serveTransformed(w http.ResponseWriter, r *http.Request, im *db.Image) {
	spec, err := parseTransform(r, im)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	key := spec.key(im.ID)
	mime := "image/" + spec.Format
	h := w.Header()
	h.Set("Content-Security-Policy", "default-src 'none'; sandbox")
	h.Set("Access-Control-Allow-Origin", "*")
	h.Set("Cross-Origin-Resource-Policy", "cross-origin")
	if im.Locked() || a.mediaHasWarning(r, im) {
		h.Set("Cache-Control", "private, no-store")
	} else {
		h.Set("Cache-Control", "public, max-age=2592000, immutable")
	}
	if obj, err := a.store.Get(r.Context(), key, ""); err == nil {
		defer obj.Body.Close()
		h.Set("Content-Type", mime)
		io.Copy(w, obj.Body)
		return
	}
	if len(a.db.Variants(r.Context(), im.ID)) >= maxVariants {
		http.Error(w, "this image already has the maximum number of sizes; use one of the existing ones", http.StatusTooManyRequests)
		return
	}
	if !a.limits.allow("transform:"+clientIP(r), 30, time.Minute) {
		http.Error(w, "too many new sizes at once; try again in a minute", http.StatusTooManyRequests)
		return
	}
	select {
	case transformSlot <- struct{}{}:
		defer func() { <-transformSlot }()
	case <-time.After(20 * time.Second):
		http.Error(w, "busy, try again", http.StatusServiceUnavailable)
		return
	}
	src, err := a.originalBytes(r.Context(), im)
	if err != nil {
		a.missingObject(w, r, im, err)
		return
	}
	out, err := a.makeVariant(r.Context(), src, spec)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnprocessableEntity)
		return
	}
	if err := a.store.Put(r.Context(), key, bytes.NewReader(out), int64(len(out)), mime); err == nil {
		a.db.AddVariant(r.Context(), im.ID, key, len(out))
	}
	h.Set("Content-Type", mime)
	h.Set("Content-Length", strconv.Itoa(len(out)))
	w.Write(out)
}

func (a *App) originalBytes(ctx context.Context, im *db.Image) ([]byte, error) {
	b := a.backends[im.Backend]
	if b == nil {
		return nil, fmt.Errorf("no storage")
	}
	obj, err := b.Get(ctx, im.KeyFull, "")
	if err != nil {
		return nil, err
	}
	defer obj.Body.Close()
	return io.ReadAll(io.LimitReader(obj.Body, 200<<20))
}

// makeVariant uses the AI service when it's there (any format, best
// quality); without it, JPEG and PNG are made in Go.
func (a *App) makeVariant(ctx context.Context, src []byte, s transformSpec) ([]byte, error) {
	if a.cfg.RembgURL != "" {
		u := fmt.Sprintf("%s/transform?w=%d&h=%d&fit=%s&format=%s&q=%d", a.cfg.RembgURL, s.W, s.H, s.Fit, s.Format, s.Q)
		req, _ := http.NewRequestWithContext(ctx, http.MethodPost, u, bytes.NewReader(src))
		if resp, err := aiClient.Do(req); err == nil {
			defer resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return io.ReadAll(io.LimitReader(resp.Body, 100<<20))
			}
		}
	}
	if s.Format == "webp" || s.Format == "avif" {
		return nil, fmt.Errorf("%s output needs the AI service on this server; use format=jpeg or png", s.Format)
	}
	img, _, err := image.Decode(bytes.NewReader(src))
	if err != nil {
		return nil, err
	}
	b := img.Bounds()
	dw, dh := s.W, s.H
	srcRect := b
	switch {
	case s.Fit == "cover":
		// crop the source to the target's shape, centred
		sw, sh := b.Dx(), b.Dy()
		if sw*dh > sh*dw {
			cw := sh * dw / dh
			srcRect = image.Rect(b.Min.X+(sw-cw)/2, b.Min.Y, b.Min.X+(sw-cw)/2+cw, b.Max.Y)
		} else {
			ch := sw * dh / dw
			srcRect = image.Rect(b.Min.X, b.Min.Y+(sh-ch)/2, b.Max.X, b.Min.Y+(sh-ch)/2+ch)
		}
	case dw == 0 && dh == 0:
		dw, dh = b.Dx(), b.Dy()
	case dw == 0:
		dw = max(1, b.Dx()*dh/b.Dy())
	case dh == 0:
		dh = max(1, b.Dy()*dw/b.Dx())
	default: // fit inside the box, keep the shape
		if b.Dx()*dh > b.Dy()*dw {
			dh = max(1, b.Dy()*dw/b.Dx())
		} else {
			dw = max(1, b.Dx()*dh/b.Dy())
		}
	}
	dst := image.NewRGBA(image.Rect(0, 0, dw, dh))
	xdraw.CatmullRom.Scale(dst, dst.Bounds(), img, srcRect, xdraw.Src, nil)
	var buf bytes.Buffer
	if s.Format == "png" {
		err = png.Encode(&buf, dst)
	} else {
		err = jpeg.Encode(&buf, dst, &jpeg.Options{Quality: s.Q})
	}
	return buf.Bytes(), err
}

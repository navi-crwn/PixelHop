// Package imaging inspects uploads, strips private metadata (GPS, camera
// serials) and makes thumbnails, using only Go's standard image packages.
package imaging

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/gif"
	"image/jpeg"
	"image/png"
	"runtime"

	_ "golang.org/x/image/bmp"
	xdraw "golang.org/x/image/draw"
	_ "golang.org/x/image/webp"
)

var ErrUnsupported = errors.New("unsupported image type: use JPG, PNG, GIF, WebP or BMP")

type Format struct {
	Mime string
	Ext  string
}

var formats = map[string]Format{
	"jpeg": {"image/jpeg", "jpg"},
	"png":  {"image/png", "png"},
	"gif":  {"image/gif", "gif"},
	"webp": {"image/webp", "webp"},
	"bmp":  {"image/bmp", "bmp"},
}

// Sniff identifies the image by its first bytes, never by the file name or
// the Content-Type the client claims.
func Sniff(b []byte) (string, bool) {
	switch {
	case bytes.HasPrefix(b, []byte{0xFF, 0xD8, 0xFF}):
		return "jpeg", true
	case bytes.HasPrefix(b, []byte("\x89PNG\r\n\x1a\n")):
		return "png", true
	case bytes.HasPrefix(b, []byte("GIF87a")), bytes.HasPrefix(b, []byte("GIF89a")):
		return "gif", true
	case len(b) >= 12 && string(b[0:4]) == "RIFF" && string(b[8:12]) == "WEBP":
		return "webp", true
	case bytes.HasPrefix(b, []byte("BM")):
		return "bmp", true
	}
	return "", false
}

// Processed is an upload that is ready to store.
type Processed struct {
	Format   Format
	Data     []byte // the original, with private metadata removed
	Width    int
	Height   int
	Thumb    []byte
	ThumbExt string
	Color    string // average color as #rrggbb
}

// decodeSlots caps how many large images are decoded at once, so a burst of
// uploads can't push a small server out of memory.
var decodeSlots = make(chan struct{}, max(2, runtime.NumCPU()))

const ThumbSize = 480

// Process validates data, strips its metadata and renders a thumbnail.
func Process(data []byte, maxPixels int64) (*Processed, error) {
	kind, ok := Sniff(data)
	if !ok {
		return nil, ErrUnsupported
	}
	cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("this file looks damaged: %w", err)
	}
	if cfg.Width <= 0 || cfg.Height <= 0 {
		return nil, errors.New("image has no pixels")
	}
	if int64(cfg.Width)*int64(cfg.Height) > maxPixels {
		return nil, fmt.Errorf("image is too large (%d×%d); the limit is %d megapixels", cfg.Width, cfg.Height, maxPixels/1_000_000)
	}

	p := &Processed{Format: formats[kind], Width: cfg.Width, Height: cfg.Height}

	decodeSlots <- struct{}{}
	defer func() { <-decodeSlots }()

	var img image.Image
	if kind == "gif" {
		// First frame only; animated GIFs are stored untouched.
		img, err = gif.Decode(bytes.NewReader(data))
	} else {
		img, _, err = image.Decode(bytes.NewReader(data))
	}
	if err != nil {
		return nil, fmt.Errorf("this file looks damaged: %w", err)
	}

	switch kind {
	case "jpeg":
		if o := jpegOrientation(data); o > 1 && o <= 8 {
			// Stripping EXIF would lose the rotation flag, so bake the rotation
			// into the pixels instead.
			img = orient(img, o)
			var buf bytes.Buffer
			if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 92}); err != nil {
				return nil, err
			}
			p.Data = buf.Bytes()
			p.Width, p.Height = img.Bounds().Dx(), img.Bounds().Dy()
		} else {
			p.Data = stripJPEG(data)
		}
	case "png":
		p.Data = stripPNG(data)
	case "webp":
		p.Data = stripWebP(data)
	case "bmp":
		// BMP is bulky and carries no metadata; store it as lossless PNG.
		var buf bytes.Buffer
		if err := png.Encode(&buf, img); err != nil {
			return nil, err
		}
		p.Data, p.Format = buf.Bytes(), formats["png"]
	default:
		p.Data = data
	}

	thumb := Fit(img, ThumbSize)
	p.Color = average(thumb)
	var buf bytes.Buffer
	if opaque(thumb) {
		err = jpeg.Encode(&buf, thumb, &jpeg.Options{Quality: 80})
		p.ThumbExt = "jpg"
	} else {
		err = (&png.Encoder{CompressionLevel: png.BestSpeed}).Encode(&buf, thumb)
		p.ThumbExt = "png"
	}
	if err != nil {
		return nil, err
	}
	p.Thumb = buf.Bytes()
	return p, nil
}

// Fit scales img down to fit in a size×size box (never up).
func Fit(img image.Image, size int) image.Image {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	if w <= size && h <= size {
		dst := image.NewRGBA(image.Rect(0, 0, w, h))
		draw.Draw(dst, dst.Bounds(), img, b.Min, draw.Src)
		return dst
	}
	if w >= h {
		h = max(1, h*size/w)
		w = size
	} else {
		w = max(1, w*size/h)
		h = size
	}
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	xdraw.CatmullRom.Scale(dst, dst.Bounds(), img, b, draw.Src, nil)
	return dst
}

func opaque(img image.Image) bool {
	if o, ok := img.(interface{ Opaque() bool }); ok {
		return o.Opaque()
	}
	return false
}

func average(img image.Image) string {
	b := img.Bounds()
	var r, g, bl, n uint64
	step := max(1, b.Dx()/32)
	for y := b.Min.Y; y < b.Max.Y; y += step {
		for x := b.Min.X; x < b.Max.X; x += step {
			c := color.NRGBAModel.Convert(img.At(x, y)).(color.NRGBA)
			if c.A < 16 {
				continue
			}
			r, g, bl, n = r+uint64(c.R), g+uint64(c.G), bl+uint64(c.B), n+1
		}
	}
	if n == 0 {
		return "#808080"
	}
	return fmt.Sprintf("#%02x%02x%02x", r/n, g/n, bl/n)
}

// orient applies an EXIF orientation (2..8) to img.
func orient(img image.Image, o int) image.Image {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	var dst *image.RGBA
	if o >= 5 {
		dst = image.NewRGBA(image.Rect(0, 0, h, w))
	} else {
		dst = image.NewRGBA(image.Rect(0, 0, w, h))
	}
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			var dx, dy int
			switch o {
			case 2:
				dx, dy = w-1-x, y
			case 3:
				dx, dy = w-1-x, h-1-y
			case 4:
				dx, dy = x, h-1-y
			case 5:
				dx, dy = y, x
			case 6:
				dx, dy = h-1-y, x
			case 7:
				dx, dy = h-1-y, w-1-x
			case 8:
				dx, dy = y, w-1-x
			}
			dst.Set(dx, dy, img.At(b.Min.X+x, b.Min.Y+y))
		}
	}
	return dst
}

// Avatar turns any supported image into a square JPEG profile photo of
// size×size, centred. Re-encoding drops every bit of metadata.
func Avatar(data []byte, maxPixels int64, size int) ([]byte, error) {
	if _, ok := Sniff(data); !ok {
		return nil, ErrUnsupported
	}
	cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("this file looks damaged: %w", err)
	}
	if int64(cfg.Width)*int64(cfg.Height) > maxPixels {
		return nil, errors.New("that image is too large for a profile photo")
	}
	decodeSlots <- struct{}{}
	defer func() { <-decodeSlots }()
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("this file looks damaged: %w", err)
	}
	if o := jpegOrientation(data); o > 1 && o <= 8 {
		img = orient(img, o)
	}
	b := img.Bounds()
	side := min(b.Dx(), b.Dy())
	x0, y0 := b.Min.X+(b.Dx()-side)/2, b.Min.Y+(b.Dy()-side)/2
	dst := image.NewRGBA(image.Rect(0, 0, size, size))
	draw.Draw(dst, dst.Bounds(), image.White, image.Point{}, draw.Src) // transparent PNGs get a white background
	xdraw.CatmullRom.Scale(dst, dst.Bounds(), img, image.Rect(x0, y0, x0+side, y0+side), draw.Over, nil)
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, dst, &jpeg.Options{Quality: 88}); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

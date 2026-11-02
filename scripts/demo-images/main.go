package main

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/color"
	"image/jpeg"
	"math"
	"os"
)

type pal struct {
	top, bot, sun color.RGBA
	ridges        []color.RGBA
	sunX, sunY    float64
}

func mix(a, b color.RGBA, t float64) color.RGBA {
	f := func(x, y uint8) uint8 { return uint8(float64(x)*(1-t) + float64(y)*t) }
	return color.RGBA{f(a.R, b.R), f(a.G, b.G), f(a.B, b.B), 255}
}

func render(p pal, seed float64, w, h int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		c := mix(p.top, p.bot, math.Pow(float64(y)/float64(h), 0.8))
		for x := 0; x < w; x++ {
			dx, dy := float64(x)-p.sunX*float64(w), float64(y)-p.sunY*float64(h)
			d := math.Sqrt(dx*dx+dy*dy) / float64(h)
			cc := c
			if d < 0.07 {
				cc = p.sun
			} else if d < 0.35 {
				cc = mix(c, p.sun, math.Pow(1-(d-0.07)/0.28, 3)*0.55)
			}
			img.SetRGBA(x, y, cc)
		}
	}
	for i, rc := range p.ridges {
		base := float64(h) * (0.52 + 0.12*float64(i))
		amp := float64(h) * (0.16 - 0.03*float64(i))
		for x := 0; x < w; x++ {
			fx := float64(x) / float64(w)
			ry := base - amp*(0.5*math.Sin(fx*5.1+seed+float64(i)*1.7)+0.3*math.Sin(fx*11.3+seed*2+float64(i))+0.2*math.Sin(fx*23+seed*3))
			for y := int(ry); y < h; y++ {
				if y >= 0 {
					img.SetRGBA(x, y, mix(rc, p.bot, float64(y-int(ry))/float64(h)*0.25))
				}
			}
		}
	}
	return img
}

func exif() []byte {
	// Little-endian TIFF: IFD0 with Make, Model, DateTime, GPS pointer.
	le := binary.LittleEndian
	var b bytes.Buffer
	b.WriteString("II*\x00")
	binary.Write(&b, le, uint32(8))
	entries := []struct {
		tag, typ uint16
		cnt, val uint32
	}{{0x010F, 2, 4, 0}, {0x0110, 2, 4, 0}, {0x0132, 2, 4, 0}, {0x8825, 4, 1, 0}}
	binary.Write(&b, le, uint16(len(entries)))
	for _, e := range entries {
		binary.Write(&b, le, e.tag)
		binary.Write(&b, le, e.typ)
		binary.Write(&b, le, e.cnt)
		binary.Write(&b, le, e.val)
	}
	binary.Write(&b, le, uint32(0))
	body := append([]byte("Exif\x00\x00"), b.Bytes()...)
	n := len(body) + 2
	return append([]byte{0xFF, 0xE1, byte(n >> 8), byte(n)}, body...)
}

func rgb(r, g, b uint8) color.RGBA { return color.RGBA{r, g, b, 255} }

// Writes six synthetic landscapes (no real photos) into the folder given as the
// first argument. Two carry a made-up EXIF block with a GPS pointer, a camera
// make and a date, so the README screenshots can show PixelHop removing them.
// Run: go run ./scripts/demo-images DIR
func main() {
	if len(os.Args) < 2 {
		os.Stderr.WriteString("usage: go run ./scripts/demo-images DIR\n")
		os.Exit(2)
	}
	out := os.Args[1]
	os.MkdirAll(out, 0o755)
	sets := []struct {
		name string
		p    pal
		seed float64
		exif bool
	}{
		{"harbour-at-dusk", pal{rgb(40, 40, 90), rgb(240, 150, 110), rgb(255, 230, 170), []color.RGBA{rgb(70, 60, 110), rgb(45, 40, 85), rgb(25, 25, 60)}, 0.7, 0.55}, 1.1, true},
		{"mountain-ridge", pal{rgb(90, 150, 210), rgb(230, 235, 245), rgb(255, 250, 220), []color.RGBA{rgb(120, 140, 175), rgb(80, 100, 140), rgb(40, 60, 90)}, 0.25, 0.3}, 2.4, true},
		{"desert-dunes", pal{rgb(240, 170, 90), rgb(250, 225, 170), rgb(255, 245, 210), []color.RGBA{rgb(215, 140, 70), rgb(185, 110, 55), rgb(140, 80, 45)}, 0.8, 0.35}, 3.3, false},
		{"forest-lake", pal{rgb(120, 170, 160), rgb(215, 235, 215), rgb(255, 255, 230), []color.RGBA{rgb(90, 140, 120), rgb(50, 100, 85), rgb(25, 65, 55)}, 0.5, 0.25}, 4.2, false},
		{"night-city", pal{rgb(15, 15, 45), rgb(90, 60, 130), rgb(240, 240, 255), []color.RGBA{rgb(45, 35, 85), rgb(28, 22, 60), rgb(15, 12, 35)}, 0.15, 0.2}, 5.7, false},
		{"autumn-hills", pal{rgb(110, 130, 190), rgb(250, 205, 150), rgb(255, 240, 200), []color.RGBA{rgb(200, 120, 70), rgb(160, 85, 50), rgb(110, 55, 40)}, 0.6, 0.4}, 6.6, false},
	}
	for _, s := range sets {
		img := render(s.p, s.seed, 1600, 1000)
		var buf bytes.Buffer
		jpeg.Encode(&buf, img, &jpeg.Options{Quality: 88})
		b := buf.Bytes()
		if s.exif {
			b = append(append([]byte{0xFF, 0xD8}, exif()...), b[2:]...)
		}
		os.WriteFile(out+"/"+s.name+".jpg", b, 0o644)
	}
}

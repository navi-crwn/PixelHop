package imaging

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/color"
	"image/jpeg"
	"testing"
)

// exifSegment builds an APP1 Exif segment holding one tag: orientation.
func exifSegment(orientation uint16) []byte {
	tiff := []byte("II*\x00\x08\x00\x00\x00") // little-endian header, IFD at 8
	ifd := make([]byte, 2+12+4)
	binary.LittleEndian.PutUint16(ifd[0:], 1)      // one entry
	binary.LittleEndian.PutUint16(ifd[2:], 0x0112) // Orientation
	binary.LittleEndian.PutUint16(ifd[4:], 3)      // SHORT
	binary.LittleEndian.PutUint32(ifd[6:], 1)      // count
	binary.LittleEndian.PutUint16(ifd[10:], orientation)
	body := append([]byte("Exif\x00\x00"), append(tiff, ifd...)...)
	body = append(body, []byte("GPS-SECRET")...)
	seg := []byte{0xFF, 0xE1, 0, 0}
	binary.BigEndian.PutUint16(seg[2:], uint16(len(body)+2))
	return append(seg, body...)
}

func sampleJPEG(t *testing.T, w, h int) []byte {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for x := 0; x < w/2; x++ {
		for y := 0; y < h; y++ {
			img.Set(x, y, color.RGBA{255, 0, 0, 255})
		}
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, nil); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func withExif(j []byte, o uint16) []byte {
	return append(append([]byte{0xFF, 0xD8}, exifSegment(o)...), j[2:]...)
}

func TestStripJPEGRemovesExifLosslessly(t *testing.T) {
	plain := sampleJPEG(t, 40, 20)
	tagged := withExif(plain, 1)
	if got := jpegOrientation(tagged); got != 1 {
		t.Fatalf("orientation = %d", got)
	}
	out := stripJPEG(tagged)
	if bytes.Contains(out, []byte("GPS-SECRET")) || bytes.Contains(out, []byte("Exif")) {
		t.Fatal("EXIF survived")
	}
	if !bytes.Equal(out, plain) {
		t.Fatal("stripping changed more than the metadata")
	}
}

func TestProcessAppliesOrientation(t *testing.T) {
	tagged := withExif(sampleJPEG(t, 40, 20), 6) // rotate 90° clockwise
	p, err := Process(tagged, 1e6)
	if err != nil {
		t.Fatal(err)
	}
	if p.Width != 20 || p.Height != 40 {
		t.Fatalf("got %dx%d, want 20x40", p.Width, p.Height)
	}
	if bytes.Contains(p.Data, []byte("GPS-SECRET")) {
		t.Fatal("EXIF survived")
	}
	img, err := jpeg.Decode(bytes.NewReader(p.Data))
	if err != nil {
		t.Fatal(err)
	}
	// The red left half must now be the top half.
	if r, _, _, _ := img.At(10, 5).RGBA(); r>>8 < 200 {
		t.Fatalf("top is not red after rotation: r=%d", r>>8)
	}
}

func TestProcessRejectsBombsAndJunk(t *testing.T) {
	if _, err := Process(sampleJPEG(t, 400, 400), 1000); err == nil {
		t.Fatal("expected the pixel limit to trip")
	}
	if _, err := Process([]byte("<svg onload=alert(1)>"), 1e6); err != ErrUnsupported {
		t.Fatalf("err = %v", err)
	}
}

func TestStripPNG(t *testing.T) {
	sig := []byte("\x89PNG\r\n\x1a\n")
	chunk := func(typ, data string) []byte {
		b := make([]byte, 8)
		binary.BigEndian.PutUint32(b, uint32(len(data)))
		copy(b[4:], typ)
		return append(append(b, data...), 0, 0, 0, 0)
	}
	in := append(append(append([]byte{}, sig...), chunk("IHDR", "0123456789abc")...), chunk("tEXt", "Comment\x00secret")...)
	in = append(in, chunk("IEND", "")...)
	out := stripPNG(in)
	if bytes.Contains(out, []byte("secret")) || !bytes.Contains(out, []byte("IHDR")) || !bytes.Contains(out, []byte("IEND")) {
		t.Fatal("wrong chunks kept")
	}
}

func TestStripWebP(t *testing.T) {
	chunk := func(typ string, data []byte) []byte {
		b := make([]byte, 8)
		copy(b, typ)
		binary.LittleEndian.PutUint32(b[4:], uint32(len(data)))
		b = append(b, data...)
		if len(data)%2 == 1 {
			b = append(b, 0)
		}
		return b
	}
	vp8x := make([]byte, 10)
	vp8x[0] = 0x08 | 0x04
	body := append([]byte("WEBP"), chunk("VP8X", vp8x)...)
	body = append(body, chunk("VP8L", []byte("pixels"))...)
	body = append(body, chunk("EXIF", []byte("GPS-SECRET"))...)
	riff := append([]byte("RIFF\x00\x00\x00\x00"), body...)
	binary.LittleEndian.PutUint32(riff[4:], uint32(len(riff)-8))
	out := stripWebP(riff)
	if bytes.Contains(out, []byte("GPS-SECRET")) {
		t.Fatal("EXIF survived")
	}
	if out[20]&0x0C != 0 {
		t.Fatal("metadata flags not cleared")
	}
	if int(binary.LittleEndian.Uint32(out[4:])) != len(out)-8 {
		t.Fatal("RIFF size wrong")
	}
}

func TestMetadataFound(t *testing.T) {
	tagged := withExif(sampleJPEG(t, 10, 10), 1) // orientation only
	if got := MetadataFound(tagged); len(got) != 0 {
		t.Fatalf("orientation alone reported as %v", got)
	}
	// IFD0 with Make (0x010F) and GPSInfo (0x8825)
	tiff := []byte("II*\x00\x08\x00\x00\x00")
	ifd := make([]byte, 2+24+4)
	binary.LittleEndian.PutUint16(ifd[0:], 2)
	binary.LittleEndian.PutUint16(ifd[2:], 0x010F)
	binary.LittleEndian.PutUint16(ifd[14:], 0x8825)
	body := append([]byte("Exif\x00\x00"), append(tiff, ifd...)...)
	seg := []byte{0xFF, 0xE1, 0, 0}
	binary.BigEndian.PutUint16(seg[2:], uint16(len(body)+2))
	j := sampleJPEG(t, 10, 10)
	in := append(append(append([]byte{0xFF, 0xD8}, seg...), body...), j[2:]...)
	got := MetadataFound(in)
	if len(got) != 2 || got[0] != "gps" || got[1] != "camera" {
		t.Fatalf("got %v", got)
	}
	if Container([]byte("\x00\x00\x00\x18ftypheic\x00\x00")) != "heic" || Container([]byte("\x00\x00\x00\x18ftypavif\x00\x00")) != "avif" {
		t.Fatal("container sniffing")
	}
}

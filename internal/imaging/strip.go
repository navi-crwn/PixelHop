package imaging

import (
	"bytes"
	"encoding/binary"
)

// Metadata stripping works on the file's container structure, so the pixels
// are never re-encoded and the image keeps its exact quality.

// stripJPEG drops EXIF/XMP (APP1), IPTC (APP13) and comments. ICC colour
// profiles (APP2) and Adobe markers (APP14) are kept so colours stay right.
func stripJPEG(b []byte) []byte {
	if len(b) < 4 || b[0] != 0xFF || b[1] != 0xD8 {
		return b
	}
	out := make([]byte, 0, len(b))
	out = append(out, 0xFF, 0xD8)
	i := 2
	for i+4 <= len(b) {
		if b[i] != 0xFF {
			return b // not where a marker should be: leave the file alone
		}
		m := b[i+1]
		if m == 0xFF { // fill byte
			i++
			continue
		}
		if m == 0xDA || m == 0xD9 { // start of scan / end: copy the rest verbatim
			return append(out, b[i:]...)
		}
		if m >= 0xD0 && m <= 0xD7 || m == 0x01 { // markers without a length
			out = append(out, b[i:i+2]...)
			i += 2
			continue
		}
		n := int(binary.BigEndian.Uint16(b[i+2:]))
		end := i + 2 + n
		if n < 2 || end > len(b) {
			return b
		}
		if m != 0xE1 && m != 0xED && m != 0xFE {
			out = append(out, b[i:end]...)
		}
		i = end
	}
	return b
}

// jpegOrientation reads the EXIF orientation tag (1 when absent).
func jpegOrientation(b []byte) int {
	i := 2
	for i+4 <= len(b) && b[i] == 0xFF {
		m := b[i+1]
		if m == 0xDA {
			break
		}
		n := int(binary.BigEndian.Uint16(b[i+2:]))
		end := i + 2 + n
		if n < 2 || end > len(b) {
			break
		}
		seg := b[i+4 : end]
		if m == 0xE1 && bytes.HasPrefix(seg, []byte("Exif\x00\x00")) {
			return tiffOrientation(seg[6:])
		}
		i = end
	}
	return 1
}

func tiffOrientation(t []byte) int {
	if len(t) < 8 {
		return 1
	}
	var bo binary.ByteOrder
	switch string(t[:2]) {
	case "II":
		bo = binary.LittleEndian
	case "MM":
		bo = binary.BigEndian
	default:
		return 1
	}
	off := int(bo.Uint32(t[4:]))
	if off+2 > len(t) {
		return 1
	}
	count := int(bo.Uint16(t[off:]))
	for k := 0; k < count; k++ {
		e := off + 2 + k*12
		if e+12 > len(t) {
			return 1
		}
		if bo.Uint16(t[e:]) == 0x0112 {
			return int(bo.Uint16(t[e+8:]))
		}
	}
	return 1
}

// stripPNG drops text, EXIF and timestamp chunks.
func stripPNG(b []byte) []byte {
	const sig = 8
	if len(b) < sig {
		return b
	}
	out := make([]byte, 0, len(b))
	out = append(out, b[:sig]...)
	i := sig
	for i+12 <= len(b) {
		n := int(binary.BigEndian.Uint32(b[i:]))
		end := i + 12 + n
		if n < 0 || end > len(b) {
			return b
		}
		switch string(b[i+4 : i+8]) {
		case "eXIf", "tEXt", "zTXt", "iTXt", "tIME":
		default:
			out = append(out, b[i:end]...)
		}
		i = end
	}
	if i != len(b) {
		return b
	}
	return out
}

// stripWebP drops EXIF and XMP chunks from extended (VP8X) WebP files and
// clears their flags.
func stripWebP(b []byte) []byte {
	if len(b) < 21 || string(b[12:16]) != "VP8X" {
		return b // simple WebP files can't carry metadata
	}
	out := make([]byte, 12, len(b))
	copy(out, b[:12])
	i := 12
	for i+8 <= len(b) {
		n := int(binary.LittleEndian.Uint32(b[i+4:]))
		end := i + 8 + n + n%2
		if end > len(b) {
			if i+8+n == len(b) { // missing pad byte on the last chunk
				end = len(b)
			} else {
				return b
			}
		}
		switch string(b[i : i+4]) {
		case "EXIF", "XMP ":
		case "VP8X":
			chunk := append([]byte(nil), b[i:end]...)
			chunk[8] &^= 0x08 | 0x04 // EXIF and XMP flags
			out = append(out, chunk...)
		default:
			out = append(out, b[i:end]...)
		}
		i = end
	}
	binary.LittleEndian.PutUint32(out[4:], uint32(len(out)-8))
	return out
}

// Container recognises formats Go can't decode but the AI service can turn
// into JPEG/PNG: iPhone photos (HEIC/HEIF) and AVIF.
func Container(b []byte) string {
	if len(b) < 12 || string(b[4:8]) != "ftyp" {
		return ""
	}
	switch string(b[8:12]) {
	case "heic", "heix", "hevc", "hevx", "heim", "heis", "mif1", "msf1":
		return "heic"
	case "avif", "avis":
		return "avif"
	}
	return ""
}

// MetadataFound lists the kinds of private data a file carries, by name only
// (gps, camera, date, software, xmp, iptc, comment, text), so the uploader can
// be told what was removed. Values are never read out.
func MetadataFound(b []byte) []string {
	found := map[string]bool{}
	kind, _ := Sniff(b)
	switch kind {
	case "jpeg":
		for i := 2; i+4 <= len(b) && b[i] == 0xFF; {
			m := b[i+1]
			if m == 0xDA || m == 0xD9 {
				break
			}
			n := int(binary.BigEndian.Uint16(b[i+2:]))
			end := i + 2 + n
			if n < 2 || end > len(b) {
				break
			}
			seg := b[i+4 : end]
			switch {
			case m == 0xE1 && bytes.HasPrefix(seg, []byte("Exif\x00\x00")):
				tiffKinds(seg[6:], found)
			case m == 0xE1 && bytes.HasPrefix(seg, []byte("http://ns.adobe.com/xap/")):
				found["xmp"] = true
			case m == 0xED:
				found["iptc"] = true
			case m == 0xFE:
				found["comment"] = true
			}
			i = end
		}
	case "png":
		for i := 8; i+12 <= len(b); {
			n := int(binary.BigEndian.Uint32(b[i:]))
			end := i + 12 + n
			if end > len(b) || n < 0 {
				break
			}
			switch typ := string(b[i+4 : i+8]); typ {
			case "eXIf":
				tiffKinds(b[i+8:i+8+n], found)
			case "tEXt", "zTXt", "iTXt":
				if bytes.HasPrefix(b[i+8:i+8+n], []byte("XML:com.adobe.xmp")) {
					found["xmp"] = true
				} else {
					found["text"] = true
				}
			case "tIME":
				found["date"] = true
			}
			i = end
		}
	case "webp":
		for i := 12; i+8 <= len(b); {
			n := int(binary.LittleEndian.Uint32(b[i+4:]))
			end := i + 8 + n + n%2
			if i+8+n > len(b) {
				break
			}
			switch string(b[i : i+4]) {
			case "EXIF":
				data := b[i+8 : i+8+n]
				tiffKinds(bytes.TrimPrefix(data, []byte("Exif\x00\x00")), found)
			case "XMP ":
				found["xmp"] = true
			}
			i = end
		}
	}
	var out []string
	for _, k := range []string{"gps", "camera", "date", "software", "xmp", "iptc", "comment", "text"} {
		if found[k] {
			out = append(out, k)
		}
	}
	return out
}

// tiffKinds looks at IFD0 (and the Exif sub-IFD for the capture date).
func tiffKinds(t []byte, found map[string]bool) {
	if len(t) < 8 {
		return
	}
	var bo binary.ByteOrder = binary.LittleEndian
	if string(t[:2]) == "MM" {
		bo = binary.BigEndian
	} else if string(t[:2]) != "II" {
		return
	}
	walk := func(off int, sub bool) int {
		exifIFD := 0
		if off+2 > len(t) {
			return 0
		}
		count := int(bo.Uint16(t[off:]))
		for k := 0; k < count; k++ {
			e := off + 2 + k*12
			if e+12 > len(t) {
				break
			}
			switch tag := bo.Uint16(t[e:]); tag {
			case 0x8825:
				found["gps"] = true
			case 0x010F, 0x0110, 0xA431, 0xA434: // make, model, body & lens serials
				found["camera"] = true
			case 0x0132, 0x9003, 0x9004:
				found["date"] = true
			case 0x0131:
				found["software"] = true
			case 0x8769:
				if !sub {
					exifIFD = int(bo.Uint32(t[e+8:]))
				}
			}
		}
		return exifIFD
	}
	if sub := walk(int(bo.Uint32(t[4:])), false); sub > 0 {
		walk(sub, true)
	}
}

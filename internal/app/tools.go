package app

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"mime/multipart"
	"net/http"
	"strconv"
	"strings"
	"time"

	"rsc.io/qr"

	"github.com/navi-crwn/pixelhop/internal/imaging"
)

// AI tools that run on the server. PixelHop forwards the image to a small
// local service (tools/ai-server.py) that keeps the models in memory and works
// on one image at a time. Nothing is stored: results go straight back.

type aiTool struct {
	path     string        // endpoint on the AI service
	perDay   int           // per person
	wait     time.Duration // how long to queue before saying "busy"
	maxPix   int64         // input size limit (0 = none)
	needMask bool
}

// Cloudflare cuts requests at 100 s, so queue time + work must stay below it.
var aiTools = map[string]aiTool{
	"remove-background": {path: "/remove", perDay: 30, wait: 60 * time.Second},
	"faces":             {path: "/faces", perDay: 100, wait: 60 * time.Second},
	"erase":             {path: "/erase", perDay: 30, wait: 45 * time.Second, needMask: true},
	"upscale":           {path: "/upscale", perDay: 10, wait: 20 * time.Second, maxPix: 1_000_000},
}

// aiSlot lets one image through the models at a time; others wait in line.
var aiSlot = make(chan struct{}, 1)

var aiClient = &http.Client{Timeout: 3 * time.Minute}

func (a *App) apiAITool(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("tool")
	tool, ok := aiTools[name]
	if !ok {
		apiError(w, http.StatusNotFound, "unknown tool")
		return
	}
	if a.cfg.RembgURL == "" {
		apiError(w, http.StatusNotFound, "server tools are not set up here")
		return
	}
	u := currentUser(r)
	if u == nil {
		apiError(w, http.StatusUnauthorized, "sign in to use the server tools")
		return
	}
	if !a.limits.allow("ai-min:"+clientIP(r), 8, time.Minute) {
		apiError(w, http.StatusTooManyRequests, "one moment: too many in a row")
		return
	}
	perDay := tool.perDay
	if name == "remove-background" {
		perDay = a.cfg.RembgPerDay
	}
	if !a.limits.allow("ai-day:"+name+":"+strconv.FormatInt(u.ID, 10), perDay, 24*time.Hour) {
		apiError(w, http.StatusTooManyRequests, fmt.Sprintf("you've used today's %d of this tool; try again tomorrow", perDay))
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 2*a.cfg.MaxUploadBytes+(1<<20))
	img, err := formBytes(r, "file", a.cfg.MaxUploadBytes)
	if err != nil {
		apiError(w, http.StatusBadRequest, err.Error())
		return
	}
	if _, ok := imaging.Sniff(img); !ok {
		apiError(w, http.StatusBadRequest, imaging.ErrUnsupported.Error())
		return
	}
	if tool.maxPix > 0 {
		if cfg, _, err := image.DecodeConfig(bytes.NewReader(img)); err == nil && int64(cfg.Width)*int64(cfg.Height) > tool.maxPix {
			apiError(w, http.StatusRequestEntityTooLarge, fmt.Sprintf("this image is %d×%d; resize it to about %d megapixel first", cfg.Width, cfg.Height, tool.maxPix/1_000_000))
			return
		}
	}
	var mask []byte
	if tool.needMask {
		if mask, err = formBytes(r, "mask", a.cfg.MaxUploadBytes); err != nil {
			apiError(w, http.StatusBadRequest, "paint over what to erase first")
			return
		}
	}

	wait, cancel := context.WithTimeout(r.Context(), tool.wait)
	defer cancel()
	select {
	case aiSlot <- struct{}{}:
		defer func() { <-aiSlot }()
	case <-wait.Done():
		apiError(w, http.StatusServiceUnavailable, "the server is busy with other images; please try again in a minute")
		return
	}

	var body io.Reader = bytes.NewReader(img)
	ctype := "application/octet-stream"
	if tool.needMask {
		var buf bytes.Buffer
		mw := multipart.NewWriter(&buf)
		for field, data := range map[string][]byte{"image": img, "mask": mask} {
			fw, _ := mw.CreateFormFile(field, field)
			fw.Write(data)
		}
		mw.Close()
		body, ctype = &buf, mw.FormDataContentType()
	}
	req, _ := http.NewRequestWithContext(r.Context(), http.MethodPost, a.cfg.RembgURL+tool.path, body)
	req.Header.Set("Content-Type", ctype)
	resp, err := aiClient.Do(req)
	if err != nil {
		a.log.Error("ai tool", "tool", name, "err", err)
		apiError(w, http.StatusBadGateway, "this tool is unavailable right now")
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		msg, _ := io.ReadAll(io.LimitReader(resp.Body, 300))
		a.log.Error("ai tool", "tool", name, "status", resp.Status, "body", string(msg))
		if resp.StatusCode == http.StatusUnprocessableEntity {
			apiError(w, http.StatusUnprocessableEntity, string(msg))
			return
		}
		apiError(w, http.StatusBadGateway, "the tool couldn't process this image")
		return
	}
	a.db.CountUsage(r.Context(), name)
	w.Header().Set("Content-Type", resp.Header.Get("Content-Type"))
	w.Header().Set("Cache-Control", "no-store")
	io.Copy(w, io.LimitReader(resp.Body, 300<<20))
}

func formBytes(r *http.Request, field string, limit int64) ([]byte, error) {
	f, _, err := r.FormFile(field)
	if err != nil {
		return nil, fmt.Errorf("send the image in a “%s” field", field)
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, limit+1))
	if err != nil || int64(len(data)) > limit {
		return nil, fmt.Errorf("images up to %d MB", limit>>20)
	}
	return data, nil
}

// ---- QR codes for any text or link ----

// qrTool draws a QR code: /tools/qr.png or /tools/qr.svg
// ?text=…&size=512&level=M&fg=000000&bg=ffffff (bg "transparent" allowed)
func (a *App) qrTool(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	text := q.Get("text")
	if text == "" || len(text) > 1500 {
		http.Error(w, "text must be 1 to 1500 characters", http.StatusBadRequest)
		return
	}
	level := map[string]qr.Level{"L": qr.L, "M": qr.M, "Q": qr.Q, "H": qr.H}[q.Get("level")]
	if q.Get("level") == "" {
		level = qr.M
	}
	code, err := qr.Encode(text, level)
	if err != nil {
		http.Error(w, "that text is too long for a QR code", http.StatusBadRequest)
		return
	}
	fg, ok1 := hexColor(q.Get("fg"), color.RGBA{0, 0, 0, 255})
	bg, ok2 := hexColor(q.Get("bg"), color.RGBA{255, 255, 255, 255})
	if !ok1 || !ok2 {
		http.Error(w, "colors are 6-digit hex like 1a2b3c", http.StatusBadRequest)
		return
	}
	const quiet = 4
	n := code.Size + 2*quiet
	w.Header().Set("Cache-Control", "public, max-age=86400")
	if strings.HasSuffix(r.URL.Path, ".svg") {
		var b strings.Builder
		fmt.Fprintf(&b, `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 %d %d" shape-rendering="crispEdges">`, n, n)
		if bg.A > 0 {
			fmt.Fprintf(&b, `<rect width="%d" height="%d" fill="#%02x%02x%02x"/>`, n, n, bg.R, bg.G, bg.B)
		}
		fmt.Fprintf(&b, `<path fill="#%02x%02x%02x" d="`, fg.R, fg.G, fg.B)
		for y := 0; y < code.Size; y++ {
			for x := 0; x < code.Size; x++ {
				if code.Black(x, y) {
					fmt.Fprintf(&b, "M%d %dh1v1h-1z", x+quiet, y+quiet)
				}
			}
		}
		b.WriteString(`"/></svg>`)
		w.Header().Set("Content-Type", "image/svg+xml")
		w.Write([]byte(b.String()))
		return
	}
	size, _ := strconv.Atoi(q.Get("size"))
	size = min(max(size, 128), 2048)
	scale := max(1, size/n)
	img := image.NewNRGBA(image.Rect(0, 0, n*scale, n*scale))
	for y := 0; y < n*scale; y++ {
		for x := 0; x < n*scale; x++ {
			mx, my := x/scale-quiet, y/scale-quiet
			if mx >= 0 && my >= 0 && mx < code.Size && my < code.Size && code.Black(mx, my) {
				img.Set(x, y, fg)
			} else {
				img.Set(x, y, bg)
			}
		}
	}
	w.Header().Set("Content-Type", "image/png")
	png.Encode(w, img)
}

func hexColor(s string, def color.RGBA) (color.RGBA, bool) {
	s = strings.TrimPrefix(s, "#")
	switch {
	case s == "":
		return def, true
	case s == "transparent":
		return color.RGBA{}, true
	case len(s) != 6:
		return def, false
	}
	v, err := strconv.ParseUint(s, 16, 32)
	if err != nil {
		return def, false
	}
	return color.RGBA{uint8(v >> 16), uint8(v >> 8), uint8(v), 255}, true
}

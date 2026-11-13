package app

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"io"
	"mime/multipart"
	"net/http"
	"strconv"
	"time"

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
		apiError(w, http.StatusTooManyRequests, a.tr(r, "one moment: too many in a row"))
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

	// The day's share counts only what gets a result: it is taken here, after the checks above, and given
	// back below when the queue or the AI service fails. Taking it first keeps racing requests from going
	// over the number; giving back only touches the window it was taken from.
	perDay := tool.perDay
	if name == "remove-background" {
		perDay = a.cfg.RembgPerDay
	}
	dayKey, takenAt := "ai-day:"+name+":"+strconv.FormatInt(u.ID, 10), time.Now()
	if !a.limits.allowAt(dayKey, perDay, 24*time.Hour, takenAt) {
		apiError(w, http.StatusTooManyRequests, a.tr(r, "you've used today's {n} of this tool; try again tomorrow", "n", perDay))
		return
	}
	giveBack := func() { a.limits.give(dayKey, takenAt) }

	wait, cancel := context.WithTimeout(r.Context(), tool.wait)
	defer cancel()
	select {
	case aiSlot <- struct{}{}:
		defer func() { <-aiSlot }()
	case <-wait.Done():
		giveBack()
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
	if err != nil { // service down, timeout, or the caller went away
		giveBack()
		a.log.Error("ai tool", "tool", name, "err", err)
		apiError(w, http.StatusBadGateway, "this tool is unavailable right now")
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		giveBack()
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

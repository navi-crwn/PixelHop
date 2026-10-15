package app

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/navi-crwn/pixelhop/internal/imaging"
)

// Background removal on the server, for people who'd rather not download the
// in-browser model. PixelHop forwards the image to a small local service
// (tools/rembg-server.py) that keeps the model in memory, one image at a time.
// Nothing is stored: the result goes straight back to the browser.

// rembgSlot lets one image through the model at a time; others wait in line.
var rembgSlot = make(chan struct{}, 1)

var rembgClient = &http.Client{Timeout: 3 * time.Minute}

func (a *App) apiRemoveBackground(w http.ResponseWriter, r *http.Request) {
	if a.cfg.RembgURL == "" {
		apiError(w, http.StatusNotFound, "background removal on the server is not set up here")
		return
	}
	u := currentUser(r)
	if u == nil {
		apiError(w, http.StatusUnauthorized, "sign in to remove backgrounds on the server, or use “On this device”")
		return
	}
	if !a.limits.allow("rembg-min:"+clientIP(r), 6, time.Minute) {
		apiError(w, http.StatusTooManyRequests, "one moment: too many in a row")
		return
	}
	if !a.limits.allow("rembg-day:"+strconv.FormatInt(u.ID, 10), a.cfg.RembgPerDay, 24*time.Hour) {
		apiError(w, http.StatusTooManyRequests, fmt.Sprintf("you've used today's %d server removals; “On this device” still works", a.cfg.RembgPerDay))
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, a.cfg.MaxUploadBytes+(1<<20))
	f, _, err := r.FormFile("file")
	if err != nil {
		apiError(w, http.StatusBadRequest, "send the image in a “file” field")
		return
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, a.cfg.MaxUploadBytes+1))
	if err != nil || int64(len(data)) > a.cfg.MaxUploadBytes {
		apiError(w, http.StatusRequestEntityTooLarge, fmt.Sprintf("images up to %d MB", a.cfg.MaxUploadBytes>>20))
		return
	}
	if _, ok := imaging.Sniff(data); !ok {
		apiError(w, http.StatusBadRequest, imaging.ErrUnsupported.Error())
		return
	}

	// Wait for the model, but not forever.
	wait, cancel := context.WithTimeout(r.Context(), 90*time.Second)
	defer cancel()
	select {
	case rembgSlot <- struct{}{}:
		defer func() { <-rembgSlot }()
	case <-wait.Done():
		apiError(w, http.StatusServiceUnavailable, "the server is busy; try “On this device” instead")
		return
	}

	req, _ := http.NewRequestWithContext(r.Context(), http.MethodPost, a.cfg.RembgURL+"/remove", bytes.NewReader(data))
	req.Header.Set("Content-Type", "application/octet-stream")
	resp, err := rembgClient.Do(req)
	if err != nil {
		a.log.Error("rembg", "err", err)
		apiError(w, http.StatusBadGateway, "background removal is unavailable right now; try “On this device”")
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		msg, _ := io.ReadAll(io.LimitReader(resp.Body, 300))
		a.log.Error("rembg", "status", resp.Status, "body", string(msg))
		apiError(w, http.StatusBadGateway, "background removal failed on this image")
		return
	}
	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Cache-Control", "no-store")
	io.Copy(w, io.LimitReader(resp.Body, 200<<20))
}

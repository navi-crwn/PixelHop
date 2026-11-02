package app

import (
	"net/http"
	"os"
	"strings"
)

const storagePaused = "Uploads are temporarily paused because server storage is low or unavailable. Please try again later."

// Check both permanent and temporary storage, including multipart spill files.
// Failure to measure capacity is not permission to write blindly.
func (a *App) storageAvailable(incoming int64) bool {
	mb, percent := a.cfg.MinFreeMB, a.cfg.MinFreePercent
	if mb <= 0 {
		mb = 1024
	}
	if percent <= 0 {
		percent = 5
	}
	for _, dir := range []string{a.cfg.DataDir, os.TempDir()} {
		free, total := a.space(dir)
		reserve := max(int64(mb)<<20, total/100*int64(percent))
		if total <= 0 || free <= reserve || incoming > (free-reserve)/3 {
			return false
		}
	}
	return true
}

func (a *App) uploadStatus(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	available := a.storageAvailable(0)
	message := ""
	if !available {
		message = a.tr(r, storagePaused)
	}
	apiOK(w, map[string]any{"available": available, "message": message})
}

func (a *App) storageGate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := r.URL.Path
		upload := (r.Method == "POST" || r.Method == "PUT") &&
			(p == "/api/v1/upload" || p == "/api/upload.php" || p == "/share" ||
				p == "/settings/avatar" || p == "/api/v1/uploads" || strings.HasPrefix(p, "/api/v1/uploads/") || strings.HasPrefix(p, "/api/v1/tools/"))
		if !upload {
			next.ServeHTTP(w, r)
			return
		}
		// Serial admission covers parsing, temporary files and final writes, so
		// simultaneous requests cannot all spend the same measured free space.
		a.uploadMu.Lock()
		defer a.uploadMu.Unlock()
		incoming := r.ContentLength
		if incoming < 0 {
			incoming = max(a.cfg.MaxMediaBytes, a.cfg.MaxUploadBytes*int64(a.cfg.MaxBatch))
		}
		// Chunk finalisation and URL imports have small request bodies but write media.
		if strings.HasSuffix(p, "/finish") || p == "/api/v1/upload" || p == "/api/upload.php" {
			incoming = max(incoming, a.cfg.MaxMediaBytes)
		}
		if !a.storageAvailable(incoming) {
			w.Header().Set("Retry-After", "60")
			apiError(w, http.StatusInsufficientStorage, a.tr(r, storagePaused))
			return
		}
		next.ServeHTTP(w, r)
	})
}

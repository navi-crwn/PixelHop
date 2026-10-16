package app

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Big files (phone videos can be hundreds of MB) are sent in pieces, so no
// single request hits the proxy's size limit and a dropped connection only
// costs one piece:
//
//	POST /api/v1/uploads            {"name": "rec.mp4", "size": 123456789} -> {"id", "chunk_size"}
//	PUT  /api/v1/uploads/{id}?offset=N   (raw bytes, at most chunk_size)  -> {"received"}
//	POST /api/v1/uploads/{id}/finish     (same options as /api/v1/upload) -> the image

const chunkSize = 8 << 20

type chunkUpload struct {
	mu      sync.Mutex
	path    string
	name    string
	size    int64
	got     int64
	owner   string
	created time.Time
}

type chunkStore struct {
	mu sync.Mutex
	m  map[string]*chunkUpload
}

func newChunkStore() *chunkStore { return &chunkStore{m: map[string]*chunkUpload{}} }

func (s *chunkStore) get(id string) *chunkUpload {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.m[id]
}

func (s *chunkStore) drop(id string) {
	s.mu.Lock()
	c := s.m[id]
	delete(s.m, id)
	s.mu.Unlock()
	if c != nil {
		os.Remove(c.path)
	}
}

// sweep forgets uploads nobody finished within six hours.
func (s *chunkStore) sweep(ctx context.Context) {
	t := time.NewTicker(10 * time.Minute)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			s.mu.Lock()
			for id, c := range s.m {
				if time.Since(c.created) > 6*time.Hour {
					os.Remove(c.path)
					delete(s.m, id)
				}
			}
			s.mu.Unlock()
		}
	}
}

func uploaderKey(r *http.Request) string {
	if u := currentUser(r); u != nil {
		return "u:" + strconv.FormatInt(u.ID, 10)
	}
	return "ip:" + clientIP(r)
}

func (a *App) apiStartUpload(w http.ResponseWriter, r *http.Request) {
	u := currentUser(r)
	if u == nil && a.db.Setting(r.Context(), "guest_uploads", fmt.Sprint(a.cfg.GuestUploads)) != "true" {
		apiError(w, http.StatusUnauthorized, "Please sign in to upload on this server.")
		return
	}
	var body struct {
		Name string `json:"name"`
		Size int64  `json:"size"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&body); err != nil || body.Size <= 0 {
		apiError(w, http.StatusBadRequest, `send JSON like {"name": "video.mp4", "size": 12345678}`)
		return
	}
	if limit := a.mediaLimit(u); body.Size > limit {
		apiError(w, http.StatusBadRequest, a.tooLarge(body.Name, u, limit).msg)
		return
	}
	if u != nil {
		if used, _ := a.db.UserStorage(r.Context(), u.ID); used+body.Size > u.QuotaBytes {
			apiError(w, http.StatusBadRequest, fmt.Sprintf("Your storage is full (%s of %s). Delete some files to make room.", humanBytes(used), humanBytes(u.QuotaBytes)))
			return
		}
	}
	if !a.limits.allow("bigup:"+clientIP(r), 60, time.Hour) {
		apiError(w, http.StatusTooManyRequests, "You're uploading very fast. Please wait a little.")
		return
	}
	a.chunks.mu.Lock()
	open := 0
	key := uploaderKey(r)
	for _, c := range a.chunks.m {
		if c.owner == key {
			open++
		}
	}
	a.chunks.mu.Unlock()
	if open >= 10 {
		apiError(w, http.StatusTooManyRequests, "Finish your other uploads first.")
		return
	}
	f, err := os.CreateTemp(a.tmpDir(), "chunk-")
	if err != nil {
		apiError(w, http.StatusInternalServerError, "couldn't start the upload")
		return
	}
	f.Close()
	id := randomToken(16)
	a.chunks.mu.Lock()
	a.chunks.m[id] = &chunkUpload{path: f.Name(), name: trim(body.Name, 200), size: body.Size, owner: key, created: time.Now()}
	a.chunks.mu.Unlock()
	apiOK(w, map[string]any{"id": id, "chunk_size": chunkSize})
}

func (a *App) ownChunk(w http.ResponseWriter, r *http.Request) *chunkUpload {
	c := a.chunks.get(r.PathValue("id"))
	if c == nil || c.owner != uploaderKey(r) {
		apiError(w, http.StatusNotFound, "this upload expired; please start again")
		return nil
	}
	return c
}

func (a *App) apiPutChunk(w http.ResponseWriter, r *http.Request) {
	c := a.ownChunk(w, r)
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	off, err := strconv.ParseInt(r.URL.Query().Get("offset"), 10, 64)
	if err != nil || off != c.got {
		// Tell the client where to continue (after a retry or a lost answer).
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusConflict)
		writeJSON(w, map[string]any{"success": false, "error": "wrong offset", "received": c.got})
		return
	}
	f, err := os.OpenFile(c.path, os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		apiError(w, http.StatusInternalServerError, "couldn't save this piece")
		return
	}
	limit := min(int64(chunkSize), c.size-c.got)
	n, err := io.Copy(f, io.LimitReader(r.Body, limit+1))
	if n > limit {
		f.Truncate(c.got)
		f.Close()
		apiError(w, http.StatusBadRequest, "this piece is too big")
		return
	}
	f.Close()
	if err != nil {
		os.Truncate(c.path, c.got) // throw away the half piece; the client resends it
		apiError(w, http.StatusBadRequest, "this piece didn't arrive whole; please resend it")
		return
	}
	c.got += n
	apiOK(w, map[string]any{"received": c.got})
}

func (a *App) apiFinishUpload(w http.ResponseWriter, r *http.Request) {
	c := a.ownChunk(w, r)
	if c == nil {
		return
	}
	c.mu.Lock()
	complete := c.got == c.size
	c.mu.Unlock()
	if !complete {
		apiError(w, http.StatusBadRequest, fmt.Sprintf("only %d of %d bytes arrived", c.got, c.size))
		return
	}
	id := r.PathValue("id")
	a.chunks.mu.Lock()
	delete(a.chunks.m, id) // storeImage removes the file when done
	a.chunks.mu.Unlock()

	r.ParseMultipartForm(1 << 20)
	ctx, u := r.Context(), currentUser(r)
	var expiresAt int64
	if d, ok := expiries[r.FormValue("expire")]; ok {
		expiresAt = time.Now().Add(d).Unix()
	}
	albumID := ""
	if aid := r.FormValue("album_id"); aid != "" && u != nil && validID(aid) {
		if al, err := a.db.Album(ctx, aid); err == nil && al.UserID.Valid && al.UserID.Int64 == u.ID {
			albumID = aid
		}
	}
	useSlug := r.FormValue("slug") != "0" && r.FormValue("slug") != "false"
	in := uploadInput{name: c.name, path: c.path, size: c.size}
	im, tok, dup, err := a.storeImage(ctx, in, u, clientIP(r), albumID, expiresAt, useSlug, false, trim(strings.TrimSpace(r.FormValue("alt")), 300))
	if err != nil {
		var ue userError
		if errors.As(err, &ue) {
			apiError(w, http.StatusBadRequest, ue.msg)
			return
		}
		a.log.Error("finish upload", "err", err)
		apiError(w, http.StatusInternalServerError, "Upload failed on our side. Please try again.")
		return
	}
	if dup && albumID != "" && im.AlbumID.String != albumID {
		im.AlbumID = sql.NullString{String: albumID, Valid: true}
		a.db.UpdateImageMeta(ctx, im)
	}
	m := a.imageJSON(im)
	if tok != "" {
		m["delete_url"] = a.cfg.BaseURL + "/delete/" + im.ID + "/" + tok
	}
	m["duplicate"] = dup
	apiOK(w, m)
}

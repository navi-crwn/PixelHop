package storage

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"
)

// Remote serves images that already live at a public URL, which is how links
// imported from PixelHop 2 (Contabo + R2) keep working. The "key" is the full
// URL, written by the importer, never taken from a visitor's request.
type Remote struct{ client *http.Client }

func NewRemote() *Remote { return &Remote{client: &http.Client{Timeout: time.Minute}} }

func (r *Remote) Name() string { return "remote" }

func (r *Remote) Put(context.Context, string, io.Reader, int64, string) error {
	return errors.New("remote storage is read-only")
}

func (r *Remote) Get(ctx context.Context, key, rangeHeader string) (*Object, error) {
	if !strings.HasPrefix(key, "https://") && !strings.HasPrefix(key, "http://") {
		return nil, ErrNotFound
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, key, nil)
	if err != nil {
		return nil, err
	}
	if rangeHeader != "" {
		req.Header.Set("Range", rangeHeader)
	}
	resp, err := r.client.Do(req)
	if err != nil {
		return nil, err
	}
	return objectFromResponse(resp)
}

// Delete is a no-op: the old buckets are cleaned up by their owner.
func (r *Remote) Delete(context.Context, string) error { return nil }

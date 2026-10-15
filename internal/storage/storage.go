// Package storage keeps image bytes somewhere: the local disk, any
// S3-compatible bucket, or (for links imported from PixelHop 2) a fixed URL.
package storage

import (
	"context"
	"errors"
	"io"
	"net/http"
)

var ErrNotFound = errors.New("object not found")

// Object is an open stored file. Body must be closed.
type Object struct {
	Body         io.ReadCloser
	Size         int64
	ContentType  string
	ETag         string
	LastModified string
	// Status is 200 or 206 (when a Range was honoured); ContentRange goes with 206.
	Status       int
	ContentRange string
}

type Backend interface {
	Name() string
	Put(ctx context.Context, key string, r io.Reader, size int64, contentType string) error
	// Get opens key. rangeHeader is the client's Range header, passed through
	// when the backend can honour it ("" for the whole object).
	Get(ctx context.Context, key, rangeHeader string) (*Object, error)
	Delete(ctx context.Context, key string) error
}

// Local backends can hand the file to http.ServeContent directly, which gives
// Range, If-Modified-Since and friends for free.
type FileServer interface {
	ServeFile(w http.ResponseWriter, r *http.Request, key string) error
}

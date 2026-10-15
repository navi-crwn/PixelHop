package storage

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

type Local struct{ root *os.Root }

func NewLocal(dir string) (*Local, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	// os.Root refuses any key that would escape dir (.., symlinks), so a bad key
	// can never read or write outside the image folder.
	root, err := os.OpenRoot(dir)
	if err != nil {
		return nil, err
	}
	return &Local{root: root}, nil
}

func (l *Local) Name() string { return "local" }

func (l *Local) Put(_ context.Context, key string, r io.Reader, _ int64, _ string) error {
	if dir := filepath.Dir(key); dir != "." {
		if err := l.root.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	tmp := key + ".part"
	f, err := l.root.Create(tmp)
	if err != nil {
		return err
	}
	if _, err := io.Copy(f, r); err != nil {
		f.Close()
		l.root.Remove(tmp)
		return err
	}
	if err := f.Close(); err != nil {
		l.root.Remove(tmp)
		return err
	}
	return l.root.Rename(tmp, key)
}

func (l *Local) Get(_ context.Context, key, _ string) (*Object, error) {
	f, err := l.root.Open(key)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	st, err := f.Stat()
	if err != nil {
		f.Close()
		return nil, err
	}
	return &Object{Body: f, Size: st.Size(), Status: 200}, nil
}

func (l *Local) ServeFile(w http.ResponseWriter, r *http.Request, key string) error {
	f, err := l.root.Open(key)
	if errors.Is(err, fs.ErrNotExist) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return err
	}
	w.Header().Set("ETag", fmt.Sprintf(`"%x-%x"`, st.ModTime().Unix(), st.Size()))
	http.ServeContent(w, r, "", st.ModTime(), f)
	return nil
}

func (l *Local) Delete(_ context.Context, key string) error {
	err := l.root.Remove(key)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	// Tidy up now-empty date folders; failures here are harmless.
	for dir := filepath.Dir(key); dir != "." && dir != "/" && !strings.HasPrefix(dir, ".."); dir = filepath.Dir(dir) {
		if l.root.Remove(dir) != nil {
			break
		}
	}
	return err
}

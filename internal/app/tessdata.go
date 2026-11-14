package app

import (
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/navi-crwn/pixelhop/web"
)

// Reading languages for the studio's "Image to text". English is built into the program; any other
// language is a <code>.traineddata.gz file the site owner puts in PIXELHOP_DATA/tessdata
// (scripts/ocr-language.sh fetches them). Nothing is loaded from another site.
const embeddedOCR = "static/vendor/tesseract/lang/"

var tessdataName = regexp.MustCompile(`^([a-z][a-z_]{2,11})\.traineddata\.gz$`)

// ocrLanguages lists the languages this server can read, English first.
func (a *App) ocrLanguages() []string {
	langs := []string{"eng"}
	entries, _ := os.ReadDir(filepath.Join(a.cfg.DataDir, "tessdata"))
	var more []string
	for _, e := range entries {
		if m := tessdataName.FindStringSubmatch(e.Name()); m != nil && !e.IsDir() && m[1] != "eng" {
			more = append(more, m[1])
		}
	}
	sort.Strings(more)
	return append(langs, more...)
}

// tessdata serves a reading model to the studio: the owner's file first, else the built-in English one.
func (a *App) tessdata(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("file")
	m := tessdataName.FindStringSubmatch(name)
	if m == nil {
		a.notFound(w, r)
		return
	}
	w.Header().Set("Cache-Control", "public, max-age=2592000")
	w.Header().Set("Content-Type", "application/gzip")
	if m[1] != "eng" {
		http.ServeFile(w, r, filepath.Join(a.cfg.DataDir, "tessdata", name))
		return
	}
	if b, err := fs.ReadFile(web.Static, embeddedOCR+name); err == nil {
		if own, err := os.ReadFile(filepath.Join(a.cfg.DataDir, "tessdata", name)); err == nil {
			b = own // an owner's own English file wins
		}
		w.Write(b)
		return
	}
	a.notFound(w, r)
}

func joinLangs(l []string) string { return strings.Join(l, ",") }

package app

import (
	"html/template"
	"net/http"
	"time"

	"github.com/navi-crwn/pixelhop/internal/db"
)

// An album can be shown on another website in an iframe: a plain, fast grid
// of thumbnails that open the image pages here. Only what anyone with the
// link may see is listed; an album with a content warning shows just a link.
var embedPage = template.Must(template.New("embed").Parse(`<!doctype html>
<html lang="{{.Lang}}"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1">
<meta name="robots" content="noindex"><title>{{.Title}}</title>
<style>
*{box-sizing:border-box}body{margin:0;font:14px/1.4 system-ui,-apple-system,Segoe UI,sans-serif;color:#10121a;background:transparent}
.g{display:grid;grid-template-columns:repeat(auto-fill,minmax(150px,1fr));gap:6px;padding:6px}
.g a{display:block;aspect-ratio:1;border-radius:10px;overflow:hidden;background:#e8eaf2}
.g img{width:100%;height:100%;object-fit:cover;display:block}
.f{padding:4px 8px 8px;font-size:12px;color:#5b6174}.f a{color:inherit}
.w{padding:20px;text-align:center}
@media (prefers-color-scheme:dark){body{color:#e8ebf4}.g a{background:#1d2130}.f{color:#9aa3bb}}
</style></head><body>
{{if .Warned}}<p class="w"><a href="{{.Page}}" target="_blank" rel="noopener">{{.Open}}</a></p>
{{else}}<div class="g">{{range .Images}}<a href="{{.Page}}" target="_blank" rel="noopener"><img src="{{.Thumb}}" alt="{{.Alt}}" loading="lazy" width="{{.W}}" height="{{.H}}"></a>{{end}}</div>
<p class="f"><a href="{{.Page}}" target="_blank" rel="noopener">{{.Open}}</a></p>{{end}}
</body></html>`))

type embedImage struct {
	Page, Thumb, Alt string
	W, H             int
}

func (a *App) albumEmbed(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !validID(id) {
		a.notFound(w, r)
		return
	}
	al, err := a.db.Album(r.Context(), id)
	if err != nil {
		a.notFound(w, r)
		return
	}
	ims, _, _ := a.db.Images(r.Context(), db.ImageQuery{AlbumID: id, Sort: "album", Limit: 60})
	var list []embedImage
	now := time.Now().Unix()
	for _, im := range ims {
		if im.Hidden || (im.ExpiresAt != 0 && im.ExpiresAt <= now) || im.Locked() || im.HasWarning() {
			continue
		}
		list = append(list, embedImage{Page: a.pageURL(im.ID), Thumb: a.thumbURL(im), Alt: im.Alt, W: 150, H: 150})
	}
	if len(list) == 0 && !al.HasWarning() {
		a.notFound(w, r)
		return
	}
	p := a.base(r, page{})
	l := a.locale(w, r)
	title := al.Title
	if title == "" {
		title = a.cfg.SiteName
	}
	h := w.Header()
	h.Set("Content-Security-Policy", "default-src 'none'; img-src * data:; style-src 'unsafe-inline'; frame-ancestors *; base-uri 'none'; form-action 'none'")
	h.Set("Cache-Control", "public, max-age=300")
	h.Del("X-Frame-Options")
	h.Set("Content-Type", "text/html; charset=utf-8")
	embedPage.Execute(w, map[string]any{
		"Lang": l.Code, "Title": title, "Images": list, "Warned": al.HasWarning(),
		"Page": a.albumURL(al.ID), "Open": l.T("embed.open", "site", p.Site),
	})
}

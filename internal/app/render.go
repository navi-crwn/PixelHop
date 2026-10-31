package app

import (
	"bytes"
	"fmt"
	"html/template"
	"io/fs"
	"math"
	"net"
	"net/http"
	"net/netip"
	"strconv"
	"strings"
	"time"

	"github.com/navi-crwn/pixelhop/internal/db"
	"github.com/navi-crwn/pixelhop/internal/i18n"
	"github.com/navi-crwn/pixelhop/web"
)

type pages struct{ t map[string]*template.Template }

// page is what every template receives as ".".
type page struct {
	Title, Desc     string
	OGImage, OGType string
	NoIndex         bool
	Data            any

	User          *db.User
	Site          string
	Base          string
	Link          string // where shared links live (PIXELHOP_LINK_URL)
	Canon         string // the canonical address of a shared page
	OnLink        bool   // served through the link host
	Path          string
	Announcement  string
	Version       string
	Asset         string // cache-busting query for /static files
	Google        bool
	SSO           bool   // accounts come from HEL.ink
	Helink        string // HEL.ink base URL
	Control       string // HOP control center, shown to admins when configured
	Turnstile     string // Turnstile site key, when forms are protected
	RembgServer   bool   // background removal can run on the server
	GuestUploads  bool
	MaxMB         int64
	MaxMediaMB    int64
	MemberMediaMB int64 // what signing in would allow, for the guest hint
	MaxBatch      int
	L             i18n.Locale
	Langs         []i18n.Lang
}

var startedAt = fmt.Sprint(time.Now().Unix())

func loadPages(a *App) (*pages, error) {
	funcs := template.FuncMap{
		"bytes": humanBytes,
		"dims":  humanDims,
		"ago":   ago,
		"date":  func(t int64) string { return time.Unix(t, 0).UTC().Format("2 Jan 2006") },
		"iso":   func(t int64) string { return time.Unix(t, 0).UTC().Format(time.RFC3339) },
		// Pages load images by path, so they work under any host name;
		// the absolute links people copy come from linksFor.
		"direct": func(im *db.Image) string { return strings.TrimPrefix(a.directURL(im), a.cfg.LinkURL) },
		"thumb":  func(im *db.Image) string { return strings.TrimPrefix(a.thumbURL(im), a.cfg.LinkURL) },
		"add":    func(x, y int) int { return x + y },
		"pct": func(n, of int) int {
			if of == 0 {
				return 0
			}
			return n * 100 / of
		},
		"seq": func(n int) []int {
			s := make([]int, n)
			for i := range s {
				s[i] = i + 1
			}
			return s
		},
		"until":     func(t int64) string { return strings.TrimSuffix(ago(2*time.Now().Unix()-t), " ago") },
		"usage":     func(m map[int64][2]int64, id int64) [2]int64 { return m[id] },
		"mb":        func(b int64) int64 { return b >> 20 },
		"gb":        func(b int64) string { return strconv.FormatFloat(math.Round(float64(b)/(1<<30)*100)/100, 'f', -1, 64) },
		"avatar":    avatarURL,
		"display":   displayName,
		"initial":   func(u *db.User) string { return strings.ToUpper(string([]rune(displayName(u))[:1])) },
		"hopFonts":  hopFonts,
		"hopFamily": func() []hopApp { return hopFamily("pixelhop") },
		"backable":  func(path string) bool { return backPaths[path] },
	}
	p := &pages{t: map[string]*template.Template{}}
	files, err := fs.Glob(web.Templates, "templates/*.html")
	if err != nil {
		return nil, err
	}
	for _, f := range files {
		name := strings.TrimSuffix(strings.TrimPrefix(f, "templates/"), ".html")
		if name == "base" {
			continue
		}
		t, err := template.New("").Funcs(funcs).ParseFS(web.Templates, "templates/base.html", "templates/partials/*.html", f)
		if err != nil {
			return nil, fmt.Errorf("template %s: %w", name, err)
		}
		p.t[name] = t
	}
	return p, nil
}

func (a *App) base(r *http.Request, p page) page {
	ctx := r.Context()
	p.User = currentUser(r)
	p.Site = a.cfg.SiteName
	p.Base = a.cfg.BaseURL
	p.Link = a.cfg.LinkURL
	p.OnLink = onLinkHost(r)
	p.Path = r.URL.Path
	p.Version = Version
	p.Asset = Version + "-" + startedAt
	p.Google = a.cfg.GoogleEnabled()
	p.SSO = a.cfg.SSOEnabled()
	p.Helink = a.cfg.HelinkURL
	if p.User != nil && p.User.IsAdmin() {
		p.Control = a.cfg.ControlURL
	}
	if a.cfg.TurnstileEnabled() {
		p.Turnstile = a.cfg.TurnstileSiteKey
	}
	p.RembgServer = a.cfg.RembgURL != ""
	p.Announcement = a.db.Setting(ctx, "announcement", "")
	p.GuestUploads = a.db.Setting(ctx, "guest_uploads", fmt.Sprint(a.cfg.GuestUploads)) == "true"
	p.MaxMB = a.imageLimit(p.User) >> 20
	p.MaxMediaMB = a.mediaLimit(p.User) >> 20
	p.MemberMediaMB = a.cfg.MaxMediaBytes >> 20
	p.MaxBatch = a.cfg.MaxBatch
	return p
}

func (a *App) render(w http.ResponseWriter, r *http.Request, name, title string, data any) {
	a.renderPage(w, r, name, page{Title: title, Data: data})
}

func (a *App) renderPage(w http.ResponseWriter, r *http.Request, name string, p page) {
	t := a.pages.t[name]
	if t == nil {
		a.serverError(w, r, fmt.Errorf("no template %q", name))
		return
	}
	p.L = a.locale(w, r)
	p.Langs = i18n.Langs
	p = a.base(r, p)
	if strings.HasPrefix(name, "admin") || name == "dashboard" || name == "settings" || name == "delete" {
		p.NoIndex = true
	}
	var buf bytes.Buffer
	if err := t.ExecuteTemplate(&buf, "base", p); err != nil {
		a.log.Error("render", "page", name, "err", err)
		http.Error(w, "Something went wrong on our side.", http.StatusInternalServerError)
		return
	}
	h := w.Header()
	h.Set("Content-Type", "text/html; charset=utf-8")
	formHelink := ""
	if a.cfg.HelinkURL != "" {
		formHelink = " " + a.cfg.HelinkURL
	}
	// The studio's background remover (onnxruntime-web) compiles code at run
	// time, so only that page may use eval. It shows no other people's content.
	eval := ""
	if name == "tools" || name == "batch" {
		eval = " 'unsafe-eval'"
	}
	// https://hel.ink serves the HOP family fonts (style-src, font-src) and app marks (img-src https:).
	h.Set("Content-Security-Policy", "default-src 'self'; img-src 'self' data: blob: https:; style-src 'self' 'unsafe-inline' https://hel.ink; "+
		"script-src 'self' 'wasm-unsafe-eval'"+eval+" https://cdn.jsdelivr.net https://challenges.cloudflare.com blob:; frame-src https://challenges.cloudflare.com; worker-src 'self' blob: https://cdn.jsdelivr.net; "+
		"connect-src 'self' https://cdn.jsdelivr.net https://tessdata.projectnaptha.com https://staticimgly.com blob: data:; "+
		"font-src 'self' https://hel.ink; frame-ancestors 'self'; base-uri 'none'; form-action 'self' https://accounts.google.com"+formHelink)
	if p.User != nil {
		h.Set("Cache-Control", "private, no-store")
	}
	buf.WriteTo(w)
}

// ---- simple pages ----

func (a *App) home(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	n, views := a.db.PublicStats(ctx)
	a.renderPage(w, r, "home", page{
		Title: a.db.Setting(ctx, "seo_title", "Free image hosting with a built-in photo studio"),
		Desc: a.db.Setting(ctx, "seo_description", "Drop an image, get a link. Strips GPS data, makes albums, works with ShareX. "+
			"Edit, compress, convert, OCR and remove backgrounds right in your browser."),
		OGImage: a.cfg.BaseURL + "/static/img/social.jpg",
		Data:    map[string]any{"Images": n, "Views": views},
	})
}

func (a *App) tools(w http.ResponseWriter, r *http.Request) {
	a.renderPage(w, r, "tools", page{Title: "Photo studio",
		Desc: "Compress, resize, crop, convert, read text (OCR), remove backgrounds, blur faces and pick colors, privately in your browser."})
}

func (a *App) batch(w http.ResponseWriter, r *http.Request) {
	a.renderPage(w, r, "batch", page{Title: "Batch editing",
		Desc: "Resize, convert, remove backgrounds, blur faces and upscale many images at once; download a ZIP or upload an album."})
}

func (a *App) help(w http.ResponseWriter, r *http.Request) {
	a.renderPage(w, r, "help", page{Title: "Help", Data: a.cfg.AdminEmail,
		Desc: "Answers about uploading, links, albums, the studio, accounts and privacy on " + a.cfg.SiteName + "."})
}

func (a *App) docs(w http.ResponseWriter, r *http.Request) {
	a.renderPage(w, r, "docs", page{Title: "API & ShareX", Desc: "Upload from scripts, apps and ShareX with the " + a.cfg.SiteName + " API."})
}

func (a *App) legal(w http.ResponseWriter, r *http.Request) {
	a.renderPage(w, r, "legal", page{Title: "Terms, privacy & takedowns", Data: a.cfg.AdminEmail})
}

func (a *App) notFound(w http.ResponseWriter, r *http.Request) {
	// PixelHop 2 trimmed slashes, so /{id}/ links exist in the wild.
	if id, ok := strings.CutSuffix(strings.TrimPrefix(r.URL.Path, "/"), "/"); ok && r.Method == http.MethodGet && validID(id) {
		http.Redirect(w, r, "/"+id, http.StatusMovedPermanently)
		return
	}
	if strings.HasPrefix(r.URL.Path, "/api/") {
		apiError(w, http.StatusNotFound, "not found")
		return
	}
	w.WriteHeader(http.StatusNotFound)
	a.renderPage(w, r, "error", page{Title: "Not found", NoIndex: true, Data: map[string]any{
		"Code": 404, "Message": "This link doesn't lead anywhere. The image may have expired or been deleted."}})
}

func (a *App) fail(w http.ResponseWriter, r *http.Request, code int, msg string) {
	if strings.HasPrefix(r.URL.Path, "/api/") || strings.Contains(r.Header.Get("Accept"), "application/json") {
		apiError(w, code, msg)
		return
	}
	w.WriteHeader(code)
	a.renderPage(w, r, "error", page{Title: http.StatusText(code), NoIndex: true, Data: map[string]any{"Code": code, "Message": msg}})
}

func (a *App) serverError(w http.ResponseWriter, r *http.Request, err error) {
	a.log.Error("server error", "path", r.URL.Path, "err", err)
	a.fail(w, r, http.StatusInternalServerError, "Something went wrong on our side. Please try again in a moment.")
}

func (a *App) favicon(w http.ResponseWriter, r *http.Request) {
	r.URL.Path = "/static/img/logo.svg"
	w.Header().Set("Content-Type", "image/svg+xml")
	a.static.ServeHTTP(w, r)
}

// metrics serves Prometheus-style counters to the server itself only.
func (a *App) metrics(w http.ResponseWriter, r *http.Request) {
	host, _, _ := net.SplitHostPort(r.RemoteAddr)
	if ip, err := netip.ParseAddr(host); err != nil || !ip.IsLoopback() || r.Header.Get("X-Forwarded-For") != "" {
		a.notFound(w, r)
		return
	}
	a.writeMetrics(w, r)
}

func (a *App) writeMetrics(w http.ResponseWriter, r *http.Request) {
	s := a.db.Stats(r.Context())
	w.Header().Set("Content-Type", "text/plain; version=0.0.4")
	fmt.Fprintf(w, "pixelhop_images %d\npixelhop_bytes %d\npixelhop_views %d\npixelhop_users %d\npixelhop_albums %d\npixelhop_open_reports %d\n",
		s.Images, s.Bytes, s.Views, s.Users, s.Albums, s.OpenReports)
	for tool, n := range a.db.Usage(r.Context(), 1) {
		fmt.Fprintf(w, "pixelhop_tool_runs_today{tool=%q} %d\n", tool, n)
	}
}

func (a *App) robots(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain")
	fmt.Fprintf(w, "User-agent: *\nDisallow: /admin\nDisallow: /dashboard\nDisallow: /settings\nDisallow: /delete/\nDisallow: /api/\n\nSitemap: %s/sitemap.xml\n", a.cfg.BaseURL)
}

// ---- formatting ----

func humanBytes(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	v := float64(b) / float64(div)
	if v >= 100 {
		return fmt.Sprintf("%.0f %cB", v, "KMGTPE"[exp])
	}
	return fmt.Sprintf("%.1f %cB", v, "KMGTPE"[exp])
}

func humanDims(w, h int) string { return fmt.Sprintf("%d × %d", w, h) }

func ago(t int64) string {
	if t == 0 {
		return "never"
	}
	d := time.Since(time.Unix(t, 0))
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return plural(int(d.Minutes()), "minute") + " ago"
	case d < 48*time.Hour:
		return plural(int(d.Hours()), "hour") + " ago"
	case d < 60*24*time.Hour:
		return plural(int(d.Hours()/24), "day") + " ago"
	case d < 730*24*time.Hour:
		return plural(int(d.Hours()/24/30), "month") + " ago"
	}
	return plural(int(d.Hours()/24/365), "year") + " ago"
}

func plural(n int, w string) string {
	if n == 1 {
		return "1 " + w
	}
	return fmt.Sprintf("%d %ss", n, w)
}

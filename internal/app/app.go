// Package app is PixelHop's web server: pages, the JSON API, image delivery
// and the admin panel, all served from one process.
package app

import (
	"context"
	"io/fs"
	"log/slog"
	"net/http"
	"time"

	"github.com/navi-crwn/pixelhop/internal/config"
	"github.com/navi-crwn/pixelhop/internal/db"
	"github.com/navi-crwn/pixelhop/internal/storage"
	"github.com/navi-crwn/pixelhop/web"
)

var Version = "3.0.0"

type App struct {
	cfg      *config.Config
	db       *db.DB
	store    storage.Backend // where new uploads go
	backends map[string]storage.Backend
	pages    *pages
	limits   *limiter
	views    *seen
	log      *slog.Logger
	static   http.Handler
	bans     banList
}

func New(cfg *config.Config, d *db.DB, log *slog.Logger) (*App, error) {
	a := &App{cfg: cfg, db: d, log: log, limits: newLimiter(), views: newSeen(time.Hour)}

	local, err := storage.NewLocal(cfg.DataDir + "/images")
	if err != nil {
		return nil, err
	}
	a.backends = map[string]storage.Backend{"local": local, "remote": storage.NewRemote()}
	a.store = local
	if cfg.Storage == "s3" {
		s3 := storage.NewS3(cfg.S3)
		a.backends["s3"] = s3
		a.store = s3
	}

	if a.pages, err = loadPages(a); err != nil {
		return nil, err
	}
	static, _ := fs.Sub(web.Static, "static")
	a.static = http.StripPrefix("/static/", cacheForever(http.FileServerFS(static)))
	return a, nil
}

func (a *App) Handler() http.Handler {
	mux := http.NewServeMux()

	// pages
	mux.HandleFunc("GET /{$}", a.home)
	mux.HandleFunc("GET /tools", a.tools)
	mux.HandleFunc("GET /tools/batch", a.batch)
	mux.HandleFunc("GET /tools/qr.png", a.qrTool)
	mux.HandleFunc("GET /tools/qr.svg", a.qrTool)
	mux.HandleFunc("GET /docs", a.docs)
	mux.HandleFunc("GET /help", a.help)
	mux.HandleFunc("GET /legal", a.legal)
	mux.HandleFunc("GET /a/{id}", a.album)
	mux.HandleFunc("GET /{id}", a.view)
	mux.HandleFunc("GET /result", a.legacyResult) // PixelHop 2 upload results: /result?id=a,b
	mux.HandleFunc("GET /delete/{id}/{token}", a.deleteConfirm)
	mux.HandleFunc("POST /delete/{id}/{token}", a.deleteWithToken)
	mux.HandleFunc("POST /report/{id}", a.report)

	// image bytes
	mux.HandleFunc("GET /i/{file}", a.serveFull)
	mux.HandleFunc("GET /i/{y}/{m}/{d}/{file}", a.serveLegacy) // PixelHop 2 links
	mux.HandleFunc("GET /t/{file}", a.serveThumb)
	mux.HandleFunc("GET /qr/{id}", a.qr)
	mux.Handle("GET /static/", a.static)
	mux.HandleFunc("GET /favicon.ico", a.favicon)
	mux.HandleFunc("GET /robots.txt", a.robots)
	mux.HandleFunc("GET /metrics", a.metrics)
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("ok")) })

	// accounts
	mux.HandleFunc("GET /login", a.loginPage)
	mux.HandleFunc("POST /login", a.login)
	mux.HandleFunc("GET /register", a.registerPage)
	mux.HandleFunc("POST /register", a.register)
	mux.HandleFunc("POST /logout", a.logout)
	mux.HandleFunc("GET /auth/helink", a.helinkStart)
	mux.HandleFunc("GET /auth/helink/callback", a.helinkCallback)
	mux.HandleFunc("GET /auth/google", a.googleStart)
	mux.HandleFunc("GET /auth/google/callback", a.googleCallback)
	mux.HandleFunc("GET /auth/google-callback", a.googleCallback) // redirect URI used by PixelHop 2
	mux.HandleFunc("GET /auth/google-callback.php", a.googleCallback)

	// member area
	mux.HandleFunc("GET /dashboard", a.member(a.dashboard))
	mux.HandleFunc("POST /dashboard/download", a.member(a.downloadZip))
	mux.HandleFunc("GET /settings", a.member(a.settingsPage))
	mux.HandleFunc("POST /settings/profile", a.member(a.saveProfile))
	mux.HandleFunc("POST /settings/password", a.member(a.savePassword))
	mux.HandleFunc("POST /settings/keys", a.member(a.createKey))
	mux.HandleFunc("POST /settings/keys/{id}/delete", a.member(a.deleteKey))
	mux.HandleFunc("GET /settings/sharex/{prefix}", a.member(a.sharexConfig))
	mux.HandleFunc("POST /settings/delete-account", a.member(a.deleteAccount))
	mux.HandleFunc("POST /settings/ack-warning", a.member(a.ackWarning))

	// admin
	mux.HandleFunc("GET /admin", a.admin(a.adminHome))
	mux.HandleFunc("GET /admin/images", a.admin(a.adminImages))
	mux.HandleFunc("GET /admin/users", a.admin(a.adminUsers))
	mux.HandleFunc("POST /admin/users/{id}", a.admin(a.adminUserUpdate))
	mux.HandleFunc("GET /admin/reports", a.admin(a.adminReports))
	mux.HandleFunc("POST /admin/reports/{id}", a.admin(a.adminReportAction))
	mux.HandleFunc("GET /admin/bans", a.admin(a.adminBans))
	mux.HandleFunc("POST /admin/bans", a.admin(a.adminBan))
	mux.HandleFunc("POST /admin/bans/delete", a.admin(a.adminUnban))
	mux.HandleFunc("GET /admin/settings", a.admin(a.adminSettings))
	mux.HandleFunc("POST /admin/settings", a.admin(a.adminSaveSettings))
	mux.HandleFunc("POST /admin/images/{id}", a.admin(a.adminImageAction))

	// JSON API (session cookie or API key)
	mux.HandleFunc("POST /api/v1/upload", a.apiUpload)
	mux.HandleFunc("POST /api/upload.php", a.apiUpload) // ShareX configs made for PixelHop 2
	mux.HandleFunc("GET /api/v1/images", a.apiImages)
	mux.HandleFunc("GET /api/v1/images/{id}", a.apiImage)
	mux.HandleFunc("PATCH /api/v1/images/{id}", a.apiRenameImage)
	mux.HandleFunc("DELETE /api/v1/images/{id}", a.apiDeleteImage)
	mux.HandleFunc("GET /api/v1/me", a.apiMe)
	mux.HandleFunc("GET /api/v1/stats", a.apiStats)
	mux.HandleFunc("POST /api/v1/tools/{tool}", a.apiAITool)

	// Pages and tool URLs from PixelHop 2.
	for from, to := range legacyPages {
		mux.Handle("GET "+from, http.RedirectHandler(to, http.StatusMovedPermanently))
	}

	mux.HandleFunc("/", a.notFound)

	var h http.Handler = mux
	h = a.withUser(h)
	h = a.checkBan(h)
	h = csrf(h)
	h = securityHeaders(h)
	h = a.withIP(h)
	h = a.recoverer(h)
	h = a.accessLog(h)
	return h
}

// Run serves until ctx is cancelled, then drains in-flight requests.
func (a *App) Run(ctx context.Context) error {
	srv := &http.Server{
		Addr:              a.cfg.Addr,
		Handler:           a.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       5 * time.Minute, // big uploads on slow links
		WriteTimeout:      5 * time.Minute,
		IdleTimeout:       2 * time.Minute,
		MaxHeaderBytes:    64 << 10,
	}
	go a.janitor(ctx)
	errc := make(chan error, 1)
	go func() { errc <- srv.ListenAndServe() }()
	a.log.Info("PixelHop is running", "addr", a.cfg.Addr, "url", a.cfg.BaseURL, "storage", a.store.Name(), "version", Version)
	select {
	case err := <-errc:
		return err
	case <-ctx.Done():
		shut, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		return srv.Shutdown(shut)
	}
}

func cacheForever(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.RawQuery != "" { // versioned (?v=...) assets never change
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		} else {
			w.Header().Set("Cache-Control", "public, max-age=3600")
		}
		h.ServeHTTP(w, r)
	})
}

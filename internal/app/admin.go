package app

import (
	"fmt"
	"net/http"
	"net/netip"
	"strconv"
	"strings"
	"time"

	"github.com/navi-crwn/pixelhop/internal/db"
)

type adminHomeData struct {
	Usage     map[string]int // AI tool runs, last 7 days
	DiskFree  int64
	DiskTotal int64
	Stats     db.Stats
	Recent    []*db.Image
	MaxDay    int
	Storage   string
	Version   string
}

func (a *App) adminHome(w http.ResponseWriter, r *http.Request) {
	s := a.db.Stats(r.Context())
	recent, _, _ := a.db.Images(r.Context(), db.ImageQuery{Limit: 12})
	// Fill in the quiet days so the chart always shows 30 even bars.
	byDay := map[string]db.DayCount{}
	for _, day := range s.Daily {
		byDay[day.Day] = day
	}
	s.Daily = s.Daily[:0]
	for i := 29; i >= 0; i-- {
		day := time.Now().UTC().AddDate(0, 0, -i).Format("2006-01-02")
		dc := byDay[day]
		dc.Day = day
		s.Daily = append(s.Daily, dc)
	}
	d := adminHomeData{Stats: s, Recent: recent, Storage: a.store.Name(), Version: Version, Usage: a.db.Usage(r.Context(), 7)}
	d.DiskFree, d.DiskTotal = diskSpace(a.cfg.DataDir)
	for _, day := range s.Daily {
		d.MaxDay = max(d.MaxDay, day.Count)
	}
	a.render(w, r, "admin_home", "Admin", d)
}

type adminImagesData struct {
	Images []*db.Image
	Total  int
	Page   int
	Pages  int
	Query  string
	Sort   string
}

func (a *App) adminImages(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	pg, _ := strconv.Atoi(q.Get("page"))
	if pg < 1 {
		pg = 1
	}
	const per = 60
	uid, _ := strconv.ParseInt(q.Get("user"), 10, 64)
	ims, total, err := a.db.Images(r.Context(), db.ImageQuery{UserID: uid, Search: q.Get("q"), Sort: q.Get("sort"), Limit: per, Offset: (pg - 1) * per})
	if err != nil {
		a.serverError(w, r, err)
		return
	}
	a.render(w, r, "admin_images", "Images", adminImagesData{Images: ims, Total: total, Page: pg, Pages: (total + per - 1) / per, Query: q.Get("q"), Sort: q.Get("sort")})
}

func (a *App) adminImageAction(w http.ResponseWriter, r *http.Request) {
	im, err := a.db.Image(r.Context(), r.PathValue("id"))
	if err != nil {
		a.notFound(w, r)
		return
	}
	switch r.FormValue("action") {
	case "hide":
		a.db.SetImageHidden(r.Context(), im.ID, true)
	case "unhide":
		a.db.SetImageHidden(r.Context(), im.ID, false)
	case "delete":
		a.removeImage(r.Context(), im)
		a.db.DeleteEmptyAlbums(r.Context())
	case "ban":
		if im.IP != "" {
			a.db.Ban(r.Context(), im.IP, "uploaded "+im.ID)
			a.invalidateBans()
		}
		a.removeImage(r.Context(), im)
	}
	back := r.FormValue("back")
	http.Redirect(w, r, safeNext(back), http.StatusSeeOther)
}

type adminUsersData struct {
	Users   []*db.User
	Usage   map[int64][2]int64
	Total   int
	Query   string
	Page    int
	Pages   int
	Default int64
}

func (a *App) adminUsers(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	pg, _ := strconv.Atoi(q.Get("page"))
	if pg < 1 {
		pg = 1
	}
	const per = 50
	users, total, err := a.db.Users(r.Context(), q.Get("q"), per, (pg-1)*per)
	if err != nil {
		a.serverError(w, r, err)
		return
	}
	usage := map[int64][2]int64{}
	for _, u := range users {
		b, n := a.db.UserStorage(r.Context(), u.ID)
		usage[u.ID] = [2]int64{b, int64(n)}
	}
	a.render(w, r, "admin_users", "Users", adminUsersData{Users: users, Usage: usage, Total: total, Query: q.Get("q"),
		Page: pg, Pages: (total + per - 1) / per, Default: a.cfg.DefaultQuota})
}

func (a *App) adminUserUpdate(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(r.PathValue("id"), 10, 64)
	u, err := a.db.UserByID(r.Context(), id)
	if err != nil {
		a.notFound(w, r)
		return
	}
	me := currentUser(r)
	switch r.FormValue("action") {
	case "status":
		st := r.FormValue("status")
		if (st == "active" || st == "locked" || st == "suspended") && u.ID != me.ID {
			u.Status = st
			u.StatusNote = trim(r.FormValue("note"), 300)
			a.db.UpdateUser(r.Context(), u)
			if st != "active" {
				a.db.DeleteUserSessions(r.Context(), u.ID)
			}
			// Suspended accounts' images go offline; anything else brings them back.
			a.db.ExecContext(r.Context(), `UPDATE images SET hidden=? WHERE user_id=?`, st == "suspended", u.ID)
		}
	case "role":
		if role := r.FormValue("role"); (role == "user" || role == "admin") && u.ID != me.ID {
			u.Role = role
			a.db.UpdateUser(r.Context(), u)
		}
	case "quota":
		if mb, err := strconv.ParseInt(r.FormValue("quota_mb"), 10, 64); err == nil && mb >= 0 {
			u.QuotaBytes = mb << 20
			a.db.UpdateUser(r.Context(), u)
		}
	case "warn":
		u.Warning = trim(strings.TrimSpace(r.FormValue("warning")), 1000)
		a.db.UpdateUser(r.Context(), u)
	case "password":
		if pw := r.FormValue("password"); len(pw) >= 8 {
			u.PassHash = HashPassword(pw)
			a.db.UpdateUser(r.Context(), u)
			a.db.DeleteUserSessions(r.Context(), u.ID)
		}
	case "delete":
		if u.ID != me.ID {
			a.purgeUser(r, u)
		}
	}
	http.Redirect(w, r, safeNext(r.FormValue("back")), http.StatusSeeOther)
}

type reportRow struct {
	db.Report
	Image *db.Image
	Count int
}

func (a *App) adminReports(w http.ResponseWriter, r *http.Request) {
	status := r.URL.Query().Get("status")
	if status != "dismissed" && status != "actioned" {
		status = "open"
	}
	reps, err := a.db.Reports(r.Context(), status)
	if err != nil {
		a.serverError(w, r, err)
		return
	}
	// One row per image, newest report first, with a count.
	var rows []*reportRow
	byImage := map[string]*reportRow{}
	for _, rp := range reps {
		if row := byImage[rp.ImageID]; row != nil {
			row.Count++
			continue
		}
		im, _ := a.db.Image(r.Context(), rp.ImageID)
		row := &reportRow{Report: rp, Image: im, Count: 1}
		byImage[rp.ImageID] = row
		rows = append(rows, row)
	}
	a.render(w, r, "admin_reports", "Reports", map[string]any{"Rows": rows, "Status": status})
}

func (a *App) adminReportAction(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(r.PathValue("id"), 10, 64)
	action := r.FormValue("action")
	status := "dismissed"
	if action == "hide" || action == "delete" {
		status = "actioned"
	}
	imageID, err := a.db.SetReportStatus(r.Context(), id, status)
	if err == nil {
		if im, err := a.db.Image(r.Context(), imageID); err == nil {
			switch action {
			case "hide":
				a.db.SetImageHidden(r.Context(), im.ID, true)
			case "delete":
				a.removeImage(r.Context(), im)
			}
		}
	}
	http.Redirect(w, r, "/admin/reports", http.StatusSeeOther)
}

func (a *App) adminBans(w http.ResponseWriter, r *http.Request) {
	bans, _ := a.db.Bans(r.Context())
	a.render(w, r, "admin_bans", "IP bans", map[string]any{"Bans": bans, "Error": r.URL.Query().Get("err")})
}

func (a *App) adminBan(w http.ResponseWriter, r *http.Request) {
	ip := strings.TrimSpace(r.FormValue("ip"))
	addr, err := netip.ParseAddr(ip)
	if err != nil {
		http.Redirect(w, r, "/admin/bans?err=That+isn't+an+IP+address", http.StatusSeeOther)
		return
	}
	a.db.Ban(r.Context(), addr.Unmap().String(), trim(r.FormValue("reason"), 200))
	a.invalidateBans()
	http.Redirect(w, r, "/admin/bans", http.StatusSeeOther)
}

func (a *App) adminUnban(w http.ResponseWriter, r *http.Request) {
	a.db.Unban(r.Context(), r.FormValue("ip"))
	a.invalidateBans()
	http.Redirect(w, r, "/admin/bans", http.StatusSeeOther)
}

type siteSettings struct {
	GuestUploads, Registration bool
	Announcement               string
	SEOTitle, SEODescription   string
	Saved                      bool
	MaxUploadMB, MaxBatch      int64
	InactiveDays               int
	Storage                    string
	Google                     bool
	TrustedProxies             int
}

func (a *App) adminSettings(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	a.render(w, r, "admin_settings", "Site settings", siteSettings{
		GuestUploads: a.db.Setting(ctx, "guest_uploads", fmt.Sprint(a.cfg.GuestUploads)) == "true",
		Registration: a.registrationOpen(ctx),
		Announcement: a.db.Setting(ctx, "announcement", ""),
		SEOTitle:     a.db.Setting(ctx, "seo_title", ""), SEODescription: a.db.Setting(ctx, "seo_description", ""),
		Saved:       r.URL.Query().Has("saved"),
		MaxUploadMB: a.cfg.MaxUploadBytes >> 20, MaxBatch: int64(a.cfg.MaxBatch), InactiveDays: a.cfg.InactiveDays,
		Storage: a.store.Name(), Google: a.cfg.GoogleEnabled(), TrustedProxies: len(a.cfg.TrustedProxies),
	})
}

func (a *App) adminSaveSettings(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	a.db.SetSetting(ctx, "guest_uploads", fmt.Sprint(r.FormValue("guest_uploads") == "on"))
	a.db.SetSetting(ctx, "registration", fmt.Sprint(r.FormValue("registration") == "on"))
	a.db.SetSetting(ctx, "announcement", trim(strings.TrimSpace(r.FormValue("announcement")), 300))
	a.db.SetSetting(ctx, "seo_title", trim(strings.TrimSpace(r.FormValue("seo_title")), 120))
	a.db.SetSetting(ctx, "seo_description", trim(strings.TrimSpace(r.FormValue("seo_description")), 300))
	http.Redirect(w, r, "/admin/settings?saved", http.StatusSeeOther)
}

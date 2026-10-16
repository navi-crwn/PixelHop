package app

import (
	"fmt"
	"math"
	"net/http"
	"net/netip"
	"net/url"
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
	var err2 error
	switch r.FormValue("action") {
	case "hide":
		err2 = a.db.SetImageHidden(r.Context(), im.ID, true)
	case "unhide":
		err2 = a.db.SetImageHidden(r.Context(), im.ID, false)
	case "delete":
		err2 = a.removeImage(r.Context(), im)
		a.db.DeleteEmptyAlbums(r.Context())
	case "ban":
		if im.IP != "" {
			if err2 = a.db.Ban(r.Context(), im.IP, "uploaded "+im.ID); err2 == nil {
				a.invalidateBans()
			}
		}
		if err2 == nil {
			err2 = a.removeImage(r.Context(), im)
		}
	}
	if err2 != nil {
		a.serverError(w, r, err2)
		return
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
	Notice  string
	Error   string
}

// adminUserMessages turn the short codes adminUserUpdate redirects with into
// the line shown above the table. Codes, not free text, so a crafted link
// can't put words on the admin page.
var adminUserMessages = map[string]string{
	"quota":        "Storage quota for %s is now %s.",
	"status":       "Status for %s saved.",
	"role":         "Role for %s saved.",
	"warn":         "Warning for %s saved.",
	"unwarn":       "Warning for %s cleared.",
	"password":     "New password set for %s; they were signed out everywhere.",
	"delete":       "%s and their files were deleted.",
	"bad-quota":    "Quota for %s not changed: type a number of GB, for example 10 or 2.5.",
	"self":         "You can't change the status, role or existence of your own account here.",
	"bad-password": "Password for %s not changed: use at least 8 characters.",
	"bad-value":    "Nothing changed for %s: that value isn't allowed.",
	"failed":       "Saving %s failed. Try again; if it keeps failing, check the server log.",
}

func (a *App) adminUserMessage(r *http.Request, code string) string {
	tpl, ok := adminUserMessages[code]
	if !ok {
		return ""
	}
	if !strings.Contains(tpl, "%s") {
		return tpl
	}
	who, quota := "The account", ""
	if id, err := strconv.ParseInt(r.URL.Query().Get("user"), 10, 64); err == nil {
		if u, err := a.db.UserByID(r.Context(), id); err == nil {
			who, quota = u.Email, humanBytes(u.QuotaBytes)
		}
	}
	if code == "quota" {
		return fmt.Sprintf(tpl, who, quota)
	}
	return fmt.Sprintf(tpl, who)
}

// parseQuotaGB reads a quota typed in GB. It takes "10", "2.5", "2,5" and
// "10 GB", and refuses anything else instead of guessing.
func parseQuotaGB(s string) (int64, bool) {
	s = strings.ToLower(strings.TrimSpace(s))
	s = strings.TrimSpace(strings.TrimSuffix(s, "gb"))
	s = strings.Replace(s, ",", ".", 1)
	gb, err := strconv.ParseFloat(s, 64)
	if err != nil || gb < 0 || gb > 1<<20 || math.IsNaN(gb) {
		return 0, false
	}
	return int64(math.Round(gb*1024)) << 20, true
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
		Page: pg, Pages: (total + per - 1) / per, Default: a.cfg.DefaultQuota,
		Notice: a.adminUserMessage(r, q.Get("done")), Error: a.adminUserMessage(r, q.Get("error"))})
}

func (a *App) adminUserUpdate(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(r.PathValue("id"), 10, 64)
	u, err := a.db.UserByID(r.Context(), id)
	if err != nil {
		a.notFound(w, r)
		return
	}
	me := currentUser(r)
	ctx := r.Context()
	// Every outcome is reported back: nothing is skipped without a word.
	done, problem := "", ""
	save := func(code string) {
		if err := a.db.UpdateUser(ctx, u); err != nil {
			a.log.Error("admin user update", "user", u.ID, "err", err)
			problem = "failed"
			return
		}
		done = code
	}
	action := r.FormValue("action")
	switch {
	case u.ID == me.ID && (action == "status" || action == "role" || action == "delete"):
		problem = "self"
	case action == "status":
		st := r.FormValue("status")
		if st != "active" && st != "locked" && st != "suspended" {
			problem = "bad-value"
			break
		}
		u.Status = st
		u.StatusNote = trim(r.FormValue("note"), 300)
		save("status")
		if problem == "" {
			if st != "active" {
				a.db.DeleteUserSessions(ctx, u.ID)
			}
			// Suspended accounts' images go offline; anything else brings them back.
			a.db.ExecContext(ctx, `UPDATE images SET hidden=? WHERE user_id=?`, st == "suspended", u.ID)
		}
	case action == "role":
		role := r.FormValue("role")
		if role != "user" && role != "admin" {
			problem = "bad-value"
			break
		}
		u.Role = role
		save("role")
	case action == "quota":
		raw := r.FormValue("quota_gb")
		q, ok := parseQuotaGB(raw)
		if raw == "" { // forms from before 3.5.2 sent megabytes
			mb, err := strconv.ParseInt(strings.TrimSpace(r.FormValue("quota_mb")), 10, 64)
			q, ok = mb<<20, err == nil && mb >= 0
		}
		if !ok {
			problem = "bad-quota"
			break
		}
		u.QuotaBytes = q
		save("quota")
	case action == "warn":
		u.Warning = trim(strings.TrimSpace(r.FormValue("warning")), 1000)
		if u.Warning == "" {
			save("unwarn")
		} else {
			save("warn")
		}
	case action == "password":
		pw := r.FormValue("password")
		if len(pw) < 8 {
			problem = "bad-password"
			break
		}
		u.PassHash = HashPassword(pw)
		save("password")
		if problem == "" {
			a.db.DeleteUserSessions(ctx, u.ID)
		}
	case action == "delete":
		a.purgeUser(r, u)
		done = "delete"
	default:
		problem = "bad-value"
	}
	back, _ := url.Parse(safeNext(r.FormValue("back")))
	v := back.Query()
	v.Del("done")
	v.Del("error")
	v.Set("user", strconv.FormatInt(u.ID, 10))
	if problem != "" {
		v.Set("error", problem)
	} else {
		v.Set("done", done)
	}
	back.RawQuery = v.Encode()
	http.Redirect(w, r, back.String(), http.StatusSeeOther)
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
	a.render(w, r, "admin_bans", "IP bans", map[string]any{"Bans": bans, "Error": adminBanMessages[r.URL.Query().Get("err")], "Notice": adminBanMessages[r.URL.Query().Get("ok")]})
}

func (a *App) adminBan(w http.ResponseWriter, r *http.Request) {
	ip := strings.TrimSpace(r.FormValue("ip"))
	addr, err := netip.ParseAddr(ip)
	if err != nil {
		http.Redirect(w, r, "/admin/bans?err=not-ip", http.StatusSeeOther)
		return
	}
	if err := a.db.Ban(r.Context(), addr.Unmap().String(), trim(r.FormValue("reason"), 200)); err != nil {
		a.log.Error("ban", "err", err)
		http.Redirect(w, r, "/admin/bans?err=failed", http.StatusSeeOther)
		return
	}
	a.invalidateBans()
	http.Redirect(w, r, "/admin/bans?ok=banned", http.StatusSeeOther)
}

var adminBanMessages = map[string]string{
	"banned":   "IP address banned.",
	"unbanned": "Ban removed.",
	"not-ip":   "That isn't an IP address. Use one like 203.0.113.7 or 2001:db8::1.",
	"failed":   "Saving failed. Try again; if it keeps failing, check the server log.",
}

func (a *App) adminUnban(w http.ResponseWriter, r *http.Request) {
	if err := a.db.Unban(r.Context(), r.FormValue("ip")); err != nil {
		a.log.Error("unban", "err", err)
		http.Redirect(w, r, "/admin/bans?err=failed", http.StatusSeeOther)
		return
	}
	a.invalidateBans()
	http.Redirect(w, r, "/admin/bans?ok=unbanned", http.StatusSeeOther)
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
	for k, v := range map[string]string{
		"guest_uploads":   fmt.Sprint(r.FormValue("guest_uploads") == "on"),
		"registration":    fmt.Sprint(r.FormValue("registration") == "on"),
		"announcement":    trim(strings.TrimSpace(r.FormValue("announcement")), 300),
		"seo_title":       trim(strings.TrimSpace(r.FormValue("seo_title")), 120),
		"seo_description": trim(strings.TrimSpace(r.FormValue("seo_description")), 300),
	} {
		if err := a.db.SetSetting(ctx, k, v); err != nil {
			a.serverError(w, r, err)
			return
		}
	}
	http.Redirect(w, r, "/admin/settings?saved", http.StatusSeeOther)
}

package app

import "net/http"

// adminRetired is what is left of the built-in admin pages. Moderation and
// settings moved to the HOP control center, which uses the signed /hop/*
// API; an admin who still opens /admin is sent there when its address is
// known, and told where things went otherwise. The admin() guard in front
// keeps the usual sign-in redirect and 403 for everyone else.
func (a *App) adminRetired(w http.ResponseWriter, r *http.Request) {
	if a.cfg.ControlURL != "" {
		http.Redirect(w, r, a.cfg.ControlURL, http.StatusSeeOther)
		return
	}
	a.render(w, r, "admin_retired", "Admin has moved", nil)
}

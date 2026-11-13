package app

import "net/http"

// legacyPages sends PixelHop 2 page addresses to their PixelHop 3 homes, so
// bookmarks and search results keep working.
var legacyPages = map[string]string{
	"/gallery":       "/dashboard",
	"/gallery.php":   "/dashboard",
	"/explore":       "/",
	"/dashboard.php": "/dashboard",
	"/features.php":  "/features",
	"/index.php":     "/",
	"/contact.php":   "/contact",
	"/help.php":      "/help",
	"/changelog.php": "/changelog",
	"/dmca":          "/legal#takedown",
	"/dmca.php":      "/legal#takedown",
	"/privacy":       "/legal#privacy",
	"/privacy.php":   "/legal#privacy",
	"/terms":         "/legal",
	"/terms.php":     "/legal",
	"/docs.php":      "/docs",
	"/tools.php":     "/tools",
	"/login.php":     "/login",
	"/register.php":  "/register",
	"/admin.php":     "/admin",
	"/compress":      "/tools#compress",
	"/resize":        "/tools#resize",
	"/crop":          "/tools#crop",
	"/convert":       "/tools#convert",
	"/ocr":           "/tools#ocr",
	"/rembg":         "/tools#rembg",
	"/upscale":       "/tools#upscale",
	"/erase":         "/tools#erase",
	"/faceblur":      "/tools#faces",
	"/palette":       "/tools#palette",
	// "/qr" (the studio's old QR tool) is answered by qrMoved, which knows whether LinkHop is around.
	"/member/":         "/dashboard",
	"/member/upload":   "/",
	"/member/tools":    "/tools",
	"/member/settings": "/settings",
	"/member/welcome":  "/dashboard?welcome=1",
	"/member/result":   "/dashboard",
	"/member/compress": "/tools#compress",
	"/member/resize":   "/tools#resize",
	"/member/crop":     "/tools#crop",
	"/member/convert":  "/tools#convert",
	"/member/ocr":      "/tools#ocr",
	"/member/rembg":    "/tools#rembg",
}

// linkhopQR is where the studio's old QR tool went: LinkHop's QR page.
const linkhopQR = "https://link.hel.ink/qr"

// qrMoved answers the studio's old /qr address. A copy connected to HEL.ink points at LinkHop's QR
// page; a stand-alone copy has no LinkHop to send people to and goes to its own studio. Temporary
// (302), so a browser does not remember a guess that depends on how this copy is set up.
func (a *App) qrMoved(w http.ResponseWriter, r *http.Request) {
	to := "/tools"
	if a.cfg.HelinkURL != "" {
		to = linkhopQR
	}
	http.Redirect(w, r, to, http.StatusFound)
}

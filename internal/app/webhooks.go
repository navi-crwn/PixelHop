package app

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/netip"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/navi-crwn/pixelhop/internal/db"
	"github.com/navi-crwn/pixelhop/internal/netsafe"
)

// Webhooks tell your own service when you upload or delete an image.
// POST JSON {"event","created_at","data"} with header
// X-PixelHop-Signature: sha256=<hex HMAC of the body with your secret>.
// Delivery goes through the same SSRF guard as "upload from link".

var webhookEvents = map[string]bool{"image.uploaded": true, "image.deleted": true}

func (a *App) fireWebhooks(userID int64, event string, data any) {
	if userID == 0 {
		return
	}
	hooks, err := a.db.Webhooks(context.Background(), userID)
	if err != nil || len(hooks) == 0 {
		return
	}
	body, _ := json.Marshal(map[string]any{"event": event, "created_at": time.Now().Unix(), "data": data})
	for _, h := range hooks {
		if !strings.Contains(","+h.Events+",", ","+event+",") {
			continue
		}
		go a.deliver(h, body)
	}
}

func (a *App) deliver(h db.Webhook, body []byte) {
	mac := hmac.New(sha256.New, []byte(h.Secret))
	mac.Write(body)
	sig := "sha256=" + hex.EncodeToString(mac.Sum(nil))
	status := ""
	for attempt := 0; attempt < 3; attempt++ {
		if attempt > 0 {
			time.Sleep(time.Duration(attempt*attempt) * 10 * time.Second)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		code, err := netsafe.Post(ctx, h.URL, "application/json", body, map[string]string{
			"X-PixelHop-Signature": sig, "User-Agent": "PixelHop-Webhook/1"})
		cancel()
		if err == nil && code/100 == 2 {
			status = fmt.Sprintf("%d at %s", code, time.Now().UTC().Format("2006-01-02 15:04"))
			break
		}
		if err != nil {
			status = "failed: " + err.Error()
		} else {
			status = fmt.Sprintf("failed: HTTP %d", code)
		}
	}
	a.db.SetWebhookStatus(context.Background(), h.ID, status)
}

func (a *App) createWebhook(w http.ResponseWriter, r *http.Request) {
	u := currentUser(r)
	raw := strings.TrimSpace(r.FormValue("url"))
	if err := a.checkWebhookURL(raw); err != nil {
		a.settingsError(w, r, err.Error())
		return
	}
	if hooks, _ := a.db.Webhooks(r.Context(), u.ID); len(hooks) >= 5 {
		a.settingsError(w, r, "You can have up to 5 webhooks.")
		return
	}
	var events []string
	for _, e := range r.Form["events"] {
		if webhookEvents[e] {
			events = append(events, e)
		}
	}
	if len(events) == 0 {
		events = []string{"image.uploaded", "image.deleted"}
	}
	h := &db.Webhook{UserID: u.ID, URL: raw, Secret: "whsec_" + randomToken(24), Events: strings.Join(events, ",")}
	if err := a.db.InsertWebhook(r.Context(), h); err != nil {
		a.serverError(w, r, err)
		return
	}
	d := a.settingsData(r)
	d.NewWebhookSecret = h.Secret
	a.render(w, r, "settings", "Settings", d)
}

func (a *App) deleteWebhook(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(r.PathValue("id"), 10, 64)
	a.db.DeleteWebhook(r.Context(), currentUser(r).ID, id)
	http.Redirect(w, r, "/settings?ok=webhook-deleted#webhooks", http.StatusSeeOther)
}

func (a *App) testWebhook(w http.ResponseWriter, r *http.Request) {
	u := currentUser(r)
	if hooks, _ := a.db.Webhooks(r.Context(), u.ID); len(hooks) == 0 {
		a.settingsError(w, r, "Add a webhook first; there is nothing to send a test to yet.")
		return
	}
	a.fireWebhooks(u.ID, "image.uploaded", map[string]any{"test": true, "id": "example-x7Kp2Q",
		"url": a.cfg.LinkURL + "/i/example-x7Kp2Q.jpg", "view_url": a.cfg.LinkURL + "/example-x7Kp2Q"})
	http.Redirect(w, r, "/settings?ok=webhook-test#webhooks", http.StatusSeeOther)
}

// checkWebhookURL refuses obviously internal targets up front; netsafe also
// checks every address at connect time, so DNS tricks fail later anyway.
func (a *App) checkWebhookURL(raw string) error {
	pu, err := url.Parse(raw)
	if err != nil || pu.Scheme != "https" || pu.Host == "" || len(raw) > 500 {
		return errors.New("Webhook addresses must start with https://")
	}
	host := strings.ToLower(pu.Hostname())
	if host == "localhost" || strings.HasSuffix(host, ".localhost") || strings.HasSuffix(host, ".internal") || strings.HasSuffix(host, ".local") {
		return errors.New("That address is not allowed.")
	}
	if ip, err := netip.ParseAddr(host); err == nil && !netsafe.Public(ip) {
		return errors.New("That address is not allowed.")
	}
	return nil
}

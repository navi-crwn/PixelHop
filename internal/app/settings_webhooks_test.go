package app

import (
	"net/http"
	"net/url"
	"strings"
	"testing"
)

// /docs sends people to Settings to add a webhook, but the card was hidden for
// HOP ID members (everyone on hel.ink) together with the password form. The
// webhooks are PixelHop's own; the password belongs to the hub.
func TestWebhooksCardForHopIDMembers(t *testing.T) {
	a, h := ssoApp(t)
	hop := linked(t, a, "78", "hook@example.com")
	s := sessionFor(t, a, hop.ID)
	body := fetch(h, "/settings", s).Body.String()
	if !strings.Contains(body, `id="webhooks"`) || !strings.Contains(body, `action="/settings/webhooks"`) {
		t.Fatal("HOP ID member's settings have no Webhooks card")
	}
	if strings.Contains(body, `action="/settings/password"`) {
		t.Error("HOP ID member's settings offer a PixelHop password; it belongs to the hub")
	}
	// Adding one works from there and shows the secret once.
	rec := post(h, "/settings/webhooks", url.Values{"url": {"https://hooks.example.com/pixelhop"}}, s)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "whsec_") || !strings.Contains(rec.Body.String(), "https://hooks.example.com/pixelhop") {
		t.Fatalf("add webhook: %d", rec.Code)
	}

	local, _ := member(t, a, "hooklocal@example.com")
	body = fetch(h, "/settings", sessionFor(t, a, local.ID)).Body.String()
	if !strings.Contains(body, `id="webhooks"`) || !strings.Contains(body, `action="/settings/password"`) {
		t.Error("local member's settings lost the Webhooks card or the password form")
	}
}

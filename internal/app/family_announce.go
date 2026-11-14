package app

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

// The family announcement: the hub publishes {hub}/announcements.json; the server of each app reads it
// (never the browser), keeps it five minutes, and shows at most one card on the dashboard. A hub that is
// down or slow changes nothing for the person: the page never waits for it.

const (
	famAnnTTL     = 5 * time.Minute
	famAnnRetry   = time.Minute // after a failed trip
	famAnnTimeout = 2 * time.Second
	famAnnMaxBody = 256 << 10
)

var famAnnID = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)

// FamilyAnnouncement is the one card, already checked.
type FamilyAnnouncement struct {
	ID, Level, Title, Body, URL string
}

// Closable reports whether the person may close the card (a critical notice stays).
func (f *FamilyAnnouncement) Closable() bool { return f.Level != "critical" }

type famAnnCache struct {
	mu         sync.Mutex
	items      []FamilyAnnouncement // the last good answer, in the order the hub sent
	freshUntil time.Time
	retryAfter time.Time
	busy       bool
}

// familyAnnouncement is the card to show now, or nil. It only reads the cache; an expired cache starts a
// refresh in the background and the stale copy (or nothing) is used meanwhile.
func (a *App) familyAnnouncement() *FamilyAnnouncement {
	if !a.cfg.SSOEnabled() {
		return nil // no hub: nothing is fetched
	}
	c := &a.famAnn
	c.mu.Lock()
	defer c.mu.Unlock()
	now := time.Now()
	if now.After(c.freshUntil) && now.After(c.retryAfter) && !c.busy {
		c.busy = true
		go a.refreshFamilyAnnouncements()
	}
	return pickFamilyAnnouncement(c.items)
}

// pickFamilyAnnouncement chooses the one card: a critical one first, else the newest (the hub sends them newest first).
func pickFamilyAnnouncement(items []FamilyAnnouncement) *FamilyAnnouncement {
	for i := range items {
		if items[i].Level == "critical" {
			return &items[i]
		}
	}
	if len(items) > 0 {
		return &items[0]
	}
	return nil
}

// refreshFamilyAnnouncements asks the hub (2 s at most) and keeps the answer; on failure the old copy stays.
func (a *App) refreshFamilyAnnouncements() {
	items, err := a.fetchFamilyAnnouncements(context.Background())
	c := &a.famAnn
	c.mu.Lock()
	defer c.mu.Unlock()
	c.busy = false
	now := time.Now()
	if err != nil {
		c.retryAfter = now.Add(famAnnRetry)
		return
	}
	c.items, c.freshUntil = items, now.Add(famAnnTTL)
}

func (a *App) fetchFamilyAnnouncements(ctx context.Context) ([]FamilyAnnouncement, error) {
	ctx, cancel := context.WithTimeout(ctx, famAnnTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(a.cfg.HelinkURL, "/")+"/announcements.json?app=pixelhop", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	client := a.annClient
	if client == nil {
		client = &http.Client{Timeout: famAnnTimeout, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, http.ErrNotSupported
	}
	var doc struct {
		Items []struct {
			ID    any      `json:"id"`
			Level string   `json:"level"`
			Title string   `json:"title"`
			Body  string   `json:"body"`
			URL   string   `json:"url"`
			Apps  []string `json:"apps"`
		} `json:"items"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, famAnnMaxBody)).Decode(&doc); err != nil {
		return nil, err
	}
	var out []FamilyAnnouncement
	for _, it := range doc.Items {
		id := anyToID(it.ID)
		if !famAnnID.MatchString(id) {
			continue // an id that could carry markup or a path is not shown
		}
		if len(it.Apps) > 0 && !appsInclude(it.Apps, "pixelhop") {
			continue
		}
		level := it.Level
		if level != "warning" && level != "critical" {
			level = "info"
		}
		f := FamilyAnnouncement{ID: id, Level: level, Title: trim(strings.TrimSpace(it.Title), 100), Body: trim(strings.TrimSpace(it.Body), 300)}
		if f.Title == "" && f.Body == "" {
			continue
		}
		if u, err := url.Parse(it.URL); err == nil && hopURL(it.URL, a.cfg.HelinkURL) && u.Scheme == "https" {
			f.URL = it.URL // a link only to an https address on the family's own domain
		}
		out = append(out, f)
		if len(out) == 20 {
			break
		}
	}
	return out, nil
}

func appsInclude(apps []string, slug string) bool {
	for _, a := range apps {
		if a == "*" || a == slug {
			return true
		}
	}
	return false
}

func anyToID(v any) string {
	switch x := v.(type) {
	case string:
		return x
	case float64:
		return strconv.FormatFloat(x, 'f', -1, 64)
	}
	return ""
}

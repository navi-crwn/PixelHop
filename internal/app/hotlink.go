package app

import (
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// Owners can switch off embedding of their images on other websites. A page
// on this site (or the short-link host) may still show them, and so may a
// browser that sends no Referer, such as someone opening the link directly or
// a chat app fetching a preview.
type hotlinkCache struct {
	mu sync.Mutex
	m  map[int64]hotlinkEntry
}

type hotlinkEntry struct {
	block bool
	until time.Time
}

func (c *hotlinkCache) get(id int64) (bool, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.m[id]
	if !ok || time.Now().After(e.until) {
		return false, false
	}
	return e.block, true
}

func (c *hotlinkCache) set(id int64, block bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.m == nil {
		c.m = map[int64]hotlinkEntry{}
	}
	if len(c.m) > 10000 {
		c.m = map[int64]hotlinkEntry{}
	}
	c.m[id] = hotlinkEntry{block: block, until: time.Now().Add(time.Minute)}
}

// hotlinkRefused tells whether this request embeds the owner's file from a site they blocked.
func (a *App) hotlinkRefused(w http.ResponseWriter, r *http.Request, ownerID int64) bool {
	block, ok := a.hotlinks.get(ownerID)
	if !ok {
		block = a.db.BlockHotlinks(r.Context(), ownerID)
		a.hotlinks.set(ownerID, block)
	}
	if !block {
		return false
	}
	w.Header().Add("Vary", "Referer") // a CDN must not hand one answer to every site
	ref := r.Referer()
	if ref == "" {
		return false
	}
	u, err := url.Parse(ref)
	if err != nil || u.Host == "" {
		return false
	}
	host := strings.ToLower(u.Hostname())
	for _, own := range []string{r.Host, a.cfg.BaseURL, a.cfg.LinkURL} {
		if own == "" {
			continue
		}
		if ou, err := url.Parse(own); err == nil && ou.Host != "" {
			own = ou.Host
		}
		if strings.EqualFold(strings.Split(own, ":")[0], host) {
			return false
		}
	}
	return true
}

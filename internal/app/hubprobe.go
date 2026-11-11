package app

import (
	"net/http"
	"strings"
	"sync"
	"time"
)

// hubProbe says whether the hub is really answering, so a re-check never sends a browser to a hub
// that is down (it would land on an error page) and a session is never ended because the hub is away.
// An open TLS port is not enough (the hub's nginx stays up when its PHP is dead), so the probe is one
// GET of <hub>/up that must return 2xx within a short timeout, redirects not followed. The answer,
// good or bad, is kept for a minute.
type hubProbe struct {
	url    string
	mu     sync.Mutex
	at     time.Time
	up     bool
	client *http.Client
	now    func() time.Time // replaced in tests
}

const hubProbeTTL = time.Minute

func newHubProbe(hub string) *hubProbe {
	return &hubProbe{url: strings.TrimRight(hub, "/") + "/up", now: time.Now,
		client: &http.Client{Timeout: 1500 * time.Millisecond,
			CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}
}

func (p *hubProbe) Up() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	if !p.at.IsZero() && p.now().Sub(p.at) < hubProbeTTL {
		return p.up
	}
	up := false
	if res, err := p.client.Get(p.url); err == nil {
		res.Body.Close()
		up = res.StatusCode >= 200 && res.StatusCode < 300
	}
	p.at, p.up = p.now(), up
	return up
}

package app

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync/atomic"
	"testing"
	"time"
)

// The hub counts as reachable only when /up answers 2xx quickly; the answer is kept for a minute.
func TestHubProbeNeedsA2xxAndRecovers(t *testing.T) {
	var status atomic.Int32
	var hits atomic.Int32
	var slow atomic.Bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		if r.URL.Path != "/up" || r.Method != http.MethodGet {
			t.Errorf("probe asked %s %s", r.Method, r.URL.Path)
		}
		if slow.Load() {
			time.Sleep(300 * time.Millisecond)
		}
		w.WriteHeader(int(status.Load()))
	}))
	defer srv.Close()
	clock := time.Now()
	p := newHubProbe(srv.URL)
	p.now = func() time.Time { return clock }
	p.client.Timeout = 100 * time.Millisecond
	step := func() { clock = clock.Add(hubProbeTTL + time.Second) }

	status.Store(200)
	if !p.Up() {
		t.Fatal("hub answering 200 counted as down")
	}
	// Cached: a hub that fails inside the minute does not change the answer, and costs no request.
	status.Store(502)
	n := hits.Load()
	if !p.Up() || hits.Load() != n {
		t.Fatal("answer not cached")
	}
	step()
	if p.Up() {
		t.Fatal("hub answering 502 (nginx up, PHP dead) counted as reachable")
	}
	// Down is cached too, so a dead hub is not asked on every page load.
	n = hits.Load()
	status.Store(200)
	if p.Up() || hits.Load() != n {
		t.Fatal("down answer not cached")
	}
	step()
	if !p.Up() {
		t.Fatal("did not recover after the hub came back")
	}
	// A redirect (a login page, a maintenance page) is not a healthy answer.
	status.Store(302)
	step()
	if p.Up() {
		t.Fatal("302 counted as reachable")
	}
	// Too slow is down.
	status.Store(200)
	slow.Store(true)
	step()
	if p.Up() {
		t.Fatal("a hub slower than the timeout counted as reachable")
	}
	slow.Store(false)
	step()
	if !p.Up() {
		t.Fatal("did not recover after the slow spell")
	}
	// Nothing listening at all.
	srv.Close()
	step()
	if p.Up() {
		t.Fatal("closed port counted as reachable")
	}
}

// While the hub is down a stale HOP ID session stays trusted: nobody is sent to the hub, signed out or locked out.
func TestHubDownKeepsTheSession(t *testing.T) {
	var status atomic.Int32
	status.Store(200)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(int(status.Load())) }))
	defer srv.Close()
	a, h := ssoApp(t)
	clock := time.Now()
	p := newHubProbe(srv.URL)
	p.now = func() time.Time { return clock }
	a.hubUp = p.Up
	if _, err := url.Parse(srv.URL); err != nil {
		t.Fatal(err)
	}
	b := newBrowser(t, h)
	b.signIn("42", "a@example.com")
	b.age(10*time.Minute, a)

	status.Store(502)
	for i := 0; i < 3; i++ {
		clock = clock.Add(hubProbeTTL + time.Second)
		if rec := b.page("/dashboard"); rec.Code != 200 {
			t.Fatalf("hub 502, page load %d: %d %s", i, rec.Code, rec.Header().Get("Location"))
		}
		if b.email() != "a@example.com" {
			t.Fatal("signed out while the hub was down")
		}
	}
	// The hub is back: the next page load re-checks.
	status.Store(200)
	clock = clock.Add(hubProbeTTL + time.Second)
	delete(b.cookies, recheckCookie)
	if rec := b.page("/dashboard"); rec.Code != http.StatusSeeOther {
		t.Fatalf("no re-check after the hub recovered: %d", rec.Code)
	}
}

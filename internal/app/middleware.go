package app

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"runtime/debug"
	"strings"
	"sync"
	"time"
)

type ctxKey int

const (
	keyIP ctxKey = iota
	keyUser
	keyViaAPIKey
)

func clientIP(r *http.Request) string {
	ip, _ := r.Context().Value(keyIP).(string)
	return ip
}

// withIP works out the visitor's address. Forwarding headers are only
// believed when the connection comes from a configured trusted proxy;
// otherwise anyone could pick their own IP and dodge limits and bans.
func (a *App) withIP(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		host, _, err := net.SplitHostPort(r.RemoteAddr)
		if err != nil {
			host = r.RemoteAddr
		}
		ip := host
		if peer, err := netip.ParseAddr(host); err == nil && a.trusted(peer) {
			if cf := r.Header.Get("CF-Connecting-IP"); cf != "" {
				ip = cf
			} else if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
				// Walk from the right, skipping our own proxies.
				parts := strings.Split(xff, ",")
				for i := len(parts) - 1; i >= 0; i-- {
					p := strings.TrimSpace(parts[i])
					addr, err := netip.ParseAddr(p)
					if err != nil {
						break
					}
					ip = p
					if !a.trusted(addr) {
						break
					}
				}
			} else if xr := r.Header.Get("X-Real-IP"); xr != "" {
				ip = xr
			}
		}
		if addr, err := netip.ParseAddr(ip); err == nil {
			ip = addr.Unmap().String()
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), keyIP, ip)))
	})
}

func (a *App) trusted(ip netip.Addr) bool {
	ip = ip.Unmap()
	if ip.IsLoopback() && len(a.cfg.TrustedProxies) > 0 {
		return true
	}
	for _, p := range a.cfg.TrustedProxies {
		if p.Contains(ip) {
			return true
		}
	}
	return false
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("Referrer-Policy", "strict-origin-when-cross-origin")
		h.Set("X-Frame-Options", "SAMEORIGIN")
		h.Set("Permissions-Policy", "camera=(), microphone=(), geolocation=(), interest-cohort=()")
		next.ServeHTTP(w, r)
	})
}

// csrf rejects cross-site form posts made with the visitor's cookies.
// Requests that authenticate with an API key don't use cookies at all, so
// they skip the check and may come from anywhere (CORS is allowed for them).
func csrf(next http.Handler) http.Handler {
	cop := http.NewCrossOriginProtection()
	protected := cop.Handler(next)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/hop/") { // signed server-to-server calls, no cookies
			next.ServeHTTP(w, r)
			return
		}
		if strings.HasPrefix(r.URL.Path, "/api/") {
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PATCH, DELETE")
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			if r.Header.Get("Authorization") != "" {
				next.ServeHTTP(w, r)
				return
			}
		}
		protected.ServeHTTP(w, r)
	})
}

// banList caches the IP ban table; it is reloaded every 30s or right after
// an admin changes it.
type banList struct {
	mu     sync.Mutex
	ips    map[string]bool
	loaded time.Time
}

func (a *App) banned(r *http.Request) bool {
	b := &a.bans
	b.mu.Lock()
	defer b.mu.Unlock()
	if time.Since(b.loaded) > 30*time.Second {
		b.ips = map[string]bool{}
		if bans, err := a.db.Bans(r.Context()); err == nil {
			for _, x := range bans {
				b.ips[x.IP] = true
			}
		}
		b.loaded = time.Now()
	}
	return b.ips[clientIP(r)]
}

func (a *App) invalidateBans() {
	a.bans.mu.Lock()
	a.bans.loaded = time.Time{}
	a.bans.mu.Unlock()
}

// checkBan stops banned addresses from changing anything; they can still
// look at pages and images.
func (a *App) checkBan(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead && !strings.HasPrefix(r.URL.Path, "/hop/") && a.banned(r) {
			a.fail(w, r, http.StatusForbidden, "This address is blocked. If you think this is a mistake, contact the site owner.")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (a *App) recoverer(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if v := recover(); v != nil {
				if v == http.ErrAbortHandler {
					panic(v)
				}
				a.log.Error("panic", "err", fmt.Sprint(v), "path", r.URL.Path, "stack", string(debug.Stack()))
				http.Error(w, "Something went wrong on our side.", http.StatusInternalServerError)
			}
		}()
		next.ServeHTTP(w, r)
	})
}

type statusWriter struct {
	http.ResponseWriter
	status int
	n      int64
}

func (s *statusWriter) WriteHeader(c int) { s.status = c; s.ResponseWriter.WriteHeader(c) }
func (s *statusWriter) Write(b []byte) (int, error) {
	if s.status == 0 {
		s.status = 200
	}
	n, err := s.ResponseWriter.Write(b)
	s.n += int64(n)
	return n, err
}
func (s *statusWriter) Unwrap() http.ResponseWriter { return s.ResponseWriter }

func (a *App) accessLog(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		sw := &statusWriter{ResponseWriter: w}
		next.ServeHTTP(sw, r)
		if strings.HasPrefix(r.URL.Path, "/static/") || r.URL.Path == "/healthz" {
			return
		}
		a.log.Info("req", "m", r.Method, "p", r.URL.Path, "s", sw.status, "b", sw.n, "ms", time.Since(start).Milliseconds())
	})
}

// ---- rate limiting ----

// limiter is a fixed-window counter per (action, key). It lives in memory:
// PixelHop is one process, and limits resetting on restart is harmless.
type limiter struct {
	mu sync.Mutex
	m  map[string]*window
}

type window struct {
	start time.Time
	n     int
}

func newLimiter() *limiter {
	l := &limiter{m: map[string]*window{}}
	go func() {
		for range time.Tick(5 * time.Minute) {
			l.mu.Lock()
			for k, w := range l.m {
				if time.Since(w.start) > time.Hour {
					delete(l.m, k)
				}
			}
			l.mu.Unlock()
		}
	}()
	return l
}

// allow counts one event and reports whether key is still under max per per.
func (l *limiter) allow(key string, max int, per time.Duration) bool {
	if max <= 0 {
		return true
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	w := l.m[key]
	if w == nil || time.Since(w.start) > per {
		w = &window{start: time.Now()}
		l.m[key] = w
	}
	w.n++
	return w.n <= max
}

// seen remembers (ip, image) pairs so one visitor refreshing a page counts
// as one view per hour.
type seen struct {
	mu  sync.Mutex
	m   map[string]time.Time
	ttl time.Duration
}

func newSeen(ttl time.Duration) *seen {
	s := &seen{m: map[string]time.Time{}, ttl: ttl}
	go func() {
		for range time.Tick(10 * time.Minute) {
			s.mu.Lock()
			for k, t := range s.m {
				if time.Since(t) > s.ttl {
					delete(s.m, k)
				}
			}
			s.mu.Unlock()
		}
	}()
	return s
}

func (s *seen) first(key string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if t, ok := s.m[key]; ok && time.Since(t) < s.ttl {
		return false
	}
	s.m[key] = time.Now()
	return true
}

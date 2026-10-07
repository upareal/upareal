// story: e03s02
package web

import (
	"net"
	"net/http"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

const (
	authRequestsPerMinute = 10
	limiterIdleTTL        = 10 * time.Minute
	limiterPruneAbove     = 1000 // prune idle entries once the map is this big
)

// ipLimiter gives each client IP its own token bucket. In memory: correct for
// the single-instance deployment.
// ponytail: keys on r.RemoteAddr; behind Caddy (e10) this must use the
// forwarded client IP from the trusted proxy only.
type ipLimiter struct {
	mu      sync.Mutex
	entries map[string]*limiterEntry
	now     func() time.Time
}

type limiterEntry struct {
	lim      *rate.Limiter
	lastSeen time.Time
}

func newIPLimiter() *ipLimiter {
	return &ipLimiter{entries: map[string]*limiterEntry{}, now: time.Now}
}

func (l *ipLimiter) allow(ip string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	if len(l.entries) > limiterPruneAbove {
		for k, e := range l.entries {
			if now.Sub(e.lastSeen) > limiterIdleTTL {
				delete(l.entries, k)
			}
		}
	}
	e, ok := l.entries[ip]
	if !ok {
		e = &limiterEntry{lim: rate.NewLimiter(rate.Every(time.Minute/authRequestsPerMinute), authRequestsPerMinute)}
		l.entries[ip] = e
	}
	e.lastSeen = now
	return e.lim.AllowN(now, 1)
}

// limit rejects requests over the per-IP budget with 429.
func (l *ipLimiter) limit(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !l.allow(clientIP(r)) {
			w.Header().Set("Retry-After", "60")
			renderError(w, r, http.StatusTooManyRequests, "Muitas tentativas",
				"Muitas tentativas. Aguarde um minuto e tente novamente.")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

package middleware

import (
	"net/http"
	"net/netip"
	"strconv"
	"strings"
	"sync"
	"time"
)

// maxTrackedClients bounds the limiter's memory. When the table is full and
// expired entries cannot be evicted, new clients are let through untracked
// rather than locking everyone out.
const maxTrackedClients = 100_000

type rateLimitEntry struct {
	requests int
	reset    time.Time
}

type ipRateLimiter struct {
	mu             sync.Mutex
	clients        map[netip.Addr]rateLimitEntry
	limit          int
	window         time.Duration
	lastCleanup    time.Time
	trustedProxies []netip.Prefix
}

func newIPRateLimiter(limit int, window time.Duration, trustedProxies []netip.Prefix) *ipRateLimiter {
	return &ipRateLimiter{
		clients:        make(map[netip.Addr]rateLimitEntry),
		limit:          limit,
		window:         window,
		lastCleanup:    time.Now(),
		trustedProxies: trustedProxies,
	}
}

// Handler limits requests per client IP (see clientIP). Static assets and the
// health check are exempt: a single page view fetches several assets, and
// probes must never be throttled.
func (l *ipRateLimiter) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if rateLimitExempt(r.URL.Path) {
			next.ServeHTTP(w, r)
			return
		}

		now := time.Now()
		key := rateLimitKey(clientIP(r, l.trustedProxies))

		l.mu.Lock()
		// Sweeping at most once per window keeps the cost amortized O(1).
		if now.Sub(l.lastCleanup) >= l.window {
			for client, entry := range l.clients {
				if !now.Before(entry.reset) {
					delete(l.clients, client)
				}
			}
			l.lastCleanup = now
		}

		entry, exists := l.clients[key]
		if !exists || !now.Before(entry.reset) {
			entry = rateLimitEntry{reset: now.Add(l.window)}
		}

		allowed := entry.requests < l.limit
		if allowed {
			entry.requests++
		}
		if exists || len(l.clients) < maxTrackedClients {
			l.clients[key] = entry
		}
		remaining := max(l.limit-entry.requests, 0)
		l.mu.Unlock()

		w.Header().Set("X-RateLimit-Limit", strconv.Itoa(l.limit))
		w.Header().Set("X-RateLimit-Remaining", strconv.Itoa(remaining))
		w.Header().Set("X-RateLimit-Reset", strconv.FormatInt(entry.reset.Unix(), 10))
		if !allowed {
			retryAfter := max(int(time.Until(entry.reset).Seconds()), 1)
			w.Header().Set("Retry-After", strconv.Itoa(retryAfter))
			http.Error(w, "Too Many Requests", http.StatusTooManyRequests)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func rateLimitExempt(path string) bool {
	return path == "/healthz" || strings.HasPrefix(path, "/static/")
}

// rateLimitKey groups IPv6 clients by /64, the smallest block typically
// assigned to a single subscriber, so rotating addresses doesn't evade limits.
func rateLimitKey(addr netip.Addr) netip.Addr {
	if addr.Is6() {
		if prefix, err := addr.Prefix(64); err == nil {
			return prefix.Addr()
		}
	}
	return addr
}

// clientIP returns the address of the client that made the request.
//
// The TCP peer is used unless it belongs to a trusted proxy. In that case
// X-Forwarded-For is walked right to left (the order proxies append to it),
// skipping trusted hops, and the first untrusted address wins. Entries left of
// that point are client-controlled and ignored, so they cannot be spoofed.
func clientIP(r *http.Request, trustedProxies []netip.Prefix) netip.Addr {
	peer := parseIP(r.RemoteAddr)
	if !isTrusted(peer, trustedProxies) {
		return peer
	}

	client := peer
	values := r.Header.Values("X-Forwarded-For")
	for i := len(values) - 1; i >= 0; i-- {
		hops := strings.Split(values[i], ",")
		for j := len(hops) - 1; j >= 0; j-- {
			hop := parseIP(strings.TrimSpace(hops[j]))
			if !hop.IsValid() {
				// Garbage can only come from beyond the trusted chain; stop at
				// the last address a trusted proxy vouched for.
				return client
			}
			client = hop
			if !isTrusted(hop, trustedProxies) {
				return client
			}
		}
	}
	return client
}

func isTrusted(addr netip.Addr, trustedProxies []netip.Prefix) bool {
	if !addr.IsValid() {
		return false
	}
	for _, prefix := range trustedProxies {
		if prefix.Contains(addr) {
			return true
		}
	}
	return false
}

// parseIP accepts "ip", "ip:port" and "[ipv6]:port". IPv4-mapped IPv6
// addresses are unmapped and zones dropped so they match configured prefixes.
// It returns the zero Addr if value is not an IP address.
func parseIP(value string) netip.Addr {
	if addrPort, err := netip.ParseAddrPort(value); err == nil {
		return addrPort.Addr().Unmap().WithZone("")
	}
	if addr, err := netip.ParseAddr(strings.Trim(value, "[]")); err == nil {
		return addr.Unmap().WithZone("")
	}
	return netip.Addr{}
}

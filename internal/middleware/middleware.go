package middleware

import (
	"compress/gzip"
	"log"
	"mime"
	"net"
	"net/http"
	"runtime/debug"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	requestLimit = 100
	rateWindow   = time.Minute
)

type middleware func(http.Handler) http.Handler

// Stack applies the application's standard-library HTTP middleware.
func Stack(next http.Handler) http.Handler {
	rateLimiter := newIPRateLimiter(requestLimit, rateWindow)
	middlewares := []middleware{
		requestLogger,
		securityHeaders,
		blockDangerousMethods,
		rateLimiter.Handler,
		gzipResponses,
		recoverPanics,
	}

	for i := len(middlewares) - 1; i >= 0; i-- {
		next = middlewares[i](next)
	}
	return next
}

func requestLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("%s %s from %s in %s", r.Method, r.URL.RequestURI(), r.RemoteAddr, time.Since(started))
	})
}

func recoverPanics(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if recovered := recover(); recovered != nil {
				log.Printf("panic serving %s %s: %v\n%s", r.Method, r.URL.Path, recovered, debug.Stack())
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			}
		}()
		next.ServeHTTP(w, r)
	})
}

func blockDangerousMethods(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodConnect || r.Method == http.MethodTrace {
			log.Printf("blocked %s %s from %s", r.Method, r.URL.Path, r.RemoteAddr)
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "strict-origin-when-cross-origin")
		next.ServeHTTP(w, r)
	})
}

type rateLimitEntry struct {
	requests int
	reset    time.Time
}

type ipRateLimiter struct {
	mu          sync.Mutex
	clients     map[string]rateLimitEntry
	limit       int
	window      time.Duration
	lastCleanup time.Time
}

func newIPRateLimiter(limit int, window time.Duration) *ipRateLimiter {
	return &ipRateLimiter{
		clients:     make(map[string]rateLimitEntry),
		limit:       limit,
		window:      window,
		lastCleanup: time.Now(),
	}
}

// Handler limits by the TCP peer address. It intentionally ignores forwarded
// headers because trusting them without a configured proxy chain is spoofable.
func (l *ipRateLimiter) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		now := time.Now()
		key := remoteIP(r.RemoteAddr)

		l.mu.Lock()
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
		l.clients[key] = entry
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

func remoteIP(remoteAddr string) string {
	host, _, err := net.SplitHostPort(remoteAddr)
	if err == nil {
		return host
	}
	return remoteAddr
}

func gzipResponses(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodHead || r.Header.Get("Range") != "" || !acceptsGzip(r.Header.Get("Accept-Encoding")) {
			next.ServeHTTP(w, r)
			return
		}

		writer := &gzipResponseWriter{ResponseWriter: w}
		defer func() {
			if writer.gzipWriter != nil {
				if err := writer.gzipWriter.Close(); err != nil {
					log.Printf("closing gzip response: %v", err)
				}
			}
		}()
		next.ServeHTTP(writer, r)
	})
}

type gzipResponseWriter struct {
	http.ResponseWriter
	gzipWriter  *gzip.Writer
	wroteHeader bool
}

func (w *gzipResponseWriter) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}

func (w *gzipResponseWriter) WriteHeader(status int) {
	if w.wroteHeader {
		return
	}
	w.wroteHeader = true

	if status != http.StatusNoContent && status != http.StatusNotModified && status != http.StatusPartialContent &&
		w.Header().Get("Content-Encoding") == "" && compressibleContentType(w.Header().Get("Content-Type")) {
		w.Header().Del("Content-Length")
		w.Header().Add("Vary", "Accept-Encoding")
		w.Header().Set("Content-Encoding", "gzip")
		w.gzipWriter = gzip.NewWriter(w.ResponseWriter)
	}

	w.ResponseWriter.WriteHeader(status)
}

func (w *gzipResponseWriter) Write(data []byte) (int, error) {
	if !w.wroteHeader {
		if w.Header().Get("Content-Type") == "" {
			w.Header().Set("Content-Type", http.DetectContentType(data))
		}
		w.WriteHeader(http.StatusOK)
	}
	if w.gzipWriter != nil {
		return w.gzipWriter.Write(data)
	}
	return w.ResponseWriter.Write(data)
}

func acceptsGzip(value string) bool {
	for _, encoding := range strings.Split(value, ",") {
		parts := strings.Split(strings.TrimSpace(encoding), ";")
		if !strings.EqualFold(strings.TrimSpace(parts[0]), "gzip") {
			continue
		}

		quality := 1.0
		for _, parameter := range parts[1:] {
			name, raw, found := strings.Cut(strings.TrimSpace(parameter), "=")
			if found && strings.EqualFold(name, "q") {
				parsed, err := strconv.ParseFloat(raw, 64)
				if err != nil {
					return false
				}
				quality = parsed
			}
		}
		return quality > 0
	}
	return false
}

func compressibleContentType(value string) bool {
	mediaType, _, err := mime.ParseMediaType(value)
	if err != nil {
		return false
	}
	if strings.HasPrefix(mediaType, "text/") {
		return mediaType != "text/event-stream"
	}
	switch mediaType {
	case "application/javascript", "application/json", "application/xml", "image/svg+xml":
		return true
	default:
		return false
	}
}

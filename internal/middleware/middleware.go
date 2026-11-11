package middleware

import (
	"log/slog"
	"maps"
	"net/http"
	"net/netip"
	"runtime/debug"
	"slices"
	"strings"

	"github.com/tonymmm1/go-htmx/internal/site"
)

// cspBaseline only allows same-origin scripts, styles and requests. Templates
// must therefore avoid inline <script>, style="..." and hx-on:* attributes;
// put that code in files under static/ instead. A site that needs another
// origin (analytics, embeds, an image host) adds it in site.CSP.
var cspBaseline = [][2]string{
	{"default-src", "'self'"},
	{"script-src", "'self'"},
	{"style-src", "'self'"},
	{"img-src", "'self' data:"},
	{"connect-src", "'self'"},
	{"font-src", "'self'"},
	{"object-src", "'none'"},
	{"base-uri", "'self'"},
	{"form-action", "'self'"},
	{"frame-ancestors", "'none'"},
}

var contentSecurityPolicy = buildCSP(site.CSP)

// buildCSP appends the extra sources to each baseline directive. Directives
// outside the baseline (frame-src, media-src, ...) start from 'self'.
func buildCSP(extra map[string][]string) string {
	var directives []string
	baseline := make(map[string]bool, len(cspBaseline))
	for _, d := range cspBaseline {
		name, sources := d[0], d[1]
		baseline[name] = true
		if add := extra[name]; len(add) > 0 {
			// 'none' is only valid on its own.
			if sources == "'none'" {
				sources = strings.Join(add, " ")
			} else {
				sources += " " + strings.Join(add, " ")
			}
		}
		directives = append(directives, name+" "+sources)
	}
	for _, name := range slices.Sorted(maps.Keys(extra)) {
		if !baseline[name] && len(extra[name]) > 0 {
			directives = append(directives, name+" 'self' "+strings.Join(extra[name], " "))
		}
	}
	return strings.Join(directives, "; ")
}

// permissionsPolicy disables powerful browser features the site does not use.
const permissionsPolicy = "camera=(), microphone=(), geolocation=(), payment=(), usb=()"

// Options configures Stack.
type Options struct {
	// TrustedProxies are reverse-proxy networks whose X-Forwarded-For header
	// identifies the client for rate limiting and request logs.
	TrustedProxies []netip.Prefix
	// Logger receives request and panic logs. Nil means slog.Default().
	Logger *slog.Logger
	// ServerError writes the response for a recovered panic (the styled 500
	// page). Nil, or a handler that panics itself, means a plain-text 500.
	ServerError http.Handler
}

type middleware func(http.Handler) http.Handler

// Stack applies the application's standard-library HTTP middleware. The first
// entry is the outermost, so the request logger sees the final status of every
// response, including 500s written by recoverPanics.
func Stack(next http.Handler, opts Options) http.Handler {
	logger := opts.Logger
	if logger == nil {
		logger = slog.Default()
	}

	rateLimiter := newIPRateLimiter(site.RateLimit, site.RateWindow, opts.TrustedProxies)
	middlewares := []middleware{
		requestLogger(logger, opts.TrustedProxies),
		recoverPanics(logger, opts.ServerError),
		securityHeaders,
		blockDangerousMethods,
		rateLimiter.Handler,
		// Rejects cross-site POST/PUT/PATCH/DELETE using Sec-Fetch-Site and
		// Origin. Safe methods and non-browser clients pass through.
		http.NewCrossOriginProtection().Handler,
		gzipResponses,
	}

	for i := len(middlewares) - 1; i >= 0; i-- {
		next = middlewares[i](next)
	}
	return next
}

func recoverPanics(logger *slog.Logger, errorPage http.Handler) middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				recovered := recover()
				if recovered == nil {
					return
				}
				if recovered == http.ErrAbortHandler {
					panic(recovered)
				}

				logger.ErrorContext(r.Context(), "panic serving request",
					slog.String("method", r.Method),
					slog.String("path", r.URL.Path),
					slog.Any("panic", recovered),
					slog.String("stack", string(debug.Stack())),
				)

				// Once headers are out, a 500 can no longer be sent. Abort the
				// connection so the client sees a failure instead of a
				// truncated response that looks complete.
				if rw, ok := w.(*responseRecorder); ok && rw.status != 0 {
					panic(http.ErrAbortHandler)
				}
				serveError(w, r, errorPage)
			}()
			next.ServeHTTP(w, r)
		})
	}
}

// serveError falls back to plain text when there is no error page or the page
// itself panics (a broken layout is a likely cause of the original panic).
func serveError(w http.ResponseWriter, r *http.Request, errorPage http.Handler) {
	if errorPage == nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	defer func() {
		if recover() != nil {
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		}
	}()
	errorPage.ServeHTTP(w, r)
}

func blockDangerousMethods(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodConnect || r.Method == http.MethodTrace {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// securityHeaders sets browser hardening headers. HSTS is deliberately left
// out: the app often runs on plain HTTP behind a TLS-terminating proxy, which
// is the better place to set Strict-Transport-Security.
func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("Content-Security-Policy", contentSecurityPolicy)
		h.Set("Cross-Origin-Opener-Policy", "same-origin")
		h.Set("Permissions-Policy", permissionsPolicy)
		h.Set("Referrer-Policy", "strict-origin-when-cross-origin")
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		next.ServeHTTP(w, r)
	})
}

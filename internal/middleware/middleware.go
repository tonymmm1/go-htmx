package middleware

import (
	"log/slog"
	"net/http"
	"net/netip"
	"runtime/debug"
	"time"
)

const (
	requestLimit = 100
	rateWindow   = time.Minute
)

// contentSecurityPolicy only allows same-origin scripts, styles and requests.
// Templates must therefore avoid inline <script>, style="..." and hx-on:*
// attributes; put that code in files under static/ instead.
const contentSecurityPolicy = "default-src 'self'; script-src 'self'; style-src 'self'; " +
	"img-src 'self' data:; connect-src 'self'; font-src 'self'; object-src 'none'; " +
	"base-uri 'self'; form-action 'self'; frame-ancestors 'none'"

// permissionsPolicy disables powerful browser features the site does not use.
const permissionsPolicy = "camera=(), microphone=(), geolocation=(), payment=(), usb=()"

// Options configures Stack.
type Options struct {
	// TrustedProxies are reverse-proxy networks whose X-Forwarded-For header
	// identifies the client for rate limiting and request logs.
	TrustedProxies []netip.Prefix
	// Logger receives request and panic logs. Nil means slog.Default().
	Logger *slog.Logger
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

	rateLimiter := newIPRateLimiter(requestLimit, rateWindow, opts.TrustedProxies)
	middlewares := []middleware{
		requestLogger(logger, opts.TrustedProxies),
		recoverPanics(logger),
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

func recoverPanics(logger *slog.Logger) middleware {
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
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			}()
			next.ServeHTTP(w, r)
		})
	}
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

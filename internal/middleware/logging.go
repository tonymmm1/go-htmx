package middleware

import (
	"log/slog"
	"net/http"
	"net/netip"
	"time"
)

// requestLogger logs one line per request. Health checks are logged at debug
// level so container probes don't flood production logs.
func requestLogger(logger *slog.Logger, trustedProxies []netip.Prefix) middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			started := time.Now()
			recorder := &responseRecorder{ResponseWriter: w}

			// Deferred so aborted (panicking) requests are logged too.
			defer func() {
				aborted := recover()

				status := recorder.status
				if status == 0 {
					status = http.StatusOK
				}

				level := slog.LevelInfo
				switch {
				case aborted != nil || status >= http.StatusInternalServerError:
					level = slog.LevelError
				case r.URL.Path == "/healthz":
					level = slog.LevelDebug
				}

				attrs := []slog.Attr{
					slog.String("method", r.Method),
					slog.String("path", r.URL.Path),
					slog.Int("status", status),
					slog.Int64("bytes", recorder.bytes),
					slog.Float64("duration_ms", float64(time.Since(started).Microseconds())/1000),
					slog.String("ip", clientIP(r, trustedProxies).String()),
				}
				if aborted != nil {
					attrs = append(attrs, slog.Bool("aborted", true))
				}
				logger.LogAttrs(r.Context(), level, "request", attrs...)

				if aborted != nil {
					panic(aborted)
				}
			}()

			next.ServeHTTP(recorder, r)
		})
	}
}

// responseRecorder captures the status code and body size of a response.
// Unwrap lets http.ResponseController reach the underlying writer for
// deadlines and hijacking.
type responseRecorder struct {
	http.ResponseWriter
	status int
	bytes  int64
}

func (w *responseRecorder) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}

func (w *responseRecorder) WriteHeader(status int) {
	// 1xx responses (e.g. 103 Early Hints) may precede the final status.
	if w.status == 0 && status >= http.StatusOK {
		w.status = status
	}
	w.ResponseWriter.WriteHeader(status)
}

func (w *responseRecorder) Write(data []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	n, err := w.ResponseWriter.Write(data)
	w.bytes += int64(n)
	return n, err
}

// Flush implements http.Flusher for handlers that stream (for example SSE).
func (w *responseRecorder) Flush() {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	_ = http.NewResponseController(w.ResponseWriter).Flush()
}

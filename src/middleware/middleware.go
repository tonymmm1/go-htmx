// middleware/stack.go
package middleware

import (
	"log"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/go-chi/httprate"
	"time"
)

func Stack(r chi.Router) {
	r.Use(middleware.RealIP)

	// sane rate limit: 100 req/min per IP
	r.Use(httprate.Limit(
		100,
		1*time.Minute,
		httprate.WithKeyFuncs(httprate.KeyByRealIP),
	))

	// block dangerous methods early
	r.Use(blockDangerousMethods)

	// basic security headers
	r.Use(securityHeaders)

	// gzip compression
	r.Use(middleware.Compress(5))

	// CORS – replace origins with your actual domains or "*" in dev
	r.Use(cors.Handler(cors.Options{
		// ←←← change only this line in your project
		AllowedOrigins: []string{"https://yourdomain.com", "https://www.yourdomain.com"},
		// or use []string{"*"} for local dev only
		AllowedMethods:   []string{"GET", "POST", "OPTIONS"},
		AllowedHeaders:   []string{"Content-Type", "HX-Request", "HX-Target", "HX-Current-URL"},
		AllowCredentials: false,
		MaxAge:           86400,
	}))

	r.Use(middleware.StripSlashes)
	r.Use(middleware.CleanPath)
	r.Use(middleware.GetHead)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
}

func blockDangerousMethods(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodConnect || r.Method == http.MethodTrace {
			log.Printf("BLOCKED %s %s from %s", r.Method, r.URL.Path, r.RemoteAddr)
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

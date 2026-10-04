package server

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"time"

	"github.com/tonymmm1/go-htmx/internal/config"
	"github.com/tonymmm1/go-htmx/internal/middleware"
	"github.com/tonymmm1/go-htmx/internal/pages"
	"github.com/tonymmm1/go-htmx/static"
)

const (
	// readHeaderTimeout limits slow-loris style clients.
	readHeaderTimeout = 5 * time.Second
	// readTimeout covers headers and body; forms here are small.
	readTimeout = 15 * time.Second
	// writeTimeout bounds each response. Long-lived streams (SSE) should
	// extend it per request with http.NewResponseController(w).SetWriteDeadline.
	writeTimeout = 30 * time.Second
	// idleTimeout closes keep-alive connections that sit unused.
	idleTimeout = 120 * time.Second
	// maxHeaderBytes caps request header size (the stdlib default is 1 MB).
	maxHeaderBytes = 64 << 10
	// shutdownTimeout is how long in-flight requests get to finish after
	// SIGINT/SIGTERM. Keep it below Docker's default 10s stop grace period.
	shutdownTimeout = 5 * time.Second
)

type Server struct {
	config *config.Config
}

func NewServer(config *config.Config) *Server {
	return &Server{
		config: config,
	}
}

// Handler builds the application's routes wrapped in the middleware stack.
// It does not listen, so tests can exercise it with httptest.
func (s *Server) Handler() http.Handler {
	// Initialize pages handler
	pagesHandler := &pages.Handler{
		Config: s.config,
	}

	// Standard library HTTP router
	mux := http.NewServeMux()

	// Health check for container orchestrators and `server -healthcheck`.
	mux.HandleFunc("GET /healthz", handleHealthz)

	// Static files
	static.Configure(s.config.IsDev(), "static")
	mux.Handle("GET /static/", static.Handler())

	// HTTP routes
	pages.RegisterPageRoutes(pagesHandler, mux)

	// Apply middleware stack
	return middleware.Stack(mux, middleware.Options{
		TrustedProxies: s.config.TrustedProxies,
		Logger:         slog.Default(),
	})
}

// Run serves HTTP until ctx is canceled, then shuts down gracefully, giving
// in-flight requests up to shutdownTimeout to complete.
func (s *Server) Run(ctx context.Context) error {
	// HTTP/1.1 plus HTTP/2 over cleartext (h2c, prior knowledge) so a
	// TLS-terminating proxy can speak HTTP/2 to the app.
	var protocols http.Protocols
	protocols.SetHTTP1(true)
	protocols.SetUnencryptedHTTP2(true)

	srv := &http.Server{
		Addr:              ":" + s.config.Port,
		Handler:           s.Handler(),
		Protocols:         &protocols,
		ReadHeaderTimeout: readHeaderTimeout,
		ReadTimeout:       readTimeout,
		WriteTimeout:      writeTimeout,
		IdleTimeout:       idleTimeout,
		MaxHeaderBytes:    maxHeaderBytes,
		ErrorLog:          slog.NewLogLogger(slog.Default().Handler(), slog.LevelWarn),
	}

	// Listen before logging so a busy port fails fast with a clear error.
	listener, err := net.Listen("tcp", srv.Addr)
	if err != nil {
		return fmt.Errorf("listening on %s: %w", srv.Addr, err)
	}
	slog.Info("server listening", slog.String("addr", listener.Addr().String()), slog.String("env", s.config.Env))

	serveErr := make(chan error, 1)
	go func() {
		serveErr <- srv.Serve(listener)
	}()

	select {
	case err := <-serveErr:
		return fmt.Errorf("serving HTTP: %w", err)
	case <-ctx.Done():
	}

	slog.Info("shutting down", slog.Duration("timeout", shutdownTimeout))
	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		// Requests still running after the grace period are cut off.
		_ = srv.Close()
		return fmt.Errorf("graceful shutdown: %w", err)
	}
	if err := <-serveErr; err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("serving HTTP: %w", err)
	}

	slog.Info("server stopped")
	return nil
}

func handleHealthz(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = w.Write([]byte("ok"))
}

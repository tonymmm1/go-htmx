package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/tonymmm1/go-htmx/internal/config"
	"github.com/tonymmm1/go-htmx/internal/server"
)

// healthcheckTimeout bounds the -healthcheck probe; keep it below the
// orchestrator's own timeout (Docker HEALTHCHECK --timeout).
const healthcheckTimeout = 2 * time.Second

func main() {
	healthcheck := flag.Bool("healthcheck", false, "probe GET /healthz on the local server and exit 0 if it returns 200")
	flag.Parse()

	cfg, err := config.LoadConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load configuration: %v\n", err)
		os.Exit(1)
	}

	// Distroless images have no shell or wget, so the binary checks itself.
	if *healthcheck {
		os.Exit(runHealthcheck(cfg.Port))
	}

	slog.SetDefault(newLogger(cfg))

	// Cancel on SIGINT/SIGTERM to start a graceful shutdown. Once canceled,
	// stop restores default signal handling so a second Ctrl-C exits at once.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	context.AfterFunc(ctx, stop)

	if err := server.NewServer(cfg).Run(ctx); err != nil {
		slog.Error("server failed", slog.Any("error", err))
		os.Exit(1)
	}
}

// newLogger logs JSON in production (for log collectors) and human-readable
// text in development.
func newLogger(cfg *config.Config) *slog.Logger {
	if cfg.IsDev() {
		return slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
	}
	return slog.New(slog.NewJSONHandler(os.Stdout, nil))
}

// runHealthcheck returns the process exit code: 0 if the local server answers
// GET /healthz with 200 OK, 1 otherwise.
func runHealthcheck(port string) int {
	client := &http.Client{
		Timeout:   healthcheckTimeout,
		Transport: &http.Transport{Proxy: nil, DisableKeepAlives: true},
	}

	resp, err := client.Get("http://127.0.0.1:" + port + "/healthz")
	if err != nil {
		fmt.Fprintf(os.Stderr, "healthcheck: %v\n", err)
		return 1
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		fmt.Fprintf(os.Stderr, "healthcheck: unexpected status %s\n", resp.Status)
		return 1
	}
	return 0
}

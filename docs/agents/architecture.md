# Architecture

How a request flows through the app, and where each concern lives. Read this before changing the server,
middleware, rendering or static asset code.

## Request lifecycle

```
cmd/server/main.go          config.LoadConfig → slog setup → server.Run(ctx) (SIGINT/SIGTERM cancel ctx)
internal/server/server.go   http.ServeMux:
                              GET /healthz        handleHealthz
                              GET /static/        static.Handler()
                              pages.RegisterPageRoutes(...)
                            wrapped by middleware.Stack(mux, middleware.Options{...})
```

`middleware.Stack` order, outermost first (`internal/middleware/middleware.go`):

1. `requestLogger`: one `slog` line per request (method, path, status, bytes, `duration_ms`, client IP).
2. `recoverPanics`: panic → logged + 500. If headers were already sent, it aborts the connection.
3. `securityHeaders`: CSP, `X-Content-Type-Options`, `X-Frame-Options`, `Referrer-Policy`, COOP,
   `Permissions-Policy`. No HSTS; set that on the TLS-terminating proxy.
4. `blockDangerousMethods`: 405 for CONNECT and TRACE.
5. Rate limiter: 100 requests/minute per client IP (IPv6 grouped by /64). `/static/` and `/healthz` are
   exempt. The client IP comes from `X-Forwarded-For` only when the TCP peer is in `TRUSTED_PROXIES`.
6. `http.NewCrossOriginProtection()`: rejects cross-site POST/PUT/PATCH/DELETE (CSRF).
7. `gzipResponses`: compresses text-like responses unless `Content-Encoding` is already set; pooled
   writers; supports `Flush`.

Server limits (`internal/server/server.go`): ReadHeaderTimeout 5s, ReadTimeout 15s, WriteTimeout 30s,
IdleTimeout 120s, 5s graceful shutdown. HTTP/1.1 and cleartext HTTP/2 (h2c).

## Rendering (`internal/pages/render.go`)

- `Render(w, r, status, component)` renders into a pooled buffer, then writes `Content-Type`,
  `Content-Length`, status and body. A render error becomes a clean 500 with nothing partial sent.
- `RenderPage(w, r, status, page)` is for components that use `layouts.Layout`. It adds
  `Vary: HX-Request, HX-Boosted, HX-History-Restore-Request`. A plain htmx request (not boosted, not a
  history restore) gets the page content only; `Layout` checks `layouts.WithContentOnly(ctx)` and skips
  the document shell. Normal and boosted navigations get the full document.
- `IsHTMX(r)` and `IsBoosted(r)` read the `HX-Request` and `HX-Boosted` headers.

The layout (`templates/layouts/layout.templ`) sets `hx-boost="true"` on `<body>`, so links and forms
navigate with htmx and swap the body. htmx is configured through the `htmxConfig` constant (rendered as
`<meta name="htmx-config">`): indicator styles off (they live in `styles/input.css`), `allowEval` and
`allowScriptTags` off (CSP), and history-restore requests get full pages.

## Client side (`static/js/app.js`)

The only app JavaScript. It's loaded without `defer` so the saved theme applies before first paint;
everything else is an event listener on `document` (so it survives hx-boost body swaps):

- theme toggle: `data-theme` on `<html>`, saved in `localStorage`;
- error toasts for `htmx:responseError`, `htmx:sendError` and `htmx:timeout` (text from the status code,
  inserted with `textContent`);
- boosted navigations that receive an HTML 4xx/5xx page (e.g. the 404 page) are swapped in like a normal
  page load.

## Static assets (`static/static.go`)

- **Production:** directories in the `//go:embed all:css img js` line are compiled into the binary. At
  startup each file is hashed (SHA-256, 12 hex chars) and text-like files are gzipped once.
  `static.Path("css/styles.css")` returns `/static/css/styles.css?v=<hash>`. Requests with the current
  hash get `Cache-Control: public, max-age=31536000, immutable`; others get `max-age=300` plus an ETag.
- **Development** (`APP_ENV=development`): files are read from `static/` on disk on every request with
  `Cache-Control: no-cache`; `static.Path` returns unversioned URLs and panics if the file is missing.
- Dotfiles, `.go` files and directories are never served.
- `static/css/` is Tailwind output (gitignored apart from `.gitkeep`). `make css` builds it, and it must
  exist before `go build` for production, because it is embedded at compile time.

## Configuration (`internal/config/config.go`)

| Variable | Default | Used by |
|---|---|---|
| `PORT` | `8080` | server address, `-healthcheck` |
| `APP_ENV` | `production` | static serving mode, log format (JSON in production, text in development) |
| `TRUSTED_PROXIES` | empty | rate limiter and request log client IP |

Real environment variables override `.env`. Handlers reach config through `pages.Handler.Config`.

## Build and dev tooling

- `make dev` runs `go tool templ generate --watch --cmd="go run ./cmd/server" --proxy=…` alongside
  Tailwind's watcher. `.go` changes restart the server; text-only `.templ` changes reload the browser
  without a restart. The browser URL is the templ proxy (`PROXY_PORT`, default 7331).
- `make build` → `CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o bin/server ./cmd/server`.
- The Dockerfile builds CSS in a Node stage and the binary in a Go stage, then copies only the binary into
  `gcr.io/distroless/static-debian12:nonroot`. The `HEALTHCHECK` runs `/app/server -healthcheck`.

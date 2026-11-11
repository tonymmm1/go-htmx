# Architecture

How a request flows through the app, and where each concern lives. Read this before changing the server,
middleware, rendering or static asset code.

## Request lifecycle

```
cmd/server/main.go          config.LoadConfig → slog setup → server.Run(ctx) (SIGINT/SIGTERM cancel ctx)
internal/server/server.go   http.ServeMux:
                              GET /healthz        handleHealthz
                              GET /static/        static.Handler()
                              pages.RegisterPageRoutes(...)   pages, /robots.txt, /sitemap.xml,
                                                              /favicon.ico (redirect), 404 catch-all
                            wrapped by middleware.Stack(mux, middleware.Options{...})
```

`middleware.Stack` order, outermost first (`internal/middleware/middleware.go`):

1. `requestLogger`: one `slog` line per request (method, path, status, bytes, `duration_ms`, client IP).
2. `recoverPanics`: panic → logged + 500, rendered by `Options.ServerError` (the server passes
   `pages.RenderError`; plain text if that is nil or panics too). If headers were already sent, it aborts
   the connection.
3. `securityHeaders`: CSP, `X-Content-Type-Options`, `X-Frame-Options`, `Referrer-Policy`, COOP,
   `Permissions-Policy`. No HSTS; set that on the TLS-terminating proxy. The CSP is the same-origin
   baseline in `internal/middleware/middleware.go` plus the sources in `site.CSP`.
4. `blockDangerousMethods`: 405 for CONNECT and TRACE.
5. Rate limiter: `site.RateLimit` requests per `site.RateWindow` (100/minute) per client IP (IPv6 grouped
   by /64). `/static/` and `/healthz` are exempt. The client IP comes from `X-Forwarded-For` only when the TCP peer is in `TRUSTED_PROXIES`.
6. `http.NewCrossOriginProtection()`: rejects cross-site POST/PUT/PATCH/DELETE (CSRF).
7. `gzipResponses`: compresses text-like responses unless `Content-Encoding` or an `ETag` is already
   set (precompressed static assets, and any handler managing its own representations, pass through);
   pooled writers; supports `Flush`. gzip only: Brotli is used just for static assets, compressed once
   at startup. `Accept-Encoding` negotiation for both lives in `internal/contentcoding`.

Server limits (`internal/server/server.go`): ReadHeaderTimeout 5s, ReadTimeout 15s, WriteTimeout 30s,
IdleTimeout 120s, 5s graceful shutdown. HTTP/1.1 and cleartext HTTP/2 (h2c).

## Rendering (`internal/pages/render.go`)

- `Render(w, r, status, component)` renders into a pooled buffer, then writes `Content-Type`,
  `Content-Length`, status and body. A render error becomes a clean 500 with nothing partial sent.
- `RenderPage(w, r, status, page)` is for components that use `layouts.Layout`. It adds
  `Vary: HX-Request, HX-Request-Type`. An htmx request with `HX-Request-Type: partial` (a targeted swap)
  gets the page content only; `Layout` checks `layouts.WithContentOnly(ctx)` and skips the document shell.
  Everything else gets the full document: normal navigations, and the requests htmx 4 marks `full`
  (boosted links and forms, history restores, `hx-select`).
- `RenderError(w, r, status)` renders the styled error page (`templates/pages/error.templ`) with
  `Cache-Control: no-store`.
- `IsHTMX(r)`, `IsBoosted(r)` and `IsPartial(r)` read the `HX-Request`, `HX-Boosted` and
  `HX-Request-Type` headers.

The layout (`templates/layouts/layout.templ`) sets `hx-boost:inherited="true"` and
`hx-indicator:inherited="#page-progress"` on `<body>` (htmx 4 inheritance is explicit), so links and forms
navigate with htmx and swap the body. htmx is configured through the `htmxConfig` constant (rendered as
`<meta name="htmx-config">`): only `includeIndicatorCSS: false`, since the indicator styles live in
`styles/input.css`. The rest are htmx 4 defaults: same-origin requests, a 60s request timeout, and history
restores that re-fetch the page from the server (`HX-History-Restore-Request: true`, swapped into `<body>`).
htmx 4 has no `allowEval`; features that need `new Function()` (`hx-on:*`, `js:` values, `hx-trigger`
filters) fail under the CSP, which has no `'unsafe-eval'`.

## Client side (`static/js/app.js`)

The only app JavaScript. It's loaded without `defer` so the saved theme applies before first paint;
everything else is an event listener on `document` (so it survives hx-boost body swaps):

- theme toggle: `data-theme` on `<html>`, saved in `localStorage`;
- error handling for htmx 4, which swaps 4xx/5xx responses by default. On `htmx:response:error`, a
  navigation (`HX-Request-Type: full`) that received an HTML error page, such as the 404 page, is swapped
  in like a normal page load. Any other error response sets `ctx.swap = "none"` and stops the URL push,
  then shows a toast, unless the source element has a matching `hx-status:<code>` attribute.
  `htmx:error` (network failure, timeout, swap exception) shows a toast too. Toast text comes from the
  status code and is inserted with `textContent`.

## Static assets (`static/static.go`)

- **Production:** directories in the `//go:embed all:css img js` line are compiled into the binary. At
  startup each file is hashed (SHA-256, 12 hex chars) and text-like files are compressed once with
  Brotli (quality 11, via `github.com/andybalholm/brotli`) and gzip (level 9); a variant is kept only
  if it is smaller than the original. `negotiateEncoding` picks br > gzip > identity from
  `Accept-Encoding` (q-values, `q=0`, `*`; ties prefer br). Responses carry `Vary: Accept-Encoding`
  and per-variant ETags (`"<hash>-br"`, `"<hash>-gz"`, `"<hash>"`); Range requests get the identity
  bytes. Files and codings are compressed in parallel, once per process; Brotli adds roughly 175ms to startup
  for the default assets (logged on startup).
  `static.Path("css/styles.css")` returns `/static/css/styles.css?v=<hash>`. Requests with the current
  hash get `Cache-Control: public, max-age=31536000, immutable`; others get `max-age=300` plus an ETag.
- **Development** (`APP_ENV=development`): files are read from `static/` on disk on every request with
  `Cache-Control: no-cache` and are not precompressed (only `gzipResponses` applies); `static.Path`
  returns unversioned URLs and panics if the file is missing.
- Dotfiles, `.go` files and directories are never served.
- `static/css/` is Tailwind output (gitignored apart from `.gitkeep`). `make css` builds it, and it must
  exist before `go build` for production, because it is embedded at compile time.

## Site values (`internal/site/site.go`)

Compile-time values that differ per site: name, canonical `URL`, language, footer, theme colours, default
link-preview image, rate limit and extra CSP sources. The layout, `layouts.Meta` (`URL()`, `ImageURL()`),
the middleware and the robots/sitemap handlers read them. Per-site edits belong here so that merging
template updates into a site rarely conflicts.

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
- CSS tooling: `scripts/install-tools.sh` (`make tools`) downloads the Tailwind standalone CLI (typography
  plugin built in) and daisyUI's `daisyui.mjs` into `.tools/`, verifying pinned SHA-256 checksums.
  `styles/input.css` loads the plugin with `@plugin "../.tools/daisyui.mjs"`. There is no Node.js or npm.
- The Dockerfile installs the CSS tools and builds the CSS and binary in a Go stage, then copies only the binary into
  `gcr.io/distroless/static-debian12:nonroot`. The `HEALTHCHECK` runs `/app/server -healthcheck`.

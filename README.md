# Go-HTMX Starter Template

[![CI](https://github.com/tonymmm1/go-htmx/actions/workflows/ci.yml/badge.svg)](https://github.com/tonymmm1/go-htmx/actions/workflows/ci.yml)

A small, production-minded starter for server-rendered websites with Go, [htmx](https://htmx.org/),
[templ](https://templ.guide/), [Tailwind CSS](https://tailwindcss.com/) 4 and [daisyUI](https://daisyui.com/) 5.
It uses the standard library for routing and middleware, builds into one static binary with all assets
embedded, and needs Node.js only at build time to compile the CSS.

[Quick start guide](QUICKSTART.md) · [UI components (daisyUI, shadcn-style)](docs/ui.md) · [Comparison with other approaches](COMPARISON.md) · [Agent instructions](AGENTS.md)

## Why it's lightweight

- **The whole home page is about 34 KB gzipped**, including the stylesheet (~15 KB), htmx (~17 KB) and
  the app script (~1.5 KB). No client framework and no CDN: htmx 2.0.10 is vendored in `static/js/`.
- **An ~8 MB static binary** (`CGO_ENABLED=0`, stripped) contains the server, the compiled templates and
  every static asset.
- **A ~19 MB container image**: `gcr.io/distroless/static-debian12:nonroot` plus the binary, nothing else.
- **Assets are cached forever and served precompressed.** URLs carry a content hash
  (`/static/css/styles.css?v=<hash>`), are served `immutable` for a year, and are gzipped once at startup.
- **Two direct Go dependencies**: templ and godotenv. Everything else is the standard library.

## What's included

- `net/http` routing (Go 1.22+ patterns), graceful shutdown, server timeouts, HTTP/1.1 + h2c, `GET /healthz`
- Middleware: structured request logging (`log/slog`), panic recovery, security headers with a strict
  Content Security Policy, CSRF protection, per-IP rate limiting, gzip
- Page rendering that serves full documents to normal and boosted navigation and just the content to
  plain htmx requests
- `hx-boost` navigation with a progress bar, a light/dark theme toggle, error toasts for failed htmx
  requests and a 404 page
- Working htmx examples at `/examples` (fragment GET, debounced search, form POST)
- Page and component generators, hot reload via templ's watcher, Docker and GitHub Actions CI

## Quick start

Prerequisites: Go 1.27+, Node.js 22+ with npm, and make.

```bash
git clone https://github.com/tonymmm1/go-htmx.git my-project
cd my-project
make setup MODULE=github.com/you/my-project   # omit MODULE to keep the module path
make dev
```

Open **http://localhost:7331**. That is templ's live-reload proxy in front of the app, which listens on
`PORT` (8080). `/examples` shows the htmx demos.

`setup.sh` checks prerequisites, sets the module path and rewrites imports in `.go` and `.templ` files,
copies `.env.example` to `.env`, installs Go and npm dependencies, generates templ code, builds the CSS and
compiles. It is safe to re-run. To start a project with the scaffolder or `gonew`, see
[QUICKSTART.md](QUICKSTART.md).

### The dev loop

`make dev` runs two watchers in parallel:

- `go tool templ generate --watch`: regenerates Go code from `.templ` files, restarts the server
  (`go run ./cmd/server`) when Go code changes and reloads the browser through the proxy.
- Tailwind's watcher: rebuilds `static/css/styles.css` from `styles/input.css` and the classes it finds in
  `templates/`, `internal/` and `static/js/app.js`.

In development (`APP_ENV=development`) static files are read from disk with `Cache-Control: no-cache`, so
CSS changes show up on the next page load without a restart. templ is pinned as a Go tool in `go.mod`, so
there is nothing to install globally: run it as `go tool templ`.

## Project structure

```
cmd/server/main.go        Entry point: config, logger, signals, -healthcheck flag
internal/
  config/                 PORT, APP_ENV, TRUSTED_PROXIES (+ optional .env)
  server/                 Routes, middleware stack, timeouts, graceful shutdown, /healthz
  middleware/             Logging, recovery, security headers, rate limit, gzip
  pages/
    pages.go              Routes and handlers
    render.go             RenderPage, Render, IsHTMX, IsBoosted
templates/
  layouts/layout.templ    Document shell, nav, theme toggle, toasts
  layouts/meta.go         layouts.Meta and SiteName
  pages/*.templ           Pages (index, about, examples, not-found)
  components/*.templ      Reusable components and htmx fragments
static/
  static.go               Embeds and serves assets (go:embed, hashing, gzip, ETags)
  css/                    Tailwind output (generated, gitignored)
  js/app.js               Theme toggle and htmx error toasts
  js/htmx.min.js          Vendored htmx 2.0.10
  img/favicon.svg
styles/input.css          Tailwind entry point (daisyUI themes, custom CSS)
scripts/                  new-page.sh, new-component.sh, test-generators.sh, check-docs.sh
setup.sh                  Project setup and module rename
create-go-htmx.sh         Scaffolder: clone, setup, git init
docs/ui.md                UI library guide (daisyUI, shadcn-style alternatives)
AGENTS.md                 Instructions for coding agents (CLAUDE.md imports it)
```

## Adding pages and fragments

### Pages

```bash
make new-page contact-us
```

This creates `templates/pages/contact-us.templ` and adds `GET /contact-us` and `HandleContactUs` to
`internal/pages/pages.go`. Add a link to the nav in `templates/layouts/layout.templ` yourself if you want
one.

By hand, a page is a templ component wrapped in `layouts.Layout`:

```templ
templ About() {
	@layouts.Layout(layouts.Meta{
		Title:       "About",
		Description: "What is included in the Go-HTMX starter template.",
		Path:        "/about",
	}) {
		<div class="container mx-auto px-4 py-8">...</div>
	}
}
```

`Title` renders as `About · Go-HTMX` (leave it empty for just the site name, set in `layouts.SiteName`),
`Description` becomes `<meta name="description">`, and `Path` marks the matching nav link as current.

The handler and route:

```go
mux.HandleFunc("GET /about", h.HandleAbout)

func (h *Handler) HandleAbout(w http.ResponseWriter, r *http.Request) {
	RenderPage(w, r, http.StatusOK, pagetemplates.About())
}
```

`RenderPage(w, r, status, page)` decides how much of the page to send:

| Request | Response |
|---------|----------|
| Normal navigation, boosted link or form (`hx-boost`), history restore | Full document |
| Other htmx request, e.g. `hx-get="/about" hx-target="#panel"` | Page content only, no `<html>`, nav or footer |

It sets `Vary: HX-Request, HX-Boosted, HX-History-Restore-Request` so caches keep the two apart. Unknown
paths fall through to the `GET /` catch-all, which renders the 404 page with `RenderPage`.

### Fragments

Endpoints that htmx swaps into an existing page return a component with `Render`:

```go
// HandleExampleTime returns only the fragment HTMX swaps into the page.
func (h *Handler) HandleExampleTime(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	now := time.Now().Format("Mon, 02 Jan 2006 15:04:05 MST")
	Render(w, r, http.StatusOK, components.ServerTime(now))
}
```

Both functions render into a pooled buffer first, so a template error becomes a clean 500 instead of a
half-written page. `pages.IsHTMX(r)` and `pages.IsBoosted(r)` are available if a handler needs to branch on
the request type itself.

### Components

```bash
make new-component feature-card   # templates/components/feature-card.templ
```

Use it from a page with `@components.FeatureCard("Title")` after importing
`<your module>/templates/components`.

### Frontend rules (CSP)

The Content Security Policy only allows same-origin scripts and styles, so templates must not use inline
`<script>`, `style="..."` or `hx-on:*` attributes. htmx runs with `allowEval: false` and
`allowScriptTags: false` (see `htmxConfig` in `templates/layouts/layout.templ`), which also disables
`hx-vals="js:..."` and event filters like `hx-trigger="click[ctrlKey]"`. Put behaviour in
`static/js/app.js` (or a new file under `static/js/`) as event listeners on `document`, the way the theme
toggle and error toasts are written.

`app.js` shows a toast when an htmx request fails (4xx/5xx, network error, timeout). Boosted navigation
that returns an HTML error page, such as the 404, is swapped in like a normal page load instead.

### Static assets

Reference assets with `static.Path`, which adds the content hash in production:

```templ
<link rel="stylesheet" href={ static.Path("css/styles.css") }/>
```

Only directories listed in the `//go:embed all:css img js` line in `static/static.go` are embedded. Add new
directories (fonts, etc.) there, or they will 404 in production. In development `static.Path` panics if the
file doesn't exist and logs a warning if it isn't embedded. Dotfiles and `.go` files are never served.

### Styling

Themes are configured in `styles/input.css` (`light --default, dark --prefersdark`). With no saved
choice the site follows the OS preference; the toggle switches between light and dark and stores the
choice in `localStorage`. Custom CSS goes at the end of `styles/input.css`.

[docs/ui.md](docs/ui.md) covers daisyUI conventions and how to switch to a shadcn/ui-style kit
([Basecoat](https://github.com/hunvreus/basecoat) or [shadcn-templ](https://templui.io/)) within the CSP.

## Configuration

Settings come from environment variables. `.env` (created from `.env.example` by setup) is loaded if
present; real environment variables take precedence.

| Variable | Default | Description |
|----------|---------|-------------|
| `PORT` | `8080` | Port the HTTP server listens on |
| `APP_ENV` | `production` | `development` serves static files from disk and logs text at debug level; `production` serves embedded assets and logs JSON. Any other value is an error. |
| `TRUSTED_PROXIES` | (empty) | Comma-separated CIDRs or IPs of reverse proxies whose `X-Forwarded-For` is trusted for the client IP (rate limiting and logs). Empty means the TCP peer address is always used. |

`.env.example` sets `APP_ENV=development`, so a local `.env` runs in development mode. The Makefile reads
`PORT` from `.env` too, and `make dev` accepts `PROXY_PORT` (default 7331) and `PROXY_BIND` (default
`127.0.0.1`).

## Security and performance notes

The middleware stack (`internal/middleware`), outermost first:

1. **Request logging**: one `slog` line per request with method, path, status, bytes, `duration_ms` and
   client IP. `/healthz` is logged at debug level.
2. **Panic recovery**: logs the stack and returns 500, or aborts the connection if headers were already sent.
3. **Security headers**: CSP (above), `X-Frame-Options: DENY`, `X-Content-Type-Options: nosniff`,
   `Referrer-Policy`, `Permissions-Policy`, `Cross-Origin-Opener-Policy`. HSTS is not set; configure it on
   the TLS-terminating proxy.
4. **`CONNECT` and `TRACE` are rejected** with 405.
5. **Rate limiting**: 100 requests per minute per client IP (IPv6 grouped by /64), with `X-RateLimit-*`
   and `Retry-After` headers. `/static/` and `/healthz` are exempt. Change `requestLimit` and `rateWindow`
   in `internal/middleware/middleware.go`. Behind a proxy, set `TRUSTED_PROXIES` or every visitor shares
   the proxy's limit.
6. **CSRF protection**: `http.CrossOriginProtection` rejects cross-origin `POST`/`PUT`/`PATCH`/`DELETE`
   from browsers using `Sec-Fetch-Site` and `Origin`. No tokens needed.
7. **gzip** for text, JSON, JavaScript, XML and SVG responses. Precompressed assets and
   `text/event-stream` pass through untouched.

The server sets read-header (5s), read (15s), write (30s) and idle (120s) timeouts, caps headers at 64 KB
and gives in-flight requests 5s to finish on `SIGINT`/`SIGTERM`. Streaming handlers (SSE) must extend the
write deadline per request, for example:

```go
_ = http.NewResponseController(w).SetWriteDeadline(time.Time{}) // no deadline for this response
```

The server speaks HTTP/1.1 and HTTP/2 over cleartext (h2c), so a TLS-terminating proxy can use HTTP/2 to
reach it.

## Deployment

### Binary

```bash
make build                       # ./bin/server, assets embedded
APP_ENV=production ./bin/server  # production is also the default when APP_ENV is unset
```

Note that `./bin/server` loads `.env` from the working directory if it exists, so a local `.env` with
`APP_ENV=development` applies here too.

### Docker

The multi-stage `Dockerfile` builds the CSS with Node, compiles the binary with Go and copies only the
binary into `gcr.io/distroless/static-debian12:nonroot`. It runs as a non-root user and has a
`HEALTHCHECK` that runs `/app/server -healthcheck` (distroless has no shell or curl).

```bash
make docker-up      # build the image and run it on http://localhost:$PORT
make docker-down

make compose-up     # docker compose: read-only filesystem, all capabilities dropped, healthcheck
make compose-dev    # hot reload in a container; open http://localhost:7331
make compose-down
```

`server -healthcheck` requests `GET /healthz` on `127.0.0.1:$PORT` and exits 0 on 200, so orchestrators
can use the binary itself as the probe.

## Make targets

Run `make` (or `make help`) for the list.

| Target | Description |
|--------|-------------|
| `setup` | Run `setup.sh` (keeps the current module path; use `bash setup.sh <module>` to rename) |
| `dev` | templ watcher + Tailwind watcher, http://localhost:7331 |
| `build` / `run` | Build `./bin/server` (CSS, templ, static binary) / build and run it |
| `generate` / `css` | `go tool templ generate` / minified Tailwind build |
| `new-page <name>` / `new-component <name>` | Generators |
| `test` / `test-race` | `go test ./...` / with the race detector |
| `test-generators` | Run both generators in a temp copy with a different module path, then vet and test |
| `check-docs` | Fail if `AGENTS.md` or `docs/ui.md` mention files or make targets that don't exist |
| `fmt` / `fmt-check` | Format Go and templ files / fail if anything is unformatted |
| `vet` / `lint` | `go vet` / `go vet` + staticcheck |
| `check` | `fmt-check`, `lint`, `check-docs`, `test-race`, `test-generators`, `build` |
| `audit` | `npm audit --audit-level=critical` + govulncheck |
| `deps` / `tools` | Install Go and npm dependencies / download Go modules (incl. templ) |
| `docker-build` / `docker-up` / `docker-down` | Standalone image and container |
| `compose-up` / `compose-dev` / `compose-down` | Docker Compose app, dev profile, stop |
| `clean` / `clean-all` | Remove build output and generated files / also `node_modules` and the Go module cache |

CI (`.github/workflows/ci.yml`) runs `go mod verify`, `fmt-check`, `lint`, `check-docs`, `test-race`,
`test-generators`, `build` and `audit`, then builds the Docker image and smoke-tests it: `/healthz`, the
home page, the embedded stylesheet, and `server -healthcheck`.

## Troubleshooting

- **`templ: command not found`**: use `go tool templ ...` or `make generate`; templ isn't installed globally.
- **Panic `static.Path("css/styles.css") ... has the asset been built?`**: run `make css` (or `make dev`).
- **Port already in use**: change `PORT` in `.env`, or `PROXY_PORT=7332 make dev` for the proxy.

## License

[MIT](LICENSE)

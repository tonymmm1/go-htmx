# AGENTS.md

Instructions for coding agents working in this repository. Humans: see README.md.

Server-rendered Go web app: `net/http` + [templ](https://templ.guide/) + htmx 2 + Tailwind CSS 4 + daisyUI 5.
One static binary with all assets embedded. Keep it lightweight: prefer the standard library and avoid new
dependencies (Go or npm) unless asked.

## Commands

| Task | Command |
|---|---|
| Full verification (run before finishing) | `make check` |
| Tests only | `make test` (builds CSS + generates templ first; plain `go test` can fail without them) |
| Regenerate Go from `.templ` | `make generate` |
| Format Go + templ | `make fmt` |
| Build CSS | `make css` |
| Dev server with live reload | `make dev` → http://localhost:7331 (app itself on `PORT`, default 8080) |
| New page / component | `make new-page <name>` / `make new-component <name>` |

templ is a Go tool pinned in `go.mod`: run it as `go tool templ`, never `templ` or `go install`.

## Layout

- `cmd/server/main.go`: entry point, logging, `-healthcheck` flag
- `internal/config`: env config (`PORT`, `APP_ENV`, `TRUSTED_PROXIES`)
- `internal/server`: mux, `/healthz`, timeouts, graceful shutdown
- `internal/middleware`: logging, recovery, security headers/CSP, CSRF, rate limit, gzip
- `internal/pages`: route registration and handlers, `Render`/`RenderPage` helpers
- `templates/layouts`, `templates/pages`, `templates/components`: `.templ` files
- `static/`: Go package that embeds and serves assets; `static/js/app.js` is the only app JavaScript
- `styles/input.css`: Tailwind + daisyUI configuration (no `tailwind.config.js`)

## Rules

1. **Never edit `*_templ.go`.** They are generated and gitignored. Edit the `.templ` file, then `make generate`.
2. **Strict CSP** (`script-src 'self'; style-src 'self'`). Do not add inline `<script>` blocks, `style="…"`
   attributes, `<style>` tags, `hx-on:*`, `hx-vals="js:…"`, hx-trigger filters (`[…]`), or `javascript:` URLs.
   Put behaviour in `static/js/app.js` (event listeners on `document`, so they survive hx-boost swaps) and
   styles in `styles/input.css`. CDN assets are not allowed either; vendor files into `static/`.
3. **Assets must be embedded.** Reference them with `static.Path("dir/file")` in templates, never a literal
   `/static/...` URL. A new directory under `static/` must be added to the `//go:embed` line in
   `static/static.go`, or it will 404 in production. `TestLayoutHead` fails if a layout asset lacks its hash.
4. **Rendering:** full pages use `RenderPage(w, r, status, pagetemplates.X())`; htmx fragments use
   `Render(w, r, status, components.Y(...))`. Don't write HTML to `w` directly.
5. **Pages** wrap their content in `@layouts.Layout(layouts.Meta{Title, Description, Path})`. Add a nav link in
   `templates/layouts/layout.templ` if the page should appear in the header.
6. **Routes** go in `RegisterPageRoutes` in `internal/pages/pages.go`, above the `// scaffold:routes` marker.
   Keep the marker and keep `GET /` (the 404 catch-all) last.
7. **Tailwind class names must appear literally** in `.templ`/`.go`/`app.js` files so the scanner finds them.
   Don't build class names by string concatenation (`"btn-" + size`); map to full class strings instead.
8. **Styling:** use daisyUI 5 components and semantic colours (`btn-primary`, `bg-base-200`,
   `text-base-content`). Don't add `dark:` variants for theme colours; themes switch via `data-theme`.
   See [docs/ui.md](docs/ui.md) before adding or replacing a UI library.
9. **State-changing requests** use POST/PUT/PATCH/DELETE. Cross-origin unsafe requests are rejected by
   `http.CrossOriginProtection`; don't disable it.
10. **Module path** appears in `.go` and `.templ` imports. Don't hand-edit it; `make setup MODULE=…` rewrites it.

## Patterns

Page (a new file in `templates/pages/`, handler in `internal/pages/pages.go`):

```templ
templ Contact() {
	@layouts.Layout(layouts.Meta{Title: "Contact", Description: "How to reach us.", Path: "/contact"}) {
		<section class="container mx-auto px-4 py-8">…</section>
	}
}
```

```go
mux.HandleFunc("GET /contact", h.HandleContact) // above // scaffold:routes

func (h *Handler) HandleContact(w http.ResponseWriter, r *http.Request) {
	RenderPage(w, r, http.StatusOK, pagetemplates.Contact())
}
```

Fragment: return only the swapped element. htmx does not swap 4xx/5xx responses; app.js shows an error toast
instead. To show inline validation errors, return the form fragment with its error markup and status 200 (or
change htmx's `responseHandling` in the layout's `htmxConfig`). See `HandleExampleCounter` and
`templates/components/examples.templ`.

## Testing

Handler tests live next to handlers and go through the real mux (`internal/pages/*_test.go`). When adding a
page or endpoint, add a test for status and key markup. Middleware and static packages have their own tests;
keep them passing with `make test-race`.

## Gotchas

- `APP_ENV` defaults to `production`. `.env` (from `.env.example`) sets `development`, which serves assets
  from disk; `static.Path` panics in development if the file doesn't exist (e.g. CSS not built yet).
- Streaming/SSE handlers must extend the 30s write timeout with
  `http.NewResponseController(w).SetWriteDeadline(...)`.
- Don't add Air (or other dev tools) to `go.mod` as a `tool`; Air pulls in a huge dependency tree. `make dev`
  uses templ's watcher.

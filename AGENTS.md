# AGENTS.md

Instructions for coding agents working in this repository. Humans: see README.md.

Read more only when the task needs it:

| Doc | Read it when |
|---|---|
| [docs/agents/recipes.md](docs/agents/recipes.md) | Adding a page, fragment, form, component, JS behaviour, asset, config var, middleware or SSE endpoint |
| [docs/agents/architecture.md](docs/agents/architecture.md) | Changing the server, middleware, rendering, static serving or build tooling |
| [docs/ui.md](docs/ui.md) | Styling, themes, or adding/replacing a UI library (daisyUI, shadcn-style kits) |

Server-rendered Go web app: `net/http` + [templ](https://templ.guide/) + htmx 4 + Tailwind CSS 4 + daisyUI 5.
One static binary with all assets embedded. Keep it lightweight: prefer the standard library and avoid new
dependencies unless asked. There is no Node.js or npm: Tailwind runs as a standalone binary.

## Commands

| Task | Command |
|---|---|
| Full verification (run before finishing) | `make check` |
| Tests only | `make test` (builds CSS + generates templ first; plain `go test` can fail without them) |
| Regenerate Go from `.templ` | `make generate` |
| Format Go + templ | `make fmt` |
| Build CSS | `make css` (downloads the Tailwind standalone CLI + daisyUI into `.tools/` first) |
| Dev server with live reload | `make dev` → http://localhost:7331 (app itself on `PORT`, default 8080) |
| New page / component | `make new-page <name>` / `make new-component <name>` |
| Remove the demo pages and template docs (once, in a new site) | `make strip-examples` |

templ is a Go tool pinned in `go.mod`: run it as `go tool templ`, never `templ` or `go install`.

## Layout

- `cmd/server/main.go`: entry point, logging, `-healthcheck` flag
- `internal/config`: env config (`PORT`, `APP_ENV`, `TRUSTED_PROXIES`)
- `internal/site/site.go`: per-site values (name, canonical URL, language, footer, rate limit, extra CSP sources)
- `internal/server`: mux, `/healthz`, timeouts, graceful shutdown
- `internal/middleware`: logging, recovery, security headers/CSP, CSRF, rate limit, gzip
- `internal/contentcoding`: `Accept-Encoding` negotiation shared by the gzip middleware and `static`
- `internal/pages`: route registration and handlers, `Render`/`RenderPage`/`RenderError` helpers,
  `/robots.txt`, `/sitemap.xml`
- `templates/layouts`, `templates/pages`, `templates/components`: `.templ` files
- `static/`: Go package that embeds and serves assets; `static/js/app.js` is the only app JavaScript
- `styles/input.css`: Tailwind + daisyUI configuration (no `tailwind.config.js`)
- `scripts/install-tools.sh`: pinned versions and checksums of the Tailwind CLI and daisyUI bundle

## Rules

1. **Never edit `*_templ.go`.** They are generated and gitignored. Edit the `.templ` file, then `make generate`.
2. **Strict CSP** (`script-src 'self'; style-src 'self'`). Do not add inline `<script>` blocks, `style="…"`
   attributes, `<style>` tags, `hx-on:*`, `hx-vals="js:…"`, hx-trigger filters (`[…]`), or `javascript:` URLs.
   Put behaviour in `static/js/app.js` (event listeners on `document`, so they survive hx-boost swaps) and
   styles in `styles/input.css`. CDN assets are not allowed either; vendor files into `static/`. If the site
   needs a third-party origin (analytics, an embed, an image host), add it to `CSP` in
   `internal/site/site.go`, never to the baseline in `internal/middleware`.
3. **Assets must be embedded.** Reference them with `static.Path("dir/file")` in templates, never a literal
   `/static/...` URL. A new directory under `static/` must be added to the `//go:embed` line in
   `static/static.go`, or it will 404 in production. `TestLayoutHead` fails if a layout asset lacks its hash.
4. **Rendering:** full pages use `RenderPage(w, r, status, pagetemplates.X())`; htmx fragments use
   `Render(w, r, status, components.Y(...))`; a failure the visitor should see as a page uses
   `RenderError(w, r, status)`. Don't write HTML to `w` directly.
5. **Pages** wrap their content in `@layouts.Layout(layouts.Meta{Title, Description, Path})`. Add a nav link in
   `templates/layouts/layout.templ` if the page should appear in the header.
6. **Routes** go in `RegisterPageRoutes` in `internal/pages/pages.go`, above the `// scaffold:routes` marker.
   Keep the marker and keep `GET /` (the 404 catch-all) last. Pages that search engines should index also
   go in `sitemapPaths` in the same file, above `// scaffold:sitemap` (`make new-page` does both).
7. **Tailwind class names must appear literally** in `.templ`/`.go`/`app.js` files so the scanner finds them.
   Don't build class names by string concatenation (`"btn-" + size`); map to full class strings instead.
8. **Styling:** use daisyUI 5 components and semantic colours (`btn-primary`, `bg-base-200`,
   `text-base-content`). Don't add `dark:` variants for theme colours; themes switch via `data-theme`.
   See [docs/ui.md](docs/ui.md) before adding or replacing a UI library.
9. **State-changing requests** use POST/PUT/PATCH/DELETE. Cross-origin unsafe requests are rejected by
   `http.CrossOriginProtection`; don't disable it.
10. **Module path** appears in `.go` and `.templ` imports. Don't hand-edit it; `make setup MODULE=…` rewrites it.
11. **Per-site values** (site name, canonical URL, language, footer, theme colours, rate limit, extra CSP
    sources) live in `internal/site/site.go`. Don't hardcode them in templates or middleware.

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

Fragment: return only the swapped element with `Render(w, r, http.StatusOK, components.X(...))`. A 4xx/5xx
response to a fragment request is not swapped; app.js shows an error toast instead (see the htmx 4 notes below).
To show inline validation errors, re-render the form with its error markup and status 200. Full examples (fragment, validated form, SSE) are in
[docs/agents/recipes.md](docs/agents/recipes.md).

Live examples, until `make strip-examples` removes them: `internal/pages/examples.go` and `templates/components/examples.templ`.

## htmx 4

The app uses htmx 4, which differs from the htmx 2 most examples online are written for. Check
[four.htmx.org](https://four.htmx.org/docs/whats-new-in-htmx-4) before copying htmx code.

- **Inheritance is explicit.** An attribute only applies to descendants with `:inherited`, e.g.
  `hx-boost:inherited` and `hx-indicator:inherited` on `<body>`. `hx-target`, `hx-swap` etc. on a parent do
  nothing for children unless they carry `:inherited`.
- **Event names are `htmx:phase:action`**: `htmx:after:process` (was `htmx:load`), `htmx:before:swap`,
  `htmx:response:error`, `htmx:error` (network errors, timeouts). Event data is in `event.detail.ctx`;
  there is no `detail.xhr`, since requests use `fetch()`.
- **Error responses swap by default in htmx 4.** app.js turns that off for fragment requests: they toast and
  swap nothing, unless the element has a matching `hx-status:<code>` attribute, which then decides the
  swap. HTML error pages for navigations (boosted links/forms, history restores) are swapped in.
- **Renamed attributes:** `hx-disable` means "disable during the request" (was `hx-disabled-elt`);
  `hx-ignore` skips htmx processing (was `hx-disable`). `hx-vars`, `hx-params`, `hx-ext`, `hx-history`
  and `hx-disinherit` are gone.
- **Server side:** htmx sends `HX-Request-Type: partial` for targeted swaps and `full` for body swaps
  (boosted navigation, history restores, `hx-select`). `RenderPage` keys on it; use `pages.IsPartial(r)`
  rather than `HX-Boosted` checks.

## Testing

Handler tests live next to handlers and go through the real mux (`internal/pages/*_test.go`). When adding a
page or endpoint, add a test for status and key markup. Middleware and static packages have their own tests;
keep them passing with `make test-race`.

## Definition of done

1. `.templ` changes regenerated (`make generate`) and formatted (`make fmt`).
2. New pages and endpoints have tests; new pages are in `sitemapPaths` (which `TestPagesAreCSPCompatible`
   checks), unless they should not be indexed.
3. User-facing changes to config, commands or structure are reflected in README.md, and in this file or
   `docs/agents/` if they change a rule or recipe.
4. `make check` passes (format check, vet, staticcheck, docs references, race tests, generator smoke test,
   build).

## Don't

- Commit generated or local files: `*_templ.go`, `static/css/styles.css`, `bin/`, `.env`.
- Loosen the CSP (`'unsafe-inline'`, `'unsafe-eval'`, CDN hosts) or remove security middleware to make
  something work; restructure the code instead.
- Add a client-side framework, a CSS framework alongside daisyUI, or new Go dependencies or downloaded tools without being
  asked.
- Upgrade htmx, templ, Tailwind or daisyUI as a side effect of another task.
- Add a `package.json` or npm-based tooling; the CSS toolchain is the standalone CLI in `.tools/`.
- Push, tag or open PRs unless asked.

## Gotchas

- `APP_ENV` defaults to `production`. `.env` (from `.env.example`) sets `development`, which serves assets
  from disk; `static.Path` panics in development if the file doesn't exist (e.g. CSS not built yet).
- Streaming/SSE handlers must extend the 30s write timeout with
  `http.NewResponseController(w).SetWriteDeadline(...)`.
- Don't add Air (or other dev tools) to `go.mod` as a `tool`; Air pulls in a huge dependency tree. `make dev`
  uses templ's watcher.

# How go-htmx compares

This template is one way to build a server-rendered site. Here is what it gives you relative to two
common alternatives, and where it falls short.

## vs. Next.js / Nuxt

| | go-htmx | Next.js / Nuxt |
|---|---|---|
| Rendering | Server-rendered HTML; htmx swaps HTML fragments | SSR/SSG plus client-side hydration |
| Client JavaScript | htmx 2.0.10 + a small `app.js` (~18 KB gzipped together) | Framework runtime plus your components |
| Routing | Explicit `net/http` routes; `make new-page` adds them | File-based |
| Type safety | Go + templ, checked at compile time | TypeScript |
| Dev reload | templ watcher (browser reload, server restart) + Tailwind watch | Vite / Fast Refresh with state preservation |
| Production artifact | One ~8 MB static binary with assets embedded | Build output + `node_modules`, Node.js runtime |
| Container | ~19 MB distroless image | Typically a Node base image, much larger |
| Ecosystem | Go modules; any CSS/JS you vendor into `static/` | npm, component libraries |

**Choose go-htmx** for content sites, dashboards, admin panels and internal tools where most state lives
on the server, and where a small, dependency-light deployment matters.

**Choose Next.js or Nuxt** for apps with heavy client-side state or rich offline/interactive UI, or when
your team and component library are already JavaScript-first.

## vs. starting from scratch with Go

| | go-htmx | Plain `net/http` + `html/template` |
|---|---|---|
| Templates | templ: typed parameters, compile-time errors | Parsed at runtime, errors surface per request |
| Full page vs. fragment | `RenderPage` / `Render` handle htmx and boosted requests, `Vary` headers | Write it yourself |
| Static assets | Embedded, content-hashed URLs, immutable caching, ETags, precompressed gzip | `http.FileServer`, manual cache busting |
| Middleware | Logging, recovery, CSP and security headers, CSRF, rate limiting, gzip | Write or pick each one |
| Server | Timeouts, graceful shutdown, h2c, `/healthz`, `-healthcheck` | Write it yourself |
| Tooling | `make dev`, generators, `make check`, CI, Dockerfile | Set up yourself |

Everything here is plain Go with two direct dependencies (templ and godotenv), so you can read, change or
delete any part of it.

## Limitations

- **Node.js is required at build time** for Tailwind and daisyUI, though not at runtime.
- **No database, sessions, authentication or i18n.** Add the libraries you prefer.
- **No file-based routing.** Routes live in `internal/pages/pages.go`; the page generator inserts them.
- **The strict CSP rules out inline scripts and `hx-on:*`.** Client code goes in files under `static/js/`.
- **The rate limiter is in memory and per process.** With several instances, rate-limit at the proxy or
  use a shared store.

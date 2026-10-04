# Quick Start

You need Go 1.27+, make and curl. There's no Node.js or npm: the CSS is built with the Tailwind standalone
CLI, which setup downloads into `.tools/`.

## 1. Create a project

Pick one of these. Each ends with a project that has your module path, a `.env`, installed dependencies,
generated templ code and built CSS.

### Option A: scaffolder

```bash
curl -sSL https://raw.githubusercontent.com/tonymmm1/go-htmx/main/create-go-htmx.sh \
  | bash -s -- my-project github.com/you/my-project
```

Or download `create-go-htmx.sh` and run `bash create-go-htmx.sh my-project`; it prompts for the module
path (default `github.com/<your user>/my-project`). The script clones the template into `my-project/`,
removes its git history, runs `setup.sh` with your module path, and makes an initial git commit.

### Option B: clone or GitHub template

Clone the repository (or create a new repository from it on GitHub and clone that), then run setup with
your module path:

```bash
git clone https://github.com/tonymmm1/go-htmx.git my-project
cd my-project
bash setup.sh github.com/you/my-project
```

`setup.sh` updates `go.mod` and every import in `.go`, `.templ` and generator scripts.
`make setup MODULE=github.com/you/my-project` does the same; plain `make setup` keeps the current module path.

### Option C: gonew

```bash
go run golang.org/x/tools/cmd/gonew@latest github.com/tonymmm1/go-htmx github.com/you/my-project
cd my-project
make setup
git init
```

`gonew` rewrites imports in `.go` files only; `make setup` fixes the ones in `.templ` files.

## 2. Run it

```bash
make dev
```

Open **http://localhost:7331** (templ's live-reload proxy; the app itself listens on port 8080). Edit
`templates/pages/index.templ` and the browser reloads. Go changes restart the server. `/examples` shows
the htmx demos.

## 3. First changes

**Add a page:**

```bash
make new-page pricing
```

This creates `templates/pages/pricing.templ` and adds a `GET /pricing` route and `HandlePricing` handler
to `internal/pages/pages.go`. To link it from the nav, add a line next to the others in
`templates/layouts/layout.templ`:

```templ
@navLink(meta.Path, "/pricing", "Pricing")
```

**Add a component:**

```bash
make new-component pricing-card
```

Then import `github.com/you/my-project/templates/components` in a page and use
`@components.PricingCard("Pro")`.

**Return a fragment for htmx.** Add a route in `internal/pages/pages.go` and render a component with
`Render` (use `RenderPage` for full pages):

```go
mux.HandleFunc("GET /examples/time", h.HandleExampleTime)

func (h *Handler) HandleExampleTime(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	now := time.Now().Format("Mon, 02 Jan 2006 15:04:05 MST")
	Render(w, r, http.StatusOK, components.ServerTime(now))
}
```

`templates/pages/examples.templ` and `templates/components/examples.templ` show the matching markup.

**Rename the site:** change `SiteName` in `templates/layouts/meta.go`.

**Change themes or add CSS:** edit `styles/input.css`.

**Add JavaScript:** put it in `static/js/app.js`. The Content Security Policy blocks inline `<script>`,
`style="..."` and `hx-on:*` attributes.

**Add a new asset directory** (e.g. `static/fonts/`): add it to the `//go:embed` line in
`static/static.go`, or it won't be in the production binary.

## 4. Before you commit

```bash
make check   # format check, go vet + staticcheck, race tests, generator test, build
```

## 5. Deploy

```bash
make build && ./bin/server   # single binary, assets embedded
make docker-up               # or a ~19 MB distroless image
```

The app runs in production mode unless `APP_ENV=development` is set; note that `./bin/server` also loads
`.env` from the current directory if there is one. If it runs behind a reverse proxy, set
`TRUSTED_PROXIES` to the proxy's network so rate limiting sees real client IPs, and set HSTS on the proxy.
See the [README](README.md) for configuration, middleware and deployment details.

## Troubleshooting

- **`templ: command not found`**: templ is a Go tool in this project. Use `go tool templ generate` or
  `make generate`.
- **500 error mentioning `has the asset been built?`**: the CSS is missing. Run `make css`.
- **Port in use**: set `PORT` in `.env`, or run `PROXY_PORT=7332 make dev` for the proxy port.

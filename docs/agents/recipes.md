# Recipes

Step-by-step instructions for common changes. Each one ends with `make check`. Paths are relative to the
repo root, and `pagetemplates`/`components` are the package names of `templates/pages` and
`templates/components`.

## Add a page

1. `make new-page pricing` creates `pricing.templ` in `templates/pages/`, adds `GET /pricing` above
   `// scaffold:routes`, and adds `HandlePricing` to `internal/pages/pages.go`. To do it by hand, copy
   `templates/pages/about.templ` and `HandleAbout`.
2. Fill in `layouts.Meta`: a `Title` (rendered as "Pricing · Go-HTMX"), a one-sentence `Description`, and
   `Path` matching the route.
3. To show it in the header, add `@navLink(meta.Path, "/pricing", "Pricing")` to the `<ul>` in
   `templates/layouts/layout.templ`.
4. Add a test in `internal/pages/pages_test.go`:

   ```go
   func TestPricingPage(t *testing.T) {
   	recorder := performRequest(http.MethodGet, "/pricing", "")
   	if recorder.Code != http.StatusOK {
   		t.Fatalf("GET /pricing returned %d", recorder.Code)
   	}
   	if !strings.Contains(recorder.Body.String(), "<title>Pricing · Go-HTMX</title>") {
   		t.Error("missing page title")
   	}
   }
   ```

`TestPagesAreCSPCompatible` in `internal/pages/render_test.go` checks a fixed list of pages for inline
scripts and styles. Add new pages to its list.

## Add an htmx fragment endpoint

A fragment is HTML that htmx swaps into the current page. The handler returns only that element.

```templ
// templates/components/stats.templ
templ Stats(visitors int) {
	<div id="stats" class="stat">
		<div class="stat-title">Visitors</div>
		<div class="stat-value">{ strconv.Itoa(visitors) }</div>
	</div>
}
```

```go
// internal/pages/pages.go
mux.HandleFunc("GET /stats", h.HandleStats) // above // scaffold:routes

func (h *Handler) HandleStats(w http.ResponseWriter, r *http.Request) {
	Render(w, r, http.StatusOK, components.Stats(42))
}
```

```templ
<button class="btn" hx-get="/stats" hx-target="#stats" hx-swap="outerHTML">Refresh</button>
```

- Use `Render` for fragments and `RenderPage` only for components wrapped in `layouts.Layout`.
- Put `aria-live="polite"` on a wrapper that stays in the page, not on the element being swapped.
- Give buttons inside swapped content stable `id`s, so htmx keeps keyboard focus across `outerHTML` swaps.
- Set `Cache-Control: no-store` on fragments that must never be cached (see `HandleExampleTime`).

## Add a form with inline validation

htmx doesn't swap 4xx/5xx responses (app.js shows a toast instead). For errors the user should see next
to the field, re-render the form with status 200:

```templ
// templates/components/subscribe.templ
templ SubscribeForm(email, problem string) {
	<form id="subscribe" hx-post="/subscribe" hx-target="this" hx-swap="outerHTML" class="flex flex-col gap-2">
		<label class="label" for="subscribe-email">Email</label>
		<input
			id="subscribe-email"
			name="email"
			type="email"
			value={ email }
			class={ "input", templ.KV("input-error", problem != "") }
			if problem != "" {
				aria-invalid="true"
				aria-describedby="subscribe-error"
			}
		/>
		if problem != "" {
			<p id="subscribe-error" class="text-error text-sm">{ problem }</p>
		}
		<button class="btn btn-primary" type="submit">Subscribe</button>
	</form>
}

templ Subscribed() {
	<div id="subscribe" class="alert alert-success" role="status">Thanks, you're subscribed.</div>
}
```

```go
mux.HandleFunc("POST /subscribe", h.HandleSubscribe) // above // scaffold:routes

func (h *Handler) HandleSubscribe(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form data", http.StatusBadRequest)
		return
	}
	email := strings.TrimSpace(r.FormValue("email"))
	if _, err := mail.ParseAddress(email); err != nil {
		Render(w, r, http.StatusOK, components.SubscribeForm(email, "Enter a valid email address."))
		return
	}
	// ... save it ...
	Render(w, r, http.StatusOK, components.Subscribed())
}
```

- Validate on the server. HTML attributes like `type="email"` and `required` are a convenience only.
- Use 4xx only for requests the UI shouldn't produce (malformed input, unknown action); those toast.
- State-changing routes must not be `GET`. CSRF protection covers unsafe methods automatically.
- Test the error and success paths with `performRequest(http.MethodPost, "/subscribe", "email=x")`.

## Add a reusable component

`make new-component feature-card` creates `feature-card.templ` in `templates/components/` with
`templ FeatureCard(title string)`. Use it as `@components.FeatureCard("Fast")`, after importing
`<module>/templates/components`. Components take typed parameters; pass `templ.Attributes` or
`children...` when callers need to customise them.

## Add client-side behaviour

There are no inline handlers, and `hx-on` is disabled. Add a delegated listener to `static/js/app.js` and
mark elements with `data-*` attributes:

```js
// Open a daisyUI <dialog class="modal"> from any button with data-open-modal="<dialog id>".
document.addEventListener("click", function (event) {
  var button = event.target.closest("[data-open-modal]");
  if (!button) return;
  var dialog = document.getElementById(button.dataset.openModal);
  if (dialog) dialog.showModal();
});
```

- To initialise something on content htmx inserts (including the first page load), listen for
  `htmx:load` on `document`. app.js runs before htmx loads, so use the event rather than `htmx.onLoad`.
- Any Tailwind/daisyUI class that app.js adds must appear literally in app.js; it's in Tailwind's `@source`
  list.
- `hx-confirm="Delete this?"` works under the CSP and needs no JavaScript.
- For bigger scripts, add a file under `static/js/` and a `<script src={ static.Path("js/…") } defer>` tag
  in the layout.

## Add a static asset (images, fonts, a vendored library)

1. Put the file under an existing embedded directory (`static/img/`, `static/js/`) or create a new one.
2. If you created a directory, add it to the `//go:embed all:css img js` line in `static/static.go`.
3. Reference it with `static.Path("img/logo.svg")` in templates. Never write a literal `/static/...` URL in
   a template; it skips the content hash and the year-long cache.
4. Files referenced from CSS (`url(/static/fonts/x.woff2)` in `styles/input.css`) can't carry the hash.
   They're served with the 5-minute cache and ETag revalidation, which is fine for fonts.
5. Vendored third-party files: download from the npm registry or the project's release page, never a CDN at
   runtime. Record the version and source in the `static` package doc comment.

## Update htmx

Follow the "Vendored files" note at the top of `static/static.go`: replace `static/js/htmx.min.js` from
the new npm tarball, verify it against the SRI hash htmx publishes, and update the version and hash in
`static/static.go` and in `TestVendoredHTMXIntegrity` (`static/static_test.go`). Check the htmx changelog
for config changes that affect the layout's `htmxConfig` constant.

## Add a configuration variable

1. Add a field to `Config` and parse it in `LoadConfig` (`internal/config/config.go`). Return an error for
   invalid values rather than exiting.
2. Document it in `.env.example`, in the configuration table in `README.md`, and in `docker-compose.yml` if
   deployments need it.
3. Read it in handlers through `h.Config`, or pass it to middleware via `middleware.Options`.

## Add middleware

Middleware has the signature `func(http.Handler) http.Handler`. Add it to the slice in `Stack`
(`internal/middleware/middleware.go`), and think about order. It must sit inside `recoverPanics` to be
covered by panic recovery, and before `gzipResponses` if it needs the uncompressed body. Test it through
`Stack(http.HandlerFunc(okHandler), Options{})` as the existing tests in
`internal/middleware/middleware_test.go` do. Route-specific behaviour belongs in the handler, not in
global middleware.

## Stream responses (server-sent events)

The server's 30s `WriteTimeout` would cut long streams. Extend it per request:

```go
func (h *Handler) HandleEvents(w http.ResponseWriter, r *http.Request) {
	rc := http.NewResponseController(w)
	_ = rc.SetWriteDeadline(time.Time{}) // no deadline for this response
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-store")
	for {
		select {
		case <-r.Context().Done():
			return
		case <-time.After(5 * time.Second):
			fmt.Fprintf(w, "data: %s\n\n", time.Now().Format(time.RFC3339))
			_ = rc.Flush()
		}
	}
}
```

The gzip middleware skips `text/event-stream`, and the CSP allows same-origin `connect-src`. htmx's SSE
support is a separate extension (`htmx-ext-sse`), which would need vendoring into `static/js/`.

## Change the look

See [docs/ui.md](../ui.md): daisyUI themes and conventions, and switching to a shadcn/ui-style kit.

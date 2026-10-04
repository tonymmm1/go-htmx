# UI components: daisyUI and shadcn-style alternatives

The template ships with [daisyUI 5](https://daisyui.com/) on Tailwind CSS 4. This page covers how to use it
here, and how to switch to a shadcn/ui-style kit if you prefer that look.

## daisyUI (default)

Configured in `styles/input.css`; there is no `tailwind.config.js` in Tailwind 4. There's no npm either:
the Tailwind standalone CLI and daisyUI's `daisyui.mjs` bundle are downloaded into `.tools/` by
`scripts/install-tools.sh`, which pins their versions and checksums:

```css
@import "tailwindcss" source(none);
@source "../templates";
@source "../internal";
@source "../static/js/app.js";

@plugin "@tailwindcss/typography";
@plugin "../.tools/daisyui.mjs" {
    themes: light --default, dark --prefersdark;
}
```

- **Components are class names:** `btn btn-primary btn-sm`, `card`, `alert alert-error`, `menu`, `toast`.
  Docs: `https://daisyui.com/components/<name>/`. Fall back to Tailwind utilities for layout and spacing.
- **Use semantic colours** (`primary`, `secondary`, `accent`, `neutral`, `base-100/200/300`, `info`, `success`,
  `warning`, `error`, and their `-content` pairs). They change with the theme, so you don't need `dark:`.
- **Themes:** with no saved choice the OS preference decides (`--prefersdark`). The header toggle sets
  `data-theme` on `<html>` and saves it in `localStorage` (see `static/js/app.js`). Each extra theme in the
  plugin block adds CSS to every page, so only list the ones you use. Custom themes use
  `@plugin "daisyui/theme" { name: "brand"; … }`; the [theme generator](https://daisyui.com/theme-generator/)
  produces that block.
- **Interactive components without JavaScript:** daisyUI's modal (`<dialog>`), dropdown (`<details>` or
  popover API), collapse, drawer and tabs work with HTML and CSS only, so they are CSP-safe. Open a
  `<dialog>` from app.js with `showModal()`, never with an inline `onclick`.
- **Class names must be literal** in `.templ`/`.go` files. Write `if primary { class="btn btn-primary" }`
  rather than `"btn-" + variant`, or Tailwind won't generate the CSS.
- **For agents:** daisyUI publishes an LLM reference at <https://daisyui.com/llms.txt>. Point your agent at it
  when generating markup.

## shadcn/ui-style options

[shadcn/ui](https://ui.shadcn.com/) itself is React (Radix/Base UI) and doesn't run in a templ/htmx app. Two
projects provide its look without React:

| | [Basecoat](https://github.com/hunvreus/basecoat) | [shadcn-templ](https://templui.io/) (formerly templUI) |
|---|---|---|
| What it is | shadcn-style CSS components on Tailwind 4, plus small vanilla JS | shadcn/ui port as templ components you copy into the repo (CLI) or import |
| Markup | Plain HTML with short class names | templ components (`@button.Button(...)`) |
| Fits here | Closest to how daisyUI is used now; replace the plugin and update classes | Most "Go-native"; you own and edit the component code |
| Theming | shadcn CSS variables; compatible with shadcn themes | shadcn CSS variables |

Pick **one** component library. daisyUI and Basecoat both define `btn`, `card`, `input` and similar classes,
so loading both produces conflicting styles.

Check each project's own docs for exact install steps and APIs. Whichever you choose, it has to meet this
template's constraints:

1. **Tailwind 4 CSS-first config, without npm.** Replace the daisyUI `@plugin` line in `styles/input.css`
   with the library's CSS, and add `@source` lines for any templ component directories the library adds.
   This template has no `node_modules`, so a package import like Basecoat's `@import "basecoat-css";`
   won't resolve. Download the library's CSS from its release into the repo (or extend
   `scripts/install-tools.sh` with a pinned checksum) and import it by relative path, e.g.
   `@import "./vendor/basecoat.css";`.
2. **No CDN, no inline scripts.** The CSP is `script-src 'self'`. Vendor the library's JavaScript into
   `static/js/` (add new directories to the `//go:embed` line in `static/static.go`) and load it from the
   layout with `<script src={ static.Path("js/…") } defer></script>`. If a component emits inline `<script>`
   tags, either move that code into a file or add per-request CSP nonces (`templ.WithNonce` plus a nonce in
   the CSP header in `internal/middleware`). Moving the code is simpler.
3. **Re-initialise after htmx swaps.** Components that need JavaScript must be initialised on content htmx
   inserts. Listen for `htmx:load`, which fires for every new element including the initial page, in
   `static/js/app.js`, and call the library's init function on the new element. For example, Basecoat
   documents `window.basecoat.initAll()`:

   ```js
   document.addEventListener("htmx:load", function () {
     if (window.basecoat) window.basecoat.initAll();
   });
   ```

   shadcn-templ documents its own re-initialisation API for htmx in its integration guide.
4. **Dark mode.** shadcn-style kits usually toggle a `dark` class on `<html>`, while daisyUI uses
   `data-theme`. Update the theme code in `static/js/app.js`, the icon rules in `styles/input.css`, and
   (if the kit needs it) add `@custom-variant dark (&:where(.dark, .dark *));` to `styles/input.css`.
5. **Replace daisyUI classes** in `templates/**`, `static/js/app.js` (the toast uses `alert`, `btn`) and the
   generator templates in `scripts/new-page.sh` and `scripts/new-component.sh`. Remove the daisyUI download
   from `scripts/install-tools.sh`, then run `make check`.

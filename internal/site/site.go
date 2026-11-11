// Package site holds the values that differ from one site to the next. Start
// here when turning the template into a new site; the rest of internal/ should
// rarely need per-site edits, which keeps merges from the template clean.
package site

import "time"

const (
	// Name is the brand in the nav and the suffix of every page title.
	Name = "Go-HTMX"
	// Lang is the <html lang> attribute.
	Lang = "en"
	// Footer is the text in the page footer.
	Footer = "Built with Go, HTMX, Templ, and Tailwind CSS"

	// ThemeColorLight and ThemeColorDark tint the browser UI. Match the
	// base-100 colour of the light and dark themes in styles/input.css.
	ThemeColorLight = "#ffffff"
	ThemeColorDark  = "#1d232a"

	// RateLimit is how many requests one client IP may make per RateWindow.
	// htmx fragment requests count; /static/ and /healthz do not.
	RateLimit  = 100
	RateWindow = time.Minute
)

// These are variables only so tests can set them; treat them as constants.
var (
	// URL is the canonical production origin without a trailing slash, for
	// example "https://example.com". While it is empty, canonical links,
	// og:url, og:image and /sitemap.xml are left out.
	URL = ""

	// Image is the default link-preview image (og:image) as an asset name
	// under static/, for example "img/og.png". Pages override it with
	// layouts.Meta.Image. Empty means no preview image.
	Image = ""

	// CSP adds sources to the Content-Security-Policy, per directive, on top
	// of the same-origin baseline in internal/middleware. For example:
	//
	//	"script-src":  {"https://plausible.io"},
	//	"connect-src": {"https://plausible.io"},
	//	"frame-src":   {"https://www.youtube-nocookie.com"},
	//
	// Don't add 'unsafe-inline' or 'unsafe-eval'; see AGENTS.md.
	CSP = map[string][]string{}
)

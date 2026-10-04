// Package static serves the files in this directory (CSS, JS, images).
//
// STUB: the public API below is the contract other packages rely on; the
// implementation (embedding, cache-busting, compression) is pending.
package static

import (
	"net/http"
)

// Configure selects how assets are served. In development, files are read from
// dir on every request so Tailwind's watcher output shows up without a rebuild.
// It must be called once at startup, before Handler or Path are used.
func Configure(dev bool, dir string) {
	devDir = dir
	devMode = dev
}

var (
	devMode bool
	devDir  = "static"
)

// Handler serves assets. It is mounted at "GET /static/" and expects the
// full request path (it strips the /static/ prefix itself).
func Handler() http.Handler {
	return http.StripPrefix("/static/", http.FileServer(http.Dir(devDir)))
}

// Path returns the URL for an asset relative to this directory, for example
// Path("css/styles.css"). Production URLs include a content hash so they can
// be cached forever.
func Path(name string) string {
	return "/static/" + name
}

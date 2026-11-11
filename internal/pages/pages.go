package pages

import (
	"net/http"

	"github.com/tonymmm1/go-htmx/internal/config"
	pagetemplates "github.com/tonymmm1/go-htmx/templates/pages"
)

type Handler struct {
	Config *config.Config
}

// sitemapPaths lists the pages in /sitemap.xml. TestPagesAreCSPCompatible
// checks every one of them too.
var sitemapPaths = []string{
	"/",
	"/about",
	"/examples",
	// scaffold:sitemap -- new-page.sh inserts generated pages above this line.
}

func RegisterPageRoutes(h *Handler, mux *http.ServeMux) {
	mux.HandleFunc("GET /robots.txt", h.HandleRobots)
	mux.HandleFunc("GET /sitemap.xml", h.HandleSitemap)
	mux.HandleFunc("GET /favicon.ico", h.HandleFavicon)

	mux.HandleFunc("GET /{$}", h.HandleIndex)
	mux.HandleFunc("GET /about", h.HandleAbout)
	registerExampleRoutes(h, mux)
	// scaffold:routes -- new-page.sh inserts generated routes above this line.

	// Catch-all for unknown paths. It is "GET /" rather than "/" so a known
	// path with the wrong method still gets 405 Method Not Allowed.
	mux.HandleFunc("GET /", h.HandleNotFound)
}

func (h *Handler) HandleIndex(w http.ResponseWriter, r *http.Request) {
	RenderPage(w, r, http.StatusOK, pagetemplates.Index())
}

func (h *Handler) HandleAbout(w http.ResponseWriter, r *http.Request) {
	RenderPage(w, r, http.StatusOK, pagetemplates.About())
}

// HandleNotFound renders the 404 page for any path without a route.
func (h *Handler) HandleNotFound(w http.ResponseWriter, r *http.Request) {
	RenderPage(w, r, http.StatusNotFound, pagetemplates.NotFound(r.URL.Path))
}

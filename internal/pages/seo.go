package pages

import (
	"encoding/xml"
	"fmt"
	"io"
	"net/http"

	"github.com/tonymmm1/go-htmx/internal/site"
	"github.com/tonymmm1/go-htmx/static"
)

// HandleRobots allows all crawlers and points them at the sitemap.
func (h *Handler) HandleRobots(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = io.WriteString(w, "User-agent: *\nAllow: /\n")
	if site.URL != "" {
		_, _ = fmt.Fprintf(w, "Sitemap: %s/sitemap.xml\n", site.URL)
	}
}

type sitemapURL struct {
	Loc string `xml:"loc"`
}

type sitemapURLSet struct {
	XMLName xml.Name     `xml:"urlset"`
	XMLNS   string       `xml:"xmlns,attr"`
	URLs    []sitemapURL `xml:"url"`
}

// HandleSitemap lists sitemapPaths as absolute URLs. Sitemaps require those,
// so without site.URL there is nothing to serve.
func (h *Handler) HandleSitemap(w http.ResponseWriter, r *http.Request) {
	if site.URL == "" {
		h.HandleNotFound(w, r)
		return
	}
	set := sitemapURLSet{XMLNS: "http://www.sitemaps.org/schemas/sitemap/0.9"}
	for _, path := range sitemapPaths {
		set.URLs = append(set.URLs, sitemapURL{Loc: site.URL + path})
	}
	w.Header().Set("Content-Type", "application/xml; charset=utf-8")
	_, _ = io.WriteString(w, xml.Header)
	_ = xml.NewEncoder(w).Encode(set)
}

// HandleFavicon sends clients that ask for /favicon.ico without reading the
// page's <link rel="icon"> to the real icon instead of the 404 page.
func (h *Handler) HandleFavicon(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, static.Path("img/favicon.svg"), http.StatusFound)
}

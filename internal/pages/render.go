package pages

import (
	"bytes"
	"log/slog"
	"net/http"
	"strconv"
	"sync"

	"github.com/a-h/templ"
	"github.com/tonymmm1/go-htmx/templates/layouts"
	pagetemplates "github.com/tonymmm1/go-htmx/templates/pages"
)

// Render writes component as an HTML response with the given status code.
// Use it for fragments that htmx swaps into an existing page.
//
// The component is rendered into a buffer first, so a render error produces a
// clean 500 response instead of a half-written page.
func Render(w http.ResponseWriter, r *http.Request, status int, component templ.Component) {
	buf := bufferPool.Get().(*bytes.Buffer)
	buf.Reset()
	defer putBuffer(buf)

	if err := component.Render(r.Context(), buf); err != nil {
		slog.ErrorContext(r.Context(), "render failed", "method", r.Method, "path", r.URL.Path, "error", err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	h := w.Header()
	h.Set("Content-Type", "text/html; charset=utf-8")
	h.Set("Content-Length", strconv.Itoa(buf.Len()))
	w.WriteHeader(status)
	_, _ = buf.WriteTo(w)
}

// RenderPage writes a full page: a component wrapped in layouts.Layout.
//
// Normal navigations, boosted links and forms (hx-boost), history restores and
// hx-select requests get the whole document. htmx requests that target part of
// the page, e.g. hx-get="/about" hx-target="#panel", get only the page content
// because Layout skips its <html>, nav, and footer. The response sets Vary so
// caches never mix the two.
func RenderPage(w http.ResponseWriter, r *http.Request, status int, page templ.Component) {
	w.Header().Add("Vary", "HX-Request, HX-Request-Type")
	if IsPartial(r) {
		r = r.WithContext(layouts.WithContentOnly(r.Context()))
	}
	Render(w, r, status, page)
}

// RenderError writes the styled error page with the given 4xx/5xx status. Use
// it when a handler fails in a way the visitor should see as a page (a failed
// database query, for example); the panic recovery middleware uses it too.
func RenderError(w http.ResponseWriter, r *http.Request, status int) {
	w.Header().Set("Cache-Control", "no-store")
	RenderPage(w, r, status, pagetemplates.Error(status))
}

// IsHTMX reports whether htmx sent the request (including boosted links).
func IsHTMX(r *http.Request) bool {
	return r.Header.Get("HX-Request") == "true"
}

// IsBoosted reports whether the request came from an hx-boost link or form,
// which expects a full page.
func IsBoosted(r *http.Request) bool {
	return r.Header.Get("HX-Boosted") == "true"
}

// IsPartial reports whether htmx will swap the response into an element other
// than <body> (HX-Request-Type: partial). Boosted navigation, history restores
// and hx-select requests are "full" and expect a whole document; so does any
// request without the header, which is always safe to swap.
func IsPartial(r *http.Request) bool {
	return IsHTMX(r) && r.Header.Get("HX-Request-Type") == "partial"
}

// maxPooledBuffer keeps an unusually large page from pinning memory in the pool.
const maxPooledBuffer = 64 << 10

var bufferPool = sync.Pool{New: func() any { return new(bytes.Buffer) }}

func putBuffer(buf *bytes.Buffer) {
	if buf.Cap() <= maxPooledBuffer {
		bufferPool.Put(buf)
	}
}

package pages

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/tonymmm1/go-htmx/internal/config"
	"github.com/tonymmm1/go-htmx/templates/components"
	pagetemplates "github.com/tonymmm1/go-htmx/templates/pages"
)

var exampleTopics = []string{
	"Go standard library routing",
	"Templ components",
	"HTMX partial swaps",
	"Tailwind CSS + DaisyUI",
	"Docker deployment",
	"GitHub Actions CI",
}

type Handler struct {
	Config *config.Config
}

func RegisterPageRoutes(h *Handler, mux *http.ServeMux) {
	mux.HandleFunc("GET /{$}", h.HandleIndex)
	mux.HandleFunc("GET /about", h.HandleAbout)
	mux.HandleFunc("GET /examples", h.HandleExamples)
	mux.HandleFunc("GET /examples/time", h.HandleExampleTime)
	mux.HandleFunc("GET /examples/search", h.HandleExampleSearch)
	mux.HandleFunc("POST /examples/counter", h.HandleExampleCounter)
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

func (h *Handler) HandleExamples(w http.ResponseWriter, r *http.Request) {
	RenderPage(w, r, http.StatusOK, pagetemplates.Examples())
}

// HandleNotFound renders the 404 page for any path without a route.
func (h *Handler) HandleNotFound(w http.ResponseWriter, r *http.Request) {
	RenderPage(w, r, http.StatusNotFound, pagetemplates.NotFound(r.URL.Path))
}

// HandleExampleTime returns only the fragment HTMX swaps into the page.
func (h *Handler) HandleExampleTime(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	now := time.Now().Format("Mon, 02 Jan 2006 15:04:05 MST")
	Render(w, r, http.StatusOK, components.ServerTime(now))
}

// HandleExampleSearch demonstrates a debounced GET that returns a result fragment.
func (h *Handler) HandleExampleSearch(w http.ResponseWriter, r *http.Request) {
	query := strings.TrimSpace(r.URL.Query().Get("q"))
	results := make([]string, 0, len(exampleTopics))
	for _, topic := range exampleTopics {
		if query != "" && strings.Contains(strings.ToLower(topic), strings.ToLower(query)) {
			results = append(results, topic)
		}
	}

	Render(w, r, http.StatusOK, components.SearchResults(query, results))
}

// HandleExampleCounter demonstrates a POST that replaces a complete component.
func (h *Handler) HandleExampleCounter(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form data", http.StatusBadRequest)
		return
	}

	count, err := strconv.Atoi(r.FormValue("count"))
	if err != nil {
		http.Error(w, "count must be a number", http.StatusBadRequest)
		return
	}
	if count < -10 {
		count = -10
	} else if count > 10 {
		count = 10
	}

	switch r.FormValue("action") {
	case "decrement":
		if count > -10 {
			count--
		}
	case "increment":
		if count < 10 {
			count++
		}
	case "reset":
		count = 0
	default:
		http.Error(w, "unknown counter action", http.StatusBadRequest)
		return
	}

	Render(w, r, http.StatusOK, components.Counter(count))
}

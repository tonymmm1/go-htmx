package pages

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/tonymmm1/go-htmx/src/config"
	pagetemplates "github.com/tonymmm1/go-htmx/templates/pages"
)

type Handler struct {
	Config *config.Config
}

func RegisterPageRoutes(h *Handler, r chi.Router) {
	r.Get("/", h.HandleIndex)
	r.Get("/about", h.HandleAbout)
}

func (h *Handler) HandleIndex(w http.ResponseWriter, r *http.Request) {
	component := pagetemplates.Index()
	component.Render(r.Context(), w)
}

func (h *Handler) HandleAbout(w http.ResponseWriter, r *http.Request) {
	component := pagetemplates.About()
	component.Render(r.Context(), w)
}

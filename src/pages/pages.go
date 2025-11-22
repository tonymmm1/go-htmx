package pages

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/tonymmm1/go-htmx/src/config"
)

type Handler struct {
	Config *config.Config
}

func RegisterPageRoutes(h *Handler, r chi.Router) {
	r.Get("/", h.HandleIndex)
	r.Get("/about", h.HandleAbout)
}

func (h *Handler) HandleIndex(w http.ResponseWriter, r *http.Request) {
	component := Index()
	component.Render(r.Context(), w)
}

func (h *Handler) HandleAbout(w http.ResponseWriter, r *http.Request) {
	component := About()
	component.Render(r.Context(), w)
}

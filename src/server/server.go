package server

import (
	"fmt"
	"log"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/tonymmm1/go-htmx/src/config"
	"github.com/tonymmm1/go-htmx/src/middleware"
	"github.com/tonymmm1/go-htmx/src/pages"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
)

type Server struct {
	config *config.Config
}

func NewServer(config *config.Config) *Server {
	return &Server{
		config: config,
	}
}

func (s *Server) Run() error {
	// Initialize pages handler
	pagesHandler := &pages.Handler{
		Config: s.config,
	}

	// HTTP router
	r := chi.NewRouter()

	// Apply middleware stack
	middleware.Stack(r)

	// Static files
	fileServer := http.FileServer(http.Dir("./static"))
	r.Handle("/static/*", http.StripPrefix("/static/", fileServer))

	// HTTP routes
	pages.RegisterPageRoutes(pagesHandler, r) // Page routes

	h2s := &http2.Server{}
	srv := &http.Server{
		Addr:    fmt.Sprintf("0.0.0.0:%v", s.config.Port),
		Handler: h2c.NewHandler(r, h2s),
	}

	// Start server
	log.Println("Starting server on :" + s.config.Port)
	if err := srv.ListenAndServe(); err != nil {
		return fmt.Errorf("error starting server: %v", err)
	}

	return nil
}

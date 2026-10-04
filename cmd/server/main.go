package main

import (
	"log"

	"github.com/tonymmm1/go-htmx/internal/config"
	"github.com/tonymmm1/go-htmx/internal/server"
)

func main() {
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}
	srv := server.NewServer(cfg)
	if err := srv.Run(); err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}

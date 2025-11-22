package config

import (
	"log"
	"os"

	"github.com/joho/godotenv"
)

// Environment variables
type Config struct {
	Port string
}

// Load environment variables
func LoadConfig() (*Config, error) {
	log.Println("Loading configuration")

	// load .env after fetching environment variables
	if err := godotenv.Load(); err != nil && !os.IsNotExist(err) {
		log.Fatalln("Error loading .env")
	}

	// Fetch environment variables
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	config := &Config{
		Port: port,
	}

	log.Println("Configuration loaded")

	return config, nil
}

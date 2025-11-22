# Makefile – go-htmx stack

PORT      ?= 8080
IMAGE     ?= gohtmx
CONTAINER ?= gohtmx-dev

.PHONY: all dev build docker-build docker-up docker-down compose-up compose-dev compose-down clean tools deps setup new-page new-component

all: dev

# One-command setup (like create-nuxt-app)
setup:
	@bash setup.sh

# Install Go tools
tools:
	@echo "Installing Go tools..."
	@go install github.com/air-verse/air@latest
	@go install github.com/a-h/templ/cmd/templ@latest
	@echo "Tools installed successfully"

# Install dependencies
deps:
	@echo "Installing dependencies..."
	@go mod download
	@npm install
	@echo "Dependencies installed"

# Generate templ files and start development server
dev: tools deps
	@echo "Starting development server..."
	@mkdir -p static/css
	@templ generate
	@npx concurrently \
		"templ generate --watch --proxy='http://localhost:$(PORT)' --open-browser=false" \
		"npx tailwindcss -i ./src/styles/input.css -o ./static/css/styles.css --watch" \
		"air" \
		--names "templ,css,go" \
		--prefix-colors "blue,green,yellow" \
		--kill-others-on-fail

# Build for production
build: deps
	@echo "Building for production..."
	@mkdir -p bin static/css
	@npx tailwindcss -i ./src/styles/input.css -o ./static/css/styles.css --minify
	@templ generate
	@go build -ldflags="-s -w" -o bin/server ./src/cmd/main.go
	@echo "Build complete: ./bin/server"

# Run the built binary
run: build
	@./bin/server

# Docker build
docker-build:
	docker build -t $(IMAGE):latest .

# Docker run (standalone)
docker-up: docker-build
	docker run -d --name $(CONTAINER) -p $(PORT):8080 $(IMAGE):latest
	@echo "Container $(CONTAINER) running on http://localhost:$(PORT)"

# Docker stop and remove (standalone)
docker-down:
	docker stop $(CONTAINER) || true
	docker rm $(CONTAINER) || true

# Docker Compose - production
compose-up:
	docker-compose up -d
	@echo "Application running on http://localhost:$(PORT)"

# Docker Compose - development with hot reload
compose-dev:
	docker-compose --profile dev up dev

# Docker Compose - stop
compose-down:
	docker-compose down

# Clean build artifacts
clean:
	@echo "Cleaning build artifacts..."
	@rm -rf bin/ static/css/ tmp/ node_modules/.cache
	@find src -type f -name "*_templ.go" -delete
	@echo "Clean complete"

# Deep clean (including dependencies)
clean-all: clean
	@echo "Removing all dependencies..."
	@rm -rf node_modules/
	@go clean -modcache
	@echo "Deep clean complete"

# Run tests
test:
	@go test -v ./...

# Generate a new page
new-page:
	@bash scripts/new-page.sh $(filter-out $@,$(MAKECMDGOALS))

# Generate a new component
new-component:
	@bash scripts/new-component.sh $(filter-out $@,$(MAKECMDGOALS))

# Prevent make from treating arguments as targets
%:
	@:

# Show help
help:
	@echo "Go-HTMX Makefile Commands:"
	@echo ""
	@echo "Setup & Development:"
	@echo "  make setup         - 🚀 Complete project setup (run this first!)"
	@echo "  make dev           - Start development server with hot reload"
	@echo "  make build         - Build production binary"
	@echo "  make run           - Build and run the server"
	@echo ""
	@echo "Generators:"
	@echo "  make new-page <name>      - Generate a new page"
	@echo "  make new-component <name> - Generate a new component"
	@echo ""
	@echo "Docker:"
	@echo "  make docker-build  - Build Docker image"
	@echo "  make docker-up     - Build and run Docker container"
	@echo "  make docker-down   - Stop and remove Docker container"
	@echo "  make compose-up    - Start with docker-compose (production)"
	@echo "  make compose-dev   - Start with docker-compose (dev mode)"
	@echo "  make compose-down  - Stop docker-compose services"
	@echo ""
	@echo "Maintenance:"
	@echo "  make test          - Run tests"
	@echo "  make clean         - Clean build artifacts"
	@echo "  make clean-all     - Clean everything including dependencies"
	@echo "  make tools         - Install required Go tools"
	@echo "  make deps          - Install dependencies"
	@echo ""

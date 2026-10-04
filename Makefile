# Makefile – go-htmx stack

# PORT comes from the environment, then .env, then defaults to 8080
ENV_PORT   := $(shell sed -n 's/^PORT=//p' .env 2>/dev/null | tr -d "\"' ")
PORT       ?= $(or $(ENV_PORT),8080)
PROXY_PORT ?= 7331
PROXY_BIND ?= 127.0.0.1
IMAGE      ?= gohtmx
CONTAINER  ?= gohtmx-dev

STATICCHECK_VERSION ?= v0.8.1
GOVULNCHECK_VERSION ?= v1.8.0

GO_BUILD := CGO_ENABLED=0 go build -trimpath -ldflags="-s -w"
# Directories holding Go code (avoids walking node_modules and friends).
GO_DIRS  := cmd internal static templates

.DEFAULT_GOAL := help

.PHONY: all help setup tools deps generate css dev watch-templ watch-css \
	build run test test-race test-generators check-docs fmt fmt-check vet lint check audit \
	docker-build docker-up docker-down compose-up compose-dev compose-down \
	clean clean-all new-page new-component

all: dev

## Setup ---------------------------------------------------------------------

# One-command setup: fixes module paths, creates .env, installs deps, builds CSS
setup:
	@bash setup.sh $(MODULE)

# templ is pinned as a Go tool in go.mod, so downloading modules is enough
tools:
	@go mod download

deps: node_modules/.package-lock.json
	@go mod download

# Reinstall npm packages only when package.json or the lockfile changes
node_modules/.package-lock.json: package.json package-lock.json
	@npm install --no-audit --no-fund
	@touch $@

## Code generation -------------------------------------------------------------

generate:
	@go tool templ generate

css: node_modules/.package-lock.json
	@npm run --silent build:css

## Development ------------------------------------------------------------------

# Hot reload: templ watches .templ and .go files, regenerates code, restarts the
# server (`go run`) on Go changes and reloads the browser through its proxy.
# Open http://localhost:$(PROXY_PORT) (the proxy) rather than :$(PORT).
dev: node_modules/.package-lock.json
	@echo "Dev server: http://localhost:$(PROXY_PORT) (proxying :$(PORT))"
	@$(MAKE) --no-print-directory -j2 watch-templ watch-css

watch-templ:
	@APP_ENV=development PORT=$(PORT) go tool templ generate --watch \
		--cmd="go run ./cmd/server" \
		--proxy="http://localhost:$(PORT)" \
		--proxyport=$(PROXY_PORT) --proxybind=$(PROXY_BIND) \
		--open-browser=false

watch-css:
	@npm run --silent dev:css

## Build ------------------------------------------------------------------------

build: css generate
	@$(GO_BUILD) -o bin/server ./cmd/server
	@echo "Build complete: ./bin/server"

run: build
	@APP_ENV=production ./bin/server

## Quality ----------------------------------------------------------------------

# Tests need the CSS because static/ embeds it
test: css generate
	@go test ./...

test-race: css generate
	@CGO_ENABLED=1 go test -race ./...

# Verify generators still compile after a module rename
test-generators: css
	@bash scripts/test-generators.sh

# Verify paths, links and make targets referenced in the agent docs exist
check-docs:
	@bash scripts/check-docs.sh

fmt:
	@gofmt -w $(GO_DIRS)
	@go tool templ fmt templates

fmt-check:
	@unformatted="$$(gofmt -l $(GO_DIRS))"; \
	if [ -n "$$unformatted" ]; then echo "Run 'make fmt'; unformatted Go files:"; echo "$$unformatted"; exit 1; fi
	@go tool templ fmt -fail templates

vet: generate
	@go vet ./...

lint: vet
	@go run honnef.co/go/tools/cmd/staticcheck@$(STATICCHECK_VERSION) ./...

# Everything CI runs except the dependency audit and the Docker build
check: fmt-check lint check-docs test-race test-generators build

# npm packages are build-time tooling only (just the generated CSS ships), so
# only critical advisories fail the build; govulncheck covers the shipped binary.
audit: node_modules/.package-lock.json
	@npm audit --audit-level=critical
	@go run golang.org/x/vuln/cmd/govulncheck@$(GOVULNCHECK_VERSION) ./...

## Docker -----------------------------------------------------------------------

docker-build:
	docker build -t $(IMAGE):latest .

docker-up: docker-build
	docker run -d --name $(CONTAINER) -p $(PORT):8080 $(IMAGE):latest
	@echo "Container $(CONTAINER) running on http://localhost:$(PORT)"

docker-down:
	docker stop $(CONTAINER) || true
	docker rm $(CONTAINER) || true

compose-up:
	docker compose up -d app
	@echo "Application running on http://localhost:$(PORT)"

compose-dev:
	docker compose --profile dev up dev

compose-down:
	docker compose --profile dev down

## Cleanup ----------------------------------------------------------------------

clean:
	@rm -rf bin/ tmp/ static/css/* node_modules/.cache
	@find templates -type f -name "*_templ.go" -delete
	@echo "Clean complete"

clean-all: clean
	@rm -rf node_modules/
	@go clean -modcache
	@echo "Deep clean complete"

## Generators -------------------------------------------------------------------

new-page:
	@bash scripts/new-page.sh $(filter-out $@,$(MAKECMDGOALS))

new-component:
	@bash scripts/new-component.sh $(filter-out $@,$(MAKECMDGOALS))

# Lets generator arguments (make new-page about-us) pass through as goals
%:
	@:

help:
	@echo "Go-HTMX Makefile Commands:"
	@echo ""
	@echo "Setup & Development:"
	@echo "  make setup [MODULE=path]  - Complete project setup (run this first!)"
	@echo "  make dev                  - Hot-reload dev server on http://localhost:$(PROXY_PORT)"
	@echo "  make build                - Build production binary (./bin/server)"
	@echo "  make run                  - Build and run the server (APP_ENV=production)"
	@echo "  make generate             - Generate Go code from .templ files"
	@echo "  make css                  - Build minified CSS"
	@echo ""
	@echo "Generators:"
	@echo "  make new-page <name>      - Generate a new page"
	@echo "  make new-component <name> - Generate a new component"
	@echo ""
	@echo "Quality:"
	@echo "  make test                 - Run tests"
	@echo "  make test-race            - Run tests with the race detector"
	@echo "  make test-generators      - Smoke-test project generators"
	@echo "  make check-docs           - Check paths and targets referenced in AGENTS.md"
	@echo "  make fmt                  - Format Go and templ files"
	@echo "  make fmt-check            - Fail if Go or templ files need formatting"
	@echo "  make vet                  - go vet"
	@echo "  make lint                 - go vet + staticcheck"
	@echo "  make check                - Format check, lint, tests, build (CI)"
	@echo "  make audit                - npm audit + govulncheck"
	@echo ""
	@echo "Docker:"
	@echo "  make docker-build         - Build Docker image"
	@echo "  make docker-up            - Build and run Docker container"
	@echo "  make docker-down          - Stop and remove Docker container"
	@echo "  make compose-up           - Start with docker compose (production)"
	@echo "  make compose-dev          - Start with docker compose (dev mode)"
	@echo "  make compose-down         - Stop docker compose services"
	@echo ""
	@echo "Maintenance:"
	@echo "  make deps                 - Install Go and npm dependencies"
	@echo "  make tools                - Download Go tools (templ is a go.mod tool)"
	@echo "  make clean                - Clean build artifacts"
	@echo "  make clean-all            - Clean everything including dependencies"

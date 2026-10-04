# syntax=docker/dockerfile:1

ARG GO_VERSION=1.27

# --- Development: Go toolchain for `docker compose --profile dev up dev`
#     (make dev downloads the Tailwind CLI into .tools/ on first run)
FROM golang:${GO_VERSION}-bookworm AS development
WORKDIR /app
EXPOSE 8080 7331
CMD ["make", "dev"]

# --- Build: CSS with the Tailwind standalone CLI, then a static Go binary
#     (static/ is embedded into it). No Node.js involved.
FROM golang:${GO_VERSION}-bookworm AS build
WORKDIR /src
COPY scripts/install-tools.sh ./scripts/
RUN bash scripts/install-tools.sh
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download
COPY . .
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    .tools/tailwindcss -i ./styles/input.css -o ./static/css/styles.css --minify && \
    go tool templ generate && \
    CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/server ./cmd/server

# --- Runtime: just the binary
FROM gcr.io/distroless/static-debian12:nonroot
WORKDIR /app
COPY --from=build /out/server /app/server
ENV APP_ENV=production \
    PORT=8080
USER nonroot
EXPOSE 8080
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD ["/app/server", "-healthcheck"]
ENTRYPOINT ["/app/server"]

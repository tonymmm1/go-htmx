# syntax=docker/dockerfile:1

ARG GO_VERSION=1.27
ARG NODE_VERSION=24

FROM node:${NODE_VERSION}-bookworm-slim AS node

# --- CSS: Tailwind only needs node_modules, styles/ and the sources it scans
#     (see the @source lines in styles/input.css)
FROM node AS css
WORKDIR /app
COPY package.json package-lock.json ./
RUN --mount=type=cache,target=/root/.npm npm ci --no-audit --no-fund
COPY styles ./styles
COPY templates ./templates
COPY internal ./internal
COPY static ./static
RUN npm run build:css

# --- Development: Go + Node toolchain for `docker compose --profile dev up dev`
FROM golang:${GO_VERSION}-bookworm AS development
COPY --from=node /usr/local/ /usr/local/
WORKDIR /app
EXPOSE 8080 7331
CMD ["make", "dev"]

# --- Build: static Go binary (static/ is embedded into it)
FROM golang:${GO_VERSION}-bookworm AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download
COPY . .
COPY --from=css /app/static/css ./static/css
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
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

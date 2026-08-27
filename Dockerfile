ARG GO_VERSION=1.27.0
ARG NODE_VERSION=24
ARG TEMPL_VERSION=v0.3.1020

FROM node:${NODE_VERSION}-bookworm-slim AS node-runtime

FROM golang:${GO_VERSION}-bookworm AS toolchain

# Use the same supported Node.js toolchain for development and production builds.
COPY --from=node-runtime /usr/local/ /usr/local/

WORKDIR /app

FROM toolchain AS development

FROM toolchain AS builder

ARG TEMPL_VERSION
RUN go install github.com/a-h/templ/cmd/templ@${TEMPL_VERSION}

COPY go.mod go.sum ./
RUN go mod download

COPY package.json package-lock.json ./
RUN npm ci

COPY . .

RUN npm run build:css
RUN templ generate
RUN CGO_ENABLED=0 go build \
    -ldflags="-s -w" \
    -o server \
    ./src/cmd/main.go

FROM debian:bookworm-slim

RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    wget \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app

COPY --from=builder /app/server .
COPY --from=builder /app/static ./static

RUN groupadd -g 1000 appuser && \
    useradd -u 1000 -g appuser -s /bin/bash -m appuser && \
    chown -R appuser:appuser /app

USER appuser

EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8080/ || exit 1

CMD ["./server"]

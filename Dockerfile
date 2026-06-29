# syntax=docker/dockerfile:1

# --- build stage ---
FROM golang:1.26-alpine AS build
WORKDIR /src

# Cache dependencies first for faster rebuilds.
COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Version metadata, injected by the release workflow (defaults for local builds).
ARG VERSION=dev
ARG COMMIT=none
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-s -w -X main.version=${VERSION} -X main.commit=${COMMIT}" \
    -o /out/proxmox-license-proxy .

# --- runtime stage ---
FROM alpine:3.24

# ca-certificates for any outbound TLS, wget for the healthcheck.
RUN apk add --no-cache ca-certificates wget \
    && adduser -D -u 10001 pmox \
    && mkdir -p /data \
    && chown pmox:pmox /data

COPY --from=build /out/proxmox-license-proxy /usr/local/bin/proxmox-license-proxy

USER pmox

# Non-privileged port (mapped to 443 on the host via compose).
EXPOSE 8443
VOLUME ["/data"]

# Sensible container defaults (override via compose if needed).
ENV PMOX_LISTEN=":8443" \
    PMOX_TLS_MODE="auto" \
    PMOX_REGISTRY_FILE="/data/registry.json" \
    PMOX_LOG="info"

HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget -qO- --no-check-certificate https://127.0.0.1:8443/readyz || exit 1

ENTRYPOINT ["proxmox-license-proxy"]
CMD ["serve"]

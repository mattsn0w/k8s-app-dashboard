# deploy-dashboard: a single static Go binary with the UI embedded (//go:embed).
# Build with:  docker build -t registry.co.slakin.net/local/deploy-dashboard:latest .
#
# Multi-stage: build the binary in a throwaway golang image, ship it in a tiny
# runtime image. CGO_ENABLED=0 produces a fully static binary, so the runtime
# needs no libc at all (scratch-sized base, only ca-certificates for the k8s API TLS).

# ── Build stage ───────────────────────────────────────────────
FROM golang:1.27-alpine AS builder
WORKDIR /src

# Cache deps first, then copy the source.
COPY go.mod go.sum* ./
RUN go mod download 2>/dev/null || true
COPY . .

# Static build. The UI is embedded into the binary at build time (//go:embed in
# embed.go) — there is no separate web/ asset to copy.
#
# `go clean -cache` is required: the golang base image ships a warm GOCACHE,
# and without clearing it, a rebuild can serve a stale embedded copy of
# web/index.html (the object cache keys off package content that buildkit
# doesn't always re-hint), so the binary would silently contain the OLD UI even
# though the source file is new. Clearing the cache makes the embed
# deterministic.
#
# The `grep` guard fails the build if the freshly-built binary does not contain
# a marker that must exist in the current web/index.html, so a stale embed is
# caught here (at build time, in the builder stage where the source is known
# fresh) rather than discovered in production.
RUN go clean -cache && CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /deploy-dashboard ./cmd/ && \
    MARKER=$(grep -oE 'function showToast|confirmAction|toastContainer' web/index.html | head -1) && \
    if [ -n "$MARKER" ] && ! strings /deploy-dashboard | grep -q "$MARKER"; then \
      echo "ERROR: binary does not embed the current web/index.html (stale //go:embed)." >&2; \
      exit 1; \
    fi

# ── Runtime stage ─────────────────────────────────────────────
FROM alpine:3.20

# A non-root user for the runtime. NOTE: no `apk add` here — the in-cluster
# K8s client reads the API-server CA directly from the service-account mount
# (/var/run/secrets/kubernetes.io/serviceaccount/ca.crt), so the system CA
# bundle is not needed, and `apk` would require outbound network (blocked in
# the build environment). addgroup/adduser ship with the Alpine base image.
RUN addgroup -g 10001 appgroup \
    && adduser -D -u 10001 -G appgroup appuser

COPY --from=builder /deploy-dashboard /usr/local/bin/deploy-dashboard

USER 10001:10001
EXPOSE 8080

# --base-path is set by the Deployment (see manifests/03-deployment.yaml); the
# defaults here keep the image runnable standalone for local testing.
ENTRYPOINT ["deploy-dashboard"]
CMD ["--listen=:8080", "--namespace=apps", "--base-path=/app"]

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
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /deploy-dashboard ./cmd/

# ── Runtime stage ─────────────────────────────────────────────
FROM alpine:3.20

# ca-certificates for the in-cluster K8s API (https) + a non-root user.
RUN apk add --no-cache ca-certificates \
    && addgroup -g 10001 appgroup \
    && adduser -D -u 10001 -G appgroup appuser

COPY --from=builder /deploy-dashboard /usr/local/bin/deploy-dashboard

USER 10001:10001
EXPOSE 8080

# --base-path is set by the Deployment (see manifests/03-deployment.yaml); the
# defaults here keep the image runnable standalone for local testing.
ENTRYPOINT ["deploy-dashboard"]
CMD ["--listen=:8080", "--namespace=apps", "--base-path=/app"]

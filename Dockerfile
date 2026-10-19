# ── Build stage ───────────────────────────────────────────────
FROM golang:1.27-alpine AS builder

WORKDIR /app
COPY go.mod ./
COPY . .

# Embed the HTML into the binary
RUN go build -o /deploy-dashboard ./cmd/

# ── Runtime stage ─────────────────────────────────────────────
FROM alpine:3.20

# Install ca-certificates for the k8s API TLS
RUN apk add --no-cache ca-certificates

COPY --from=builder /deploy-dashboard /usr/local/bin/deploy-dashboard

EXPOSE 8080

ENTRYPOINT ["deploy-dashboard"]

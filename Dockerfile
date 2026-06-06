# ==============================================================================
# STAGE 1: Builder (Build & Key Generation)
# ==============================================================================
FROM golang:1.23-alpine AS builder

# Install build-time tools for SSL key generation and JSON formatting
RUN apk add --no-cache openssl 

ENV CGO_ENABLED=0 \
    GOOS=linux

WORKDIR /build

# 1. Generate SSL Keys and certs.json
RUN mkdir -p certs && \
    openssl req -x509 -newkey rsa:4096 -sha256 -days 3650 -nodes \
      -keyout certs/payformeproxy-ca.key \
      -out certs/payformeproxy-ca.crt \
      -subj "/C=UK/ST=London/L=London/O=402Proxy/OU=402Proxy/CN=proxy"

# 2. Build Go application
COPY go.mod go.sum ./
RUN go mod download

COPY cmd/ ./cmd/
COPY internal/ ./internal/

RUN go build -ldflags="-s -w" -o /app/payformeproxy ./cmd/payformeproxy


# ==============================================================================
# STAGE 2: Runtime (Production Environment)
# ==============================================================================
FROM alpine:3.19

RUN apk add --no-cache ca-certificates

WORKDIR /app

# Create unprivileged system user/group
RUN addgroup -S appgroup && adduser -S appuser -G appgroup

# Copy Go binary and generated SSL keys / configuration
COPY --from=builder /app/payformeproxy /app/payformeproxy
COPY --from=builder /build/certs/ /app/certs/

# Secure files for the unprivileged user
RUN chown -R appuser:appgroup /app

EXPOSE 8089

USER appuser

ENTRYPOINT ["/app/payformeproxy"]

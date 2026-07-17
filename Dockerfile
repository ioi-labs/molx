# syntax=docker/dockerfile:1

# -----------------------------------------------------------------------------
# Build stage
# -----------------------------------------------------------------------------
FROM golang:1.25-alpine AS builder

WORKDIR /app

# Install ca-certificates for TLS during go mod download
RUN apk add --no-cache ca-certificates

# Copy go module files and download dependencies first for better layer caching
COPY go.mod go.sum ./
RUN go mod download

# Copy application source
COPY . .

# Build the Go binary. CGO_ENABLED=0 for static binary.
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o nexora-crawl .

# -----------------------------------------------------------------------------
# Runtime stage
# -----------------------------------------------------------------------------
FROM alpine:3.21 AS runner

WORKDIR /app

# Install ca-certificates so HTTPS requests work
RUN apk add --no-cache ca-certificates

# Create non-root user
RUN adduser -D -u 1000 appuser

# Copy built binary
COPY --from=builder /app/nexora-crawl /app/nexora-crawl

# Copy runtime assets (OpenAPI spec and Scalar bundle) used by HTTP handlers
COPY --from=builder /app/docs/openapi.yaml /app/docs/openapi.yaml
COPY --from=builder /app/deps/scalar-standalone.js /app/deps/scalar-standalone.js

# Copy the correct Obscura binaries for the target architecture.
# IMPORTANT: both files must be Linux binaries for the matching architecture,
# not the macOS ones.
# Place binaries under:
#   deps/obscura/linux/<amd64|arm64>/obscura
#   deps/obscura/linux/<amd64|arm64>/obscura-worker
ARG OBSCURA_DIR=deps/obscura/linux
ARG TARGETARCH
COPY ${OBSCURA_DIR}/${TARGETARCH}/obscura /app/deps/obscura
COPY ${OBSCURA_DIR}/${TARGETARCH}/obscura-worker /app/deps/obscura-worker

# Ensure binaries are executable
RUN chmod +x /app/deps/obscura /app/deps/obscura-worker /app/nexora-crawl

# Switch to non-root user
USER appuser

# Expose the default port (can be overridden via PORT env var)
EXPOSE 8080

# Run the server
ENTRYPOINT ["/app/nexora-crawl"]

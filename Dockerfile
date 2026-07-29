# =============================================================================
# SubShare - Backend Dockerfile
# =============================================================================
# Stage 1: Build Go backend binary
# Stage 2: Minimal runtime image
#
# The Next.js frontend runs in its own container (see frontend/Dockerfile).
# =============================================================================

# ---------------------------------------------------------------------------
# Build arguments
# ---------------------------------------------------------------------------
ARG GO_VERSION=1.24
ARG ALPINE_VERSION=3.21

# ---------------------------------------------------------------------------
# Stage 1 - Go backend build
# ---------------------------------------------------------------------------
FROM golang:${GO_VERSION}-alpine AS backend-builder

WORKDIR /src

# Download dependencies first (layer cache: only re-runs if go.mod/go.sum change)
COPY go.mod go.sum ./
RUN go mod download && go mod verify

# Copy source and build
COPY cmd/ ./cmd/
COPY internal/ ./internal/

ARG VERSION=dev
ARG COMMIT=unknown
ARG BUILD_TIME=unknown

RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-s -w -X main.version=${VERSION} -X main.commit=${COMMIT} -X main.buildTime=${BUILD_TIME}" \
    -trimpath \
    -o /server \
    ./cmd/server

# ---------------------------------------------------------------------------
# Stage 2 - Runtime
# ---------------------------------------------------------------------------
FROM alpine:${ALPINE_VERSION} AS runtime

# OCI image labels
LABEL org.opencontainers.image.title="SubShare" \
      org.opencontainers.image.description="Self-hosted subscription delivery and configuration operations hub" \
      org.opencontainers.image.source="https://github.com/romanpodg/SubShare-Go" \
      org.opencontainers.image.licenses="MIT"

# Install minimal runtime dependencies in a single layer
# curl is used for the HEALTHCHECK probe
RUN apk add --no-cache ca-certificates tzdata curl \
    && addgroup -g 1000 appgroup \
    && adduser -u 1000 -G appgroup -D -h /app appuser \
    && mkdir -p /app/data \
    && chown -R appuser:appgroup /app

WORKDIR /app

# Copy built artifact from builder stage
COPY --from=backend-builder --chown=appuser:appgroup /server ./server

# Persistent volume for SQLite database
VOLUME /app/data

EXPOSE 8080

# Health check: verify the Go server responds and the database is reachable
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD curl -sf http://127.0.0.1:8080/health || exit 1

# Drop to non-root user
USER appuser

# Use exec form so the Go binary receives signals directly (SIGTERM for graceful shutdown)
CMD ["./server"]

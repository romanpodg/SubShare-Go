# =============================================================================
# SubShare - Frontend Dockerfile
# =============================================================================
# Stage 1: Build Next.js static export
# Stage 2: Serve with Caddy and reverse-proxy API to the backend
# =============================================================================

# ---------------------------------------------------------------------------
# Build arguments
# ---------------------------------------------------------------------------
ARG NODE_VERSION=22
ARG CADDY_VERSION=2.11.4-alpine

# ---------------------------------------------------------------------------
# Stage 1 - Build static export
# ---------------------------------------------------------------------------
FROM node:${NODE_VERSION}-alpine AS builder

WORKDIR /app

# Install dependencies first (layer cache: only re-runs if lock file changes)
COPY package.json package-lock.json ./
RUN npm ci --no-audit --no-fund

# Copy source and build static export
COPY . .
RUN npm run build

# ---------------------------------------------------------------------------
# Stage 2 - Serve with Caddy
# ---------------------------------------------------------------------------
FROM caddy:${CADDY_VERSION} AS runtime

# OCI image labels
LABEL org.opencontainers.image.title="SubShare Frontend" \
      org.opencontainers.image.description="Technical operations interface for SubShare" \
      org.opencontainers.image.source="https://github.com/romanpodg/vless-keys-to-sub" \
      org.opencontainers.image.licenses="MIT"

COPY Caddyfile /etc/caddy/Caddyfile
COPY --from=builder /app/out /srv

EXPOSE 80 443 443/udp

# The admin endpoint is bound to loopback and provides a stable local probe
# regardless of whether the public site uses HTTP or automatic HTTPS.
HEALTHCHECK --interval=30s --timeout=5s --start-period=5s --retries=3 \
    CMD wget -q --spider http://127.0.0.1:2019/config/ || exit 1

CMD ["caddy", "run", "--config", "/etc/caddy/Caddyfile", "--adapter", "caddyfile"]

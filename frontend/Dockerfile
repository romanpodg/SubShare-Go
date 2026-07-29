# =============================================================================
# SubShare - Frontend Dockerfile
# =============================================================================
# Stage 1: Build Next.js static export
# Stage 2: Serve with nginx and reverse-proxy API to the backend
# =============================================================================

# ---------------------------------------------------------------------------
# Build arguments
# ---------------------------------------------------------------------------
ARG NODE_VERSION=22
ARG NGINX_VERSION=1.27.3-alpine

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
# Stage 2 - Serve with nginx
# ---------------------------------------------------------------------------
FROM nginxinc/nginx-unprivileged:${NGINX_VERSION} AS runtime

# OCI image labels
LABEL org.opencontainers.image.title="SubShare Frontend" \
      org.opencontainers.image.description="Technical operations interface for SubShare" \
      org.opencontainers.image.source="https://github.com/romanpodg/SubShare-Go" \
      org.opencontainers.image.licenses="MIT"

# Switch to root to set up config and static files, then back to nginx user
USER root
RUN rm -rf /etc/nginx/conf.d/default.conf /usr/share/nginx/html/*
COPY nginx.conf /etc/nginx/conf.d/default.conf
COPY --from=builder /app/out /usr/share/nginx/html
RUN chown -R nginx:nginx /usr/share/nginx/html
USER nginx

EXPOSE 3000 3443

# Health check: verify nginx is serving the frontend
HEALTHCHECK --interval=30s --timeout=5s --start-period=5s --retries=3 \
    CMD wget -qO /dev/null http://127.0.0.1:3000/ || exit 1

CMD ["nginx", "-g", "daemon off;"]

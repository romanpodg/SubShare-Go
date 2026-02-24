#!/usr/bin/env bash
# =============================================================================
# Xray Sub — First-time SSL certificate setup (Let's Encrypt)
# =============================================================================
# Usage:
#   1. Set DOMAIN= and SSL_EMAIL= in your .env file
#   2. Make sure ports 80 and 443 are open in your firewall
#   3. Run: bash scripts/init-ssl.sh
# =============================================================================

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(dirname "$SCRIPT_DIR")"
# Certbot saves certs to live/cert-0001 when live/cert already existed
CERT_DIR="$ROOT_DIR/certs/live/cert-0001"
CERT_DIR_BOOTSTRAP="$ROOT_DIR/certs/live/cert"

# Load .env
if [[ -f "$ROOT_DIR/.env" ]]; then
  export $(grep -v '^#' "$ROOT_DIR/.env" | grep -v '^$' | xargs)
fi

if [[ -z "${DOMAIN:-}" ]]; then
  echo "ERROR: DOMAIN is not set in .env"; exit 1
fi
if [[ -z "${SSL_EMAIL:-}" ]]; then
  echo "ERROR: SSL_EMAIL is not set in .env"; exit 1
fi

cd "$ROOT_DIR"

# Step 1: Temporary self-signed cert so nginx can start before Let's Encrypt
if [[ ! -f "$CERT_DIR_BOOTSTRAP/fullchain.pem" ]] && [[ ! -f "$CERT_DIR/fullchain.pem" ]]; then
  echo "==> Creating temporary self-signed certificate..."
  mkdir -p "$CERT_DIR_BOOTSTRAP"
  openssl req -x509 -nodes -newkey rsa:2048 -days 1 \
    -keyout "$CERT_DIR_BOOTSTRAP/privkey.pem" \
    -out "$CERT_DIR_BOOTSTRAP/fullchain.pem" \
    -subj "/CN=$DOMAIN" 2>/dev/null
  # nginx runs as uid 101 — certs must be world-readable
  chmod 644 "$CERT_DIR_BOOTSTRAP/privkey.pem" "$CERT_DIR_BOOTSTRAP/fullchain.pem"
fi

# Step 2: Build and start services (always rebuild to pick up nginx.conf changes)
echo "==> Building and starting services..."
docker compose up -d --build frontend backend
sleep 6

# Step 3: Obtain real Let's Encrypt certificate
# Archive self-signed cert so certbot can create a fresh live/cert directory
if [[ -f "$CERT_DIR_BOOTSTRAP/fullchain.pem" ]]; then
  mv "$CERT_DIR_BOOTSTRAP" "${CERT_DIR_BOOTSTRAP}.selfsigned" 2>/dev/null || true
fi

# Skip if real cert already exists
if [[ -f "$CERT_DIR/fullchain.pem" ]]; then
  echo "==> Certificate already exists at $CERT_DIR, skipping certbot."
else
  echo "==> Obtaining Let's Encrypt certificate for ${DOMAIN}..."
  docker compose run --rm certbot

  # Fix permissions — certbot creates keys as root:root 600, nginx needs to read them
  echo "==> Fixing certificate permissions..."
  find "$ROOT_DIR/certs" -name "*.pem" -exec chmod 644 {} \;
  find "$ROOT_DIR/certs" -type d -exec chmod 755 {} \;
fi

# Step 4: Reload nginx to pick up the real certificate
echo "==> Reloading nginx..."
docker compose exec frontend nginx -s reload 2>/dev/null || docker compose restart frontend

echo ""
echo "✓ Done! Panel is now available at: https://${DOMAIN}/admin"
echo ""
echo "To renew: docker compose run --rm certbot renew && docker compose exec frontend nginx -s reload"

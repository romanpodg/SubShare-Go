#!/usr/bin/env bash
# Export the latest integrity-checked backup from the persistent Docker volume.

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
BACKUP_DIR="${1:-$ROOT_DIR/backups}"
TIMESTAMP="$(date -u +%Y%m%d_%H%M%S)"
OUT="$BACKUP_DIR/app_${TIMESTAMP}.db"
KEYRING_OUT="$BACKUP_DIR/keyring_${TIMESTAMP}.json"

cd "$ROOT_DIR"
mkdir -p "$BACKUP_DIR"

if ! docker compose exec -T backend sh -ec 'test -f /app/data/keyring.json'; then
  echo "ERROR: no file-backed keyring exists in the data volume. Preserve PROFILE_ENCRYPTION_KEYRING_JSON securely before exporting this database." >&2
  exit 1
fi
docker compose cp backend:/app/data/backups/app.db "$OUT"
docker compose cp backend:/app/data/keyring.json "$KEYRING_OUT"
chmod 600 "$OUT"
chmod 600 "$KEYRING_OUT"

echo "Backup exported to: $OUT"
echo "Matching encryption keyring exported to: $KEYRING_OUT"

#!/usr/bin/env bash
# Export the latest integrity-checked backup from the persistent Docker volume.

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
BACKUP_DIR="${1:-$ROOT_DIR/backups}"
TIMESTAMP="$(date -u +%Y%m%d_%H%M%S)"
OUT="$BACKUP_DIR/app_${TIMESTAMP}.db"

cd "$ROOT_DIR"
mkdir -p "$BACKUP_DIR"

docker compose cp backend:/app/data/backups/app.db "$OUT"
chmod 600 "$OUT"

echo "Backup exported to: $OUT"

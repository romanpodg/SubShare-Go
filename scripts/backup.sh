#!/usr/bin/env bash
# =============================================================================
# SubShare — Database backup
# =============================================================================
# Creates a consistent snapshot of app.db using SQLite VACUUM INTO (safe to
# run while the database is in use by the backend container).
#
# Usage:
#   bash scripts/backup.sh              # backup to ./backups/
#   bash scripts/backup.sh /my/dir      # backup to custom directory
# =============================================================================

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
BACKUP_DIR="${1:-$ROOT_DIR/backups}"
DB="$ROOT_DIR/data/app.db"
TIMESTAMP="$(date +%Y%m%d_%H%M%S)"
OUT="$BACKUP_DIR/app_${TIMESTAMP}.db"

mkdir -p "$BACKUP_DIR"

if [[ ! -f "$DB" ]]; then
  echo "ERROR: Database not found at $DB"; exit 1
fi

echo "==> Backing up $DB → $OUT"
sqlite3 "$DB" "VACUUM INTO '$OUT';"
chmod 600 "$OUT"
SIZE="$(du -h "$OUT" | cut -f1)"
echo "✓ Backup created: $OUT ($SIZE)"

# Keep only the last 30 backups
KEPT=30
COUNT=$(ls "$BACKUP_DIR"/app_*.db 2>/dev/null | wc -l)
if (( COUNT > KEPT )); then
  REMOVE=$(( COUNT - KEPT ))
  ls -t "$BACKUP_DIR"/app_*.db | tail -n "$REMOVE" | xargs rm -f
  echo "  Removed $REMOVE old backup(s), kept $KEPT"
fi

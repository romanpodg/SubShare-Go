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

# Match backend TrimSpace without treating UTF-8 path bytes as pattern ranges.
trim_backup_path() {
  local path="$1" previous space
  local spaces=( $'\t' $'\n' $'\v' $'\f' $'\r' ' '
    $'\xc2\x85' $'\xc2\xa0' $'\xe1\x9a\x80'
    $'\xe2\x80\x80' $'\xe2\x80\x81' $'\xe2\x80\x82' $'\xe2\x80\x83'
    $'\xe2\x80\x84' $'\xe2\x80\x85' $'\xe2\x80\x86' $'\xe2\x80\x87'
    $'\xe2\x80\x88' $'\xe2\x80\x89' $'\xe2\x80\x8a'
    $'\xe2\x80\xa8' $'\xe2\x80\xa9' $'\xe2\x80\xaf' $'\xe2\x81\x9f' $'\xe3\x80\x80' )
  while true; do
    previous="$path"
    for space in "${spaces[@]}"; do
      path="${path#"$space"}"
      path="${path%"$space"}"
    done
    [[ "$path" != "$previous" ]] || break
  done
  printf '%s' "$path"
}

resolve_backend_export_path() {
  if [[ "$1" == /* ]]; then
    printf '%s' "$1"
    return
  fi
  docker compose exec -T backend sh -ec 'printf "%s/%s" "$PWD" "$1"' sh "$1"
}

if ! CONFIGURED_KEYRING_PATH="$(docker compose exec -T backend sh -ec 'test -z "${PROFILE_ENCRYPTION_KEYRING_JSON:-}" && printf "%s" "${PROFILE_ENCRYPTION_KEYRING_FILE:-}"')"; then
  echo "ERROR: an active file-backed keyring is required. Preserve PROFILE_ENCRYPTION_KEYRING_JSON securely before exporting an inline-keyring database." >&2
  exit 1
fi
CONFIGURED_KEYRING_PATH="$(trim_backup_path "$CONFIGURED_KEYRING_PATH")"
if [[ -z "$CONFIGURED_KEYRING_PATH" ]]; then
  echo "ERROR: no active file-backed keyring is configured for export." >&2
  exit 1
fi
if ! docker compose exec -T backend sh -ec 'test -f "$1"' sh "$CONFIGURED_KEYRING_PATH"; then
  echo "ERROR: no file-backed keyring exists in the data volume. Preserve PROFILE_ENCRYPTION_KEYRING_JSON securely before exporting this database." >&2
  exit 1
fi
if ! CONFIGURED_KEYRING_PATH="$(resolve_backend_export_path "$CONFIGURED_KEYRING_PATH")"; then
  echo "ERROR: could not resolve the configured keyring path in the running backend." >&2
  exit 1
fi
if ! CONFIGURED_BACKUP_PATH="$(docker compose exec -T backend sh -ec 'printf "%s" "${BACKUP_PATH:-}"')"; then
  echo "ERROR: could not read BACKUP_PATH from the running backend." >&2
  exit 1
fi
CONFIGURED_BACKUP_PATH="$(trim_backup_path "$CONFIGURED_BACKUP_PATH")"
if [[ -z "$CONFIGURED_BACKUP_PATH" ]]; then
  echo "ERROR: automatic backup is disabled (BACKUP_PATH is empty)." >&2
  exit 1
fi
if ! CONFIGURED_BACKUP_PATH="$(resolve_backend_export_path "$CONFIGURED_BACKUP_PATH")"; then
  echo "ERROR: could not resolve the configured backup path in the running backend." >&2
  exit 1
fi
docker compose cp "backend:$CONFIGURED_BACKUP_PATH" "$OUT"
docker compose cp "backend:$CONFIGURED_KEYRING_PATH" "$KEYRING_OUT"
chmod 600 "$OUT"
chmod 600 "$KEYRING_OUT"

echo "Backup exported to: $OUT"
echo "Matching encryption keyring exported to: $KEYRING_OUT"

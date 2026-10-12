#!/usr/bin/env bash
# Run the actual export script with an exported fake Docker function.
set -euo pipefail
docker() {
  if [[ "$2" == exec ]]; then
    if [[ "$*" == *'$PWD'* ]]; then
      printf '/app/%s\n' "${!#}"
    elif [[ "$*" == *PROFILE_ENCRYPTION_KEYRING* ]]; then
      [[ -z "${SUBSHARE_BACKUP_TEST_INLINE:-}" ]] || return 1
      printf '%s\n' "${SUBSHARE_BACKUP_TEST_KEYRING_PATH-/app/data/keyring.json}"
    elif [[ "$*" == *BACKUP_PATH* ]]; then
      [[ "$SUBSHARE_BACKUP_TEST_FAILURE" != lookup ]] || return 1
      printf '%s\n' "$SUBSHARE_BACKUP_TEST_PATH"
    else
      [[ "$SUBSHARE_BACKUP_TEST_FAILURE" != keyring ]] || return 1
    fi
    return 0
  fi
  if [[ "$2" == cp ]]; then
    printf '%s\n' "$3" >> "$SUBSHARE_BACKUP_TEST_CAPTURE"
    if [[ "$SUBSHARE_BACKUP_TEST_FAILURE" == database-copy && "$3" != backend:/app/data/keyring.json ]]; then return 1; fi
    if [[ "$SUBSHARE_BACKUP_TEST_FAILURE" == keyring-copy && "$3" == backend:/app/data/keyring.json ]]; then return 1; fi
    printf 'disposable export fixture\n' > "$4"
    return 0
  fi
  printf 'Unexpected fake Docker command\n' >&2
  return 1
}
export -f docker
bash "$SUBSHARE_BACKUP_TEST_ROOT/scripts/backup.sh" "$SUBSHARE_BACKUP_TEST_DESTINATION"

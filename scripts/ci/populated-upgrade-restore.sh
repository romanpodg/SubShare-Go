#!/usr/bin/env bash
# Required CI assurance uses generated fixtures and two isolated named projects.
set -euo pipefail
[[ "${CI:-}" == true ]] || { echo 'This assurance script requires a disposable CI runner.' >&2; exit 1; }
[[ "${GITHUB_RUN_ID:-}" =~ ^[0-9]+$ ]] || { echo 'A numeric CI run identity is required.' >&2; exit 1; }
ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT_DIR"
FIXTURE_DIR="$ROOT_DIR/.cache/docker-upgrade-fixture"
UPGRADE_PROJECT="ss-assurance-${GITHUB_RUN_ID}-upgrade"
RESTORE_PROJECT="ss-assurance-${GITHUB_RUN_ID}-restore"
export PROFILE_ENCRYPTION_KEYRING_FILE='data/fixture keyring.json'
export PROFILE_ENCRYPTION_KEYRING_JSON=''
export BACKUP_PATH='data/custom backups/fixture.db'

cleanup() {
  docker compose -p "$UPGRADE_PROJECT" down -v --remove-orphans || true
  docker compose -p "$RESTORE_PROJECT" down -v --remove-orphans || true
}
trap cleanup EXIT

seed_project() {
  local project="$1" database="$2" keyring="$3"
  docker compose -p "$project" run --rm --no-deps --entrypoint sh backend -ec \
    'umask 077; cat > /app/data/app.db' < "$database"
  docker compose -p "$project" run --rm --no-deps --entrypoint sh backend -ec \
    'umask 077; cat > "/app/data/fixture keyring.json"' < "$keyring"
}

verify_delivery() {
  local output="$1"
  curl --fail --silent --show-error http://localhost/health > /dev/null
  curl --fail --silent --show-error http://localhost/sub/upgrade-fixture-token/subbody/plain > "$output"
  cmp "$FIXTURE_DIR/expected-body.txt" "$output"
}

export_pair() {
  local destination="$1"
  for attempt in $(seq 1 30); do
    if docker compose exec -T backend sh -ec 'test -s "/app/data/custom backups/fixture.db"'; then break; fi
    sleep 1
  done
  bash scripts/backup.sh "$destination"
}

verify_pair() {
  local directory="$1"
  local databases=("$directory"/app_*.db) keyrings=("$directory"/keyring_*.json)
  [[ "${#databases[@]}" == 1 && -f "${databases[0]}" ]] || { echo 'Expected one exported fixture database.' >&2; exit 1; }
  [[ "${#keyrings[@]}" == 1 && -f "${keyrings[0]}" ]] || { echo 'Expected one exported fixture keyring.' >&2; exit 1; }
  SUBSHARE_DOCKER_RESTORE_DB="${databases[0]}" SUBSHARE_DOCKER_RESTORE_KEYRING="${keyrings[0]}" \
    go test -tags=dockerfixture ./internal/storage -run '^TestVerifyDockerRestoredFixture$' -count=1
}

export COMPOSE_PROJECT_NAME="$UPGRADE_PROJECT"
seed_project "$UPGRADE_PROJECT" "$FIXTURE_DIR/app.db" "$FIXTURE_DIR/keyring.json"
docker compose up -d --wait --wait-timeout 180
verify_delivery "$FIXTURE_DIR/upgraded-body.txt"
export_pair "$FIXTURE_DIR/exported"
verify_pair "$FIXTURE_DIR/exported"
database=("$FIXTURE_DIR/exported"/app_*.db)
keyring=("$FIXTURE_DIR/exported"/keyring_*.json)
docker compose down -v --remove-orphans

export COMPOSE_PROJECT_NAME="$RESTORE_PROJECT"
seed_project "$RESTORE_PROJECT" "${database[0]}" "${keyring[0]}"
docker compose up -d --wait --wait-timeout 180
verify_delivery "$FIXTURE_DIR/restored-body.txt"
export_pair "$FIXTURE_DIR/restored-export"
verify_pair "$FIXTURE_DIR/restored-export"

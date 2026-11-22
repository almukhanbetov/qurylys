#!/usr/bin/env bash
# QURYLYS production deploy on the VPS. Installed as /opt/qurylys/bin/deploy.sh.
# See docs/deployment.md for setup, rollback and the threat model.
#
#   deploy.sh deploy <backend-ref> <frontend-ref>
#   deploy.sh rollback [--force-schema]
#   deploy.sh status
#
# A ref is the full, immutable reference produced by CI:
#   ghcr.io/almukhanbetov/qurylys-backend:<40-hex commit sha>@sha256:<64-hex digest>
#
# Order of a deploy (any failure stops it with a non-zero exit code):
#   lock -> validate refs and config -> pull + verify labels -> pg_dump backup
#   -> verify archive -> migrate up -> compose up (no down) -> readiness checks
#   -> record release.
# It never runs `compose down`, never removes volumes and never prunes images.

set -Eeuo pipefail
umask 077

readonly ROOT=/opt/qurylys
readonly COMPOSE_FILE="$ROOT/compose.prod.yml"
readonly ENV_DIR="$ROOT/env"
readonly STATE="$ROOT/state"
readonly RELEASES="$STATE/releases"
readonly BACKUPS="$ROOT/backups"
readonly LOCK_FILE="$STATE/deploy.lock"
readonly HISTORY="$STATE/history.log"

readonly BACKEND_REPO=ghcr.io/almukhanbetov/qurylys-backend
readonly FRONTEND_REPO=ghcr.io/almukhanbetov/qurylys-frontend

# Must match the loopback ports in compose.prod.yml and the Nginx upstreams.
readonly API_READY_URL=http://127.0.0.1:18080/api/v1/ready
readonly WEB_URL=http://127.0.0.1:13000/

readonly LOCK_WAIT=300          # seconds to wait for another deploy to finish
readonly PULL_TIMEOUT=600
readonly BACKUP_TIMEOUT=1800
readonly MIGRATE_TIMEOUT=900
readonly UP_WAIT=180
readonly CHECK_ATTEMPTS=24      # x CHECK_DELAY seconds
readonly CHECK_DELAY=5
readonly MIN_FREE_MB=2048       # free space required in $BACKUPS before pg_dump

log() { printf '%s %s\n' "$(date -u +%Y-%m-%dT%H:%M:%SZ)" "$*" >&2; }
die() { log "ERROR: $*"; exit 1; }
trap 'log "ERROR: command failed at line $LINENO: $BASH_COMMAND"' ERR

usage() {
  cat >&2 <<EOF
usage: $0 deploy <backend-ref> <frontend-ref>
       $0 rollback [--force-schema]
       $0 status
EOF
  exit 2
}

compose() {
  local env_file=$1; shift
  docker compose --project-directory "$ROOT" -f "$COMPOSE_FILE" --env-file "$env_file" "$@"
}

acquire_lock() {
  mkdir -p "$STATE" "$RELEASES"
  exec 9>"$LOCK_FILE"
  if ! flock -w "$LOCK_WAIT" 9; then
    die "another deploy holds $LOCK_FILE for more than ${LOCK_WAIT}s"
  fi
}

# validate_ref <ref> <repo> -> prints the commit sha embedded in the tag.
validate_ref() {
  local ref=$1 repo=$2 re
  re="^${repo//./\\.}:([0-9a-f]{40})@sha256:[0-9a-f]{64}$"
  [[ $ref =~ $re ]] || die "image reference not allowed: '$ref' (expected $repo:<sha>@sha256:<digest>)"
  printf '%s\n' "${BASH_REMATCH[1]}"
}

# Env files hold secrets: they must exist, belong to us, be private, and be filled in.
check_env_file() {
  local f=$1 mode owner
  [[ -f $f ]] || die "missing $f (copy from deploy/env/$(basename "$f").example)"
  mode=$(stat -c %a "$f"); owner=$(stat -c %u "$f")
  [[ $owner == "$(id -u)" ]] || die "$f must be owned by $(id -un)"
  [[ $mode == 600 || $mode == 400 ]] || die "$f must have mode 600 (has $mode)"
  if grep -Eq '<[A-Z_]+>' "$f"; then die "$f still contains <PLACEHOLDER> values"; fi
}

preflight() {
  command -v docker >/dev/null || die "docker is not installed"
  docker compose version >/dev/null || die "docker compose plugin is not installed"
  for c in curl flock pg_dump pg_restore psql sha256sum; do
    command -v "$c" >/dev/null || die "$c is not installed"
  done
  [[ -f $COMPOSE_FILE ]] || die "missing $COMPOSE_FILE"
  check_env_file "$ENV_DIR/api.env"
  check_env_file "$ENV_DIR/migrate.env"
  check_env_file "$ENV_DIR/backup.env"
}

# Load only PG* connection settings for pg_dump (the password comes from ~/.pgpass).
load_backup_env() {
  local key value
  while IFS='=' read -r key value; do
    case $key in
      PGHOST|PGPORT|PGDATABASE|PGUSER) export "$key=$value" ;;
      ''|\#*) ;;
      *) die "unexpected key '$key' in $ENV_DIR/backup.env" ;;
    esac
  done < "$ENV_DIR/backup.env"
  [[ -n ${PGDATABASE:-} && -n ${PGUSER:-} ]] || die "PGDATABASE and PGUSER are required in backup.env"
  export PGCONNECT_TIMEOUT=10
}

pull_and_verify() {
  local ref=$1 sha=$2 rev
  log "pull $ref"
  timeout "$PULL_TIMEOUT" docker pull --quiet "$ref" >/dev/null || die "pull failed: $ref"
  rev=$(docker image inspect --format '{{ index .Config.Labels "org.opencontainers.image.revision" }}' "$ref")
  [[ $rev == "$sha" ]] || die "$ref has revision label '$rev', expected $sha"
}

schema_label() {
  docker image inspect --format '{{ index .Config.Labels "kz.qurylys.schema-version" }}' "$1" 2>/dev/null || true
}

db_schema_version() {
  load_backup_env
  psql --no-password -XAtq -v ON_ERROR_STOP=1 \
    -c "SELECT COALESCE(max(version_id), 0) FROM goose_db_version WHERE is_applied" 2>/dev/null || echo unknown
}

# backup <tag> -> prints the path of a verified pg_dump archive.
backup() {
  local tag=$1 ts tmp final avail_mb server_major client_major
  mkdir -p "$BACKUPS"; chmod 700 "$BACKUPS"
  load_backup_env

  avail_mb=$(df -Pm "$BACKUPS" | awk 'NR==2 {print $4}')
  (( avail_mb >= MIN_FREE_MB )) || die "only ${avail_mb} MB free in $BACKUPS (need ${MIN_FREE_MB})"

  server_major=$(psql --no-password -XAtq -v ON_ERROR_STOP=1 -c 'SHOW server_version_num') \
    || die "cannot connect to PostgreSQL as $PGUSER (check pg_hba.conf and ~/.pgpass)"
  server_major=$(( server_major / 10000 ))
  client_major=$(pg_dump --version | grep -Eo '[0-9]+' | head -1)
  (( client_major >= server_major )) || die "pg_dump $client_major is older than server $server_major"

  ts=$(date -u +%Y%m%dT%H%M%SZ)
  final="$BACKUPS/${PGDATABASE}-${ts}-pre-${tag}.dump"
  tmp="$final.partial"
  log "backup $PGDATABASE -> $final"
  if ! timeout "$BACKUP_TIMEOUT" pg_dump --no-password --format=custom --file="$tmp" >&2; then
    rm -f "$tmp"; die "pg_dump failed; deploy stopped before migrations"
  fi
  [[ -s $tmp ]] || { rm -f "$tmp"; die "pg_dump produced an empty file"; }
  # Readability check only: the archive and its table of contents can be parsed.
  # A real restore test needs pg_restore into an isolated database (docs/deployment.md).
  if ! pg_restore --list "$tmp" >/dev/null; then
    rm -f "$tmp"; die "backup archive is not readable by pg_restore"
  fi
  chmod 600 "$tmp"
  mv "$tmp" "$final"
  sha256sum "$final" > "$final.sha256"
  chmod 600 "$final.sha256"
  printf '%s\n' "$final"
}

check_http() {
  local name=$1 url=$2 i code
  for (( i = 1; i <= CHECK_ATTEMPTS; i++ )); do
    code=$(curl -sS -o /dev/null -w '%{http_code}' --max-time 5 "$url" 2>/dev/null || true)
    if [[ $code == 200 ]]; then
      log "$name OK ($url)"
      return 0
    fi
    log "$name not ready yet (HTTP ${code:-none}), attempt $i/$CHECK_ATTEMPTS"
    sleep "$CHECK_DELAY"
  done
  return 1
}

# activate <release-env> -> starts api/web from that release and checks them.
activate() {
  local release=$1
  log "starting api and web from $(basename "$release")"
  if ! compose "$release" up -d --no-build --wait --wait-timeout "$UP_WAIT" api web; then
    log "containers did not become healthy within ${UP_WAIT}s"
    return 1
  fi
  check_http "api readiness" "$API_READY_URL" || return 1
  check_http "frontend" "$WEB_URL" || return 1
}

report_failure() {
  local release=$1
  compose "$release" ps >&2 || true
  compose "$release" logs --no-color --tail 80 api web >&2 || true
}

# schema_compatible <backend-ref>: the release may run against the current
# database only if its migrations include every applied version (down-migrations
# are never run automatically).
schema_compatible() {
  local want db
  want=$(schema_label "$1")
  db=$(db_schema_version)
  log "schema: release=${want:-unknown}, database=$db"
  [[ $want =~ ^[0-9]+$ && $db =~ ^[0-9]+$ ]] || { log "cannot compare schema versions"; return 1; }
  (( 10#$db <= 10#$want )) || { log "database schema ($db) is newer than the release ($want)"; return 1; }
}

record() { printf '%s %s\n' "$(date -u +%Y-%m-%dT%H:%M:%SZ)" "$*" >> "$HISTORY"; }

cmd_deploy() {
  [[ $# -eq 2 ]] || usage
  local backend_ref=$1 frontend_ref=$2 sha fsha release dump schema
  sha=$(validate_ref "$backend_ref" "$BACKEND_REPO")
  fsha=$(validate_ref "$frontend_ref" "$FRONTEND_REPO")
  [[ $sha == "$fsha" ]] || die "backend ($sha) and frontend ($fsha) are from different commits"

  acquire_lock
  preflight
  log "deploy ${sha}"

  pull_and_verify "$backend_ref" "$sha"
  pull_and_verify "$frontend_ref" "$sha"

  release="$RELEASES/$sha.env"
  printf 'BACKEND_IMAGE=%s\nFRONTEND_IMAGE=%s\n' "$backend_ref" "$frontend_ref" > "$release.tmp"
  mv "$release.tmp" "$release"

  dump=$(backup "${sha:0:12}")
  log "backup verified: $dump"

  log "migrate up"
  # `timeout` cannot call the shell function, so the compose command is spelled out.
  if ! timeout "$MIGRATE_TIMEOUT" docker compose --project-directory "$ROOT" -f "$COMPOSE_FILE" \
      --env-file "$release" --profile migrate run --rm --no-deps migrate; then
    record "FAILED migrate $sha backup=$dump"
    die "migration failed; application containers were not changed. Backup: $dump"
  fi
  schema=$(db_schema_version)

  if ! activate "$release"; then
    report_failure "$release"
    record "FAILED start $sha schema=$schema backup=$dump"
    # Bring the last healthy release back if its code knows the current schema.
    # The deploy still fails: a restored old release is not a successful deploy.
    if [[ -f $STATE/current.env ]]; then
      local last
      last=$(sed -n 's/^BACKEND_IMAGE=//p' "$STATE/current.env")
      if schema_compatible "$last" && activate "$STATE/current.env"; then
        record "RESTORED $last after failed start of $sha"
        die "new release $sha is not healthy; the previous release was restored (state/current.env unchanged)"
      fi
      die "new release $sha is not healthy and the previous release could not be restored automatically; see docs/deployment.md (rollback)"
    fi
    die "first release $sha is not healthy; nothing to restore. Fix the cause and deploy again"
  fi

  if [[ -f $STATE/current.env ]] && ! cmp -s "$STATE/current.env" "$release"; then
    cp "$STATE/current.env" "$STATE/previous.env"
  fi
  cp "$release" "$STATE/current.env"
  record "OK deploy $sha schema=$schema backup=$dump backend=$backend_ref frontend=$frontend_ref"
  log "deploy $sha finished: api and frontend are healthy"
}

# Rollback switches api/web back to the last known-good release. It never runs
# down-migrations: the database keeps its current schema. That is safe only if
# the older code works with the newer schema (additive migrations); the image
# label kz.qurylys.schema-version is compared with the database for that reason.
cmd_rollback() {
  local force=false target running_sha target_backend db
  case ${1:-} in
    '') ;;
    --force-schema) force=true ;;
    *) usage ;;
  esac
  acquire_lock
  preflight

  # After a failed deploy current.env still points at the last healthy release;
  # after a successful one, roll back to previous.env.
  running_sha=$(docker inspect --format '{{ index .Config.Labels "org.opencontainers.image.revision" }}' qurylys-api-1 2>/dev/null || true)
  if [[ -f $STATE/current.env ]] && ! grep -q ":${running_sha}@" "$STATE/current.env" 2>/dev/null; then
    target="$STATE/current.env"
  elif [[ -f $STATE/previous.env ]]; then
    target="$STATE/previous.env"
  else
    die "no earlier release recorded in $STATE; nothing to roll back to"
  fi

  target_backend=$(sed -n 's/^BACKEND_IMAGE=//p' "$target")
  validate_ref "$target_backend" "$BACKEND_REPO" >/dev/null
  validate_ref "$(sed -n 's/^FRONTEND_IMAGE=//p' "$target")" "$FRONTEND_REPO" >/dev/null
  docker image inspect "$target_backend" >/dev/null 2>&1 || docker pull --quiet "$target_backend" >/dev/null

  log "rollback target $(basename "$target")"
  if ! schema_compatible "$target_backend"; then
    [[ $force == true ]] \
      || die "rollback refused. Down-migrations are not automatic; confirm the old code works with the current schema, then rerun with --force-schema"
    log "WARNING: --force-schema given, starting the older release on the current schema"
  fi
  db=$(db_schema_version)

  if ! activate "$target"; then
    report_failure "$target"
    record "FAILED rollback to $(basename "$target")"
    die "rollback target is not healthy either; investigate manually"
  fi
  if [[ $target == "$STATE/previous.env" ]]; then
    cp "$STATE/current.env" "$STATE/rolled-back.env"
    cp "$STATE/previous.env" "$STATE/current.env"
  fi
  record "OK rollback to $(sed -n 's/^BACKEND_IMAGE=//p' "$target") db_schema=$db"
  log "rollback finished"
}

cmd_status() {
  local f
  for f in current previous; do
    if [[ -f $STATE/$f.env ]]; then
      printf '%s:\n' "$f"; sed 's/^/  /' "$STATE/$f.env"
    fi
  done
  if [[ -f $STATE/current.env ]]; then compose "$STATE/current.env" ps; fi
  if [[ -f $HISTORY ]]; then printf 'history:\n'; tail -n 10 "$HISTORY" | sed 's/^/  /'; fi
}

case ${1:-} in
  deploy) shift; cmd_deploy "$@" ;;
  rollback) shift; cmd_rollback "$@" ;;
  status) shift; [[ $# -eq 0 ]] || usage; cmd_status ;;
  *) usage ;;
esac

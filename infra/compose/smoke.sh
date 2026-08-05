#!/bin/sh
# smoke.sh — one isolated Compose smoke runner for the API, RustFS, and
# Playwright synthetics suites. Usage: smoke.sh {api|rustfs|synthetics|all}.
#
# Owns prerequisites, per-run isolation, temporary credentials, the Compose
# lifecycle, readiness, the selected suite, failure diagnostics, and cleanup.
# Boring to read and usable unchanged by developers and CI.
set -eu
umask 077

ROOT=$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)
. "$ROOT/infra/compose/smoke-lib.sh"

SDDS_SMOKE_SELECTOR=${1:-}
SDDS_SMOKE_PROJECT=
SDDS_SMOKE_TMP=
SDDS_SMOKE_COMPOSE_FILE=$ROOT/infra/compose/compose.yaml
SDDS_SMOKE_PREFLIGHT_COMPOSE_FILE=$ROOT/infra/compose/compose-preflight.yaml
SDDS_SMOKE_ARTIFACT_DIR=${SDDS_SMOKE_ARTIFACT_DIR:-}
root_access=
root_secret=
media_access=
media_secret=

phase_cleanup() {
  status=$?
  trap - EXIT INT TERM
  if [ "$status" -ne 0 ] && [ -n "$SDDS_SMOKE_PROJECT" ]; then
    smoke_compose ps >&2 || :
    smoke_collect_failure_logs || :
  fi
  if [ -n "$SDDS_SMOKE_PROJECT" ]; then
    if ! docker compose -f "$SDDS_SMOKE_COMPOSE_FILE" -p "$SDDS_SMOKE_PROJECT" down --rmi local --volumes --remove-orphans >/dev/null 2>&1; then
      printf '%s\n' "smoke: WARNING: compose down failed; containers or volumes may remain" >&2
    fi
  fi
  [ -z "$SDDS_SMOKE_TMP" ] || rm -rf "$SDDS_SMOKE_TMP"
  exit "$status"
}
trap phase_cleanup EXIT INT TERM

smoke_collect_failure_logs() {
  for svc in api embedding rustfs-init rustfs rustfs-permissions; do
    if ! smoke_compose ps --services --all 2>/dev/null | grep -qx "$svc"; then
      continue
    fi
    raw=$(smoke_compose logs --no-color --timestamps --tail=200 "$svc" 2>&1 || :)
    redacted=$(printf '%s\n' "$raw" | sed \
      -e "s/$root_access/[redacted]/g" \
      -e "s/$root_secret/[redacted]/g" \
      -e "s/$media_access/[redacted]/g" \
      -e "s/$media_secret/[redacted]/g") || continue
    printf '%s\n' "$redacted" >&2 || :
    if [ -n "$SDDS_SMOKE_ARTIFACT_DIR" ]; then
      { mkdir -p "$SDDS_SMOKE_ARTIFACT_DIR" && printf '%s\n' "$redacted" >"$SDDS_SMOKE_ARTIFACT_DIR/$svc.log"; } || :
    fi
  done
}

phase_prerequisites() {
  smoke_log_phase prerequisites
  command -v docker >/dev/null || smoke_die 'docker is required'
  docker compose version >/dev/null 2>&1 || smoke_die 'docker compose is required'
  command -v pnpm >/dev/null || smoke_die 'pnpm is required'
  command -v curl >/dev/null || smoke_die 'curl is required'
  command -v openssl >/dev/null || smoke_die 'openssl is required'
  command -v node >/dev/null || smoke_die 'node is required'
  case "$SDDS_SMOKE_SELECTOR" in
    api|rustfs|all) command -v go >/dev/null || smoke_die 'go is required' ;;
  esac
  [ -r "$SDDS_SMOKE_COMPOSE_FILE" ] || smoke_die "compose file not found: $SDDS_SMOKE_COMPOSE_FILE"
  [ -r "$SDDS_SMOKE_PREFLIGHT_COMPOSE_FILE" ] || smoke_die "preflight compose file not found: $SDDS_SMOKE_PREFLIGHT_COMPOSE_FILE"
}

phase_isolation() {
  smoke_log_phase isolation
  SDDS_SMOKE_PROJECT=sdds-smoke-$SDDS_SMOKE_SELECTOR-$(date +%s)-$$-$(od -An -N2 -tu2 </dev/urandom | tr -d ' ')
  SDDS_SMOKE_TMP=$(mktemp -d "${TMPDIR:-/tmp}/sdds-smoke.XXXXXX")
  export SDDS_SMOKE_PROJECT SDDS_SMOKE_TMP SDDS_SMOKE_COMPOSE_FILE SDDS_SMOKE_PREFLIGHT_COMPOSE_FILE
}

phase_credentials() {
  smoke_log_phase credentials
  openssl rand -hex 20 >"$SDDS_SMOKE_TMP/root-access"
  openssl rand -hex 20 >"$SDDS_SMOKE_TMP/root-secret"
  openssl rand -hex 20 >"$SDDS_SMOKE_TMP/media-access"
  openssl rand -hex 20 >"$SDDS_SMOKE_TMP/media-secret"
  chmod 0444 "$SDDS_SMOKE_TMP/root-access" "$SDDS_SMOKE_TMP/root-secret" "$SDDS_SMOKE_TMP/media-access" "$SDDS_SMOKE_TMP/media-secret"
  export SDDS_COMPOSE_RUSTFS_ROOT_ACCESS_KEY_FILE="$SDDS_SMOKE_TMP/root-access"
  export SDDS_COMPOSE_RUSTFS_ROOT_SECRET_KEY_FILE="$SDDS_SMOKE_TMP/root-secret"
  export SDDS_COMPOSE_SDDS_MEDIA_ACCESS_KEY_FILE="$SDDS_SMOKE_TMP/media-access"
  export SDDS_COMPOSE_SDDS_MEDIA_SECRET_KEY_FILE="$SDDS_SMOKE_TMP/media-secret"
  root_access=$(tr -d '\r\n' <"$SDDS_SMOKE_TMP/root-access")
  root_secret=$(tr -d '\r\n' <"$SDDS_SMOKE_TMP/root-secret")
  media_access=$(tr -d '\r\n' <"$SDDS_SMOKE_TMP/media-access")
  media_secret=$(tr -d '\r\n' <"$SDDS_SMOKE_TMP/media-secret")
}

allocate_web_port() {
  for _ in 1 2 3; do
    port=$(node -e 'const s=require("net").createServer();s.listen(0,"127.0.0.1",()=>{const p=s.address().port;s.close(()=>console.log(p))})')
    if node -e "const s=require('net').createServer();s.listen($port,'127.0.0.1',()=>{s.close(()=>process.exit(0))})" 2>/dev/null; then
      printf '%s\n' "$port"
      return 0
    fi
  done
  smoke_die "could not allocate a free web port after 3 attempts"
}

phase_port() {
  smoke_log_phase port
  export SDDS_HTTP_PORT=0
  export SDDS_MAILSINK_PORT=0
}

realize_web_port() {
  smoke_log_phase web-port
  SDDS_SYNTHETICS_WEB_PORT=$(allocate_web_port)
  export SDDS_SYNTHETICS_WEB_PORT
  export PLAYWRIGHT_BASE_URL="http://localhost:$SDDS_SYNTHETICS_WEB_PORT"
}

phase_build_start() {
  smoke_log_phase build-start
  docker compose --env-file /dev/null -f "$SDDS_SMOKE_PREFLIGHT_COMPOSE_FILE" -p "$SDDS_SMOKE_PROJECT" run --build --rm --no-deps --entrypoint /usr/local/bin/validate-compose-secrets rustfs-init >/dev/null
  case "$SDDS_SMOKE_SELECTOR" in
    api|synthetics|all)
      export SDDS_AUTH_SIGNUP_REQUESTS_PER_MINUTE=60
      export SDDS_AUTH_LOGIN_REQUESTS_PER_MINUTE=60
      export SDDS_MAIL_MODE=enabled
      export SDDS_MAIL_API_TOKEN=smoke-token
      export SDDS_MAIL_FROM_ADDRESS=smoke@sdds.test
      export SDDS_MAIL_API_URL=http://mail-sink:8090/emails
      export SDDS_MAIL_TIMEOUT_MS=2000
      export SDDS_APP_BASE_URL=http://localhost:8081
      ;;
  esac
  smoke_compose up --build -d api mail-sink >/dev/null
}

phase_readiness() {
  smoke_log_phase readiness
  smoke_wait_for_api_readiness
  export SDDS_SYNTHETICS_API_BASE_URL="$SDDS_API_BASE_URL"
  smoke_mailsink_published=$(smoke_compose port mail-sink 8090 2>/dev/null || :)
  smoke_mailsink_port=${smoke_mailsink_published##*:}
  if [ -z "$smoke_mailsink_port" ]; then
    smoke_die "could not resolve the mail-sink published port"
  fi
  export SDDS_MAILSINK_URL="http://127.0.0.1:$smoke_mailsink_port"
  export SDDS_SYNTHETICS_MAILSINK_URL="$SDDS_MAILSINK_URL"
}

run_suite() {
  smoke_log_phase tests
  case "$1" in
    api) pnpm test:api:integration ;;
    rustfs) pnpm test:rustfs ;;
    synthetics) pnpm test:synthetics ;;
    *) smoke_die "unknown suite: $1" ;;
  esac
}

smoke_reset_stack() {
  if ! smoke_compose down --volumes --remove-orphans >/dev/null 2>&1; then
    smoke_die "compose down failed during stack reset; containers or volumes may contaminate the next suite"
  fi
  phase_build_start
  phase_readiness
}

phase_tests() {
  case "$SDDS_SMOKE_SELECTOR" in
    api|rustfs)
      run_suite "$SDDS_SMOKE_SELECTOR"
      ;;
    synthetics)
      realize_web_port
      run_suite "$SDDS_SMOKE_SELECTOR"
      ;;
    all)
      for suite in rustfs api synthetics; do
        if [ "$suite" = "synthetics" ]; then
          realize_web_port
        fi
        run_suite "$suite"
        [ "$suite" = "$SDDS_SMOKE_LAST" ] && break
        smoke_reset_stack
      done
      ;;
  esac
}

case "$SDDS_SMOKE_SELECTOR" in
  api|rustfs|synthetics)
    SDDS_SMOKE_LAST=$SDDS_SMOKE_SELECTOR
    ;;
  all)
    SDDS_SMOKE_LAST=synthetics
    ;;
  *)
    printf 'usage: smoke.sh {api|rustfs|synthetics|all}\n' >&2
    exit 2
    ;;
esac

phase_prerequisites
phase_isolation
phase_credentials
phase_port
phase_build_start
phase_readiness
phase_tests

printf '%s\n' "smoke: $SDDS_SMOKE_SELECTOR verified"

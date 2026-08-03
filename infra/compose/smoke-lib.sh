# Shared helpers for the Compose smoke runner (smoke.sh) and the rustfs suite
# (rustfs-suite.sh). This file is sourced, never executed.
#
# The caller MUST export SDDS_SMOKE_COMPOSE_FILE and SDDS_SMOKE_PROJECT before
# using smoke_compose or smoke_wait_for_api_readiness.

smoke_log_phase() {
  printf 'smoke: phase %s\n' "$1" >&2
}

smoke_compose() {
  docker compose -f "$SDDS_SMOKE_COMPOSE_FILE" -p "$SDDS_SMOKE_PROJECT" "$@"
}

smoke_die() {
  printf 'smoke: %s\n' "$1" >&2
  exit 1
}

# smoke_wait_for_api_readiness waits for rustfs-init to exit 0, discovers the
# OS-assigned published API port, probes /readyz, and on success exports
# SDDS_API_BASE_URL. Bounded to 120 attempts of 1s; a timeout is a hard failure.
smoke_wait_for_api_readiness() {
  API_URL=
  smoke_readiness_attempt=0
  while [ "$smoke_readiness_attempt" -lt 120 ]; do
    smoke_readiness_init_id=$(smoke_compose ps -aq rustfs-init 2>/dev/null || :)
    if [ -n "$smoke_readiness_init_id" ]; then
      smoke_readiness_init_state=$(docker inspect --format '{{.State.Status}} {{.State.ExitCode}}' "$smoke_readiness_init_id" 2>/dev/null || :)
      case "$smoke_readiness_init_state" in
        'exited 0') ;;
        'exited '*) smoke_die 'rustfs-init failed';;
      esac
    fi
    smoke_readiness_published=$(smoke_compose port api 8080 2>/dev/null || :)
    smoke_readiness_port=${smoke_readiness_published##*:}
    case "$smoke_readiness_port" in
      ''|*[!0-9]*) ;;
      *)
        API_URL=http://127.0.0.1:$smoke_readiness_port
        if curl --silent --show-error --fail --max-time 2 "$API_URL/readyz" >/dev/null 2>&1; then
          export SDDS_API_BASE_URL="$API_URL"
          return
        fi
        ;;
    esac
    smoke_readiness_attempt=$((smoke_readiness_attempt + 1))
    sleep 1
  done
  smoke_die 'api did not become ready'
}

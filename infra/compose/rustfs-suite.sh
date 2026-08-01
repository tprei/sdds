#!/bin/sh
# rustfs-suite.sh — the RustFS object-store assertions, run against a
# smoke-provided Compose stack. This is the suite body behind `pnpm smoke rustfs`;
# it does NOT own the lifecycle (project, credentials, readiness, cleanup).
set -eu
umask 077
ROOT=$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)
. "$ROOT/infra/compose/smoke-lib.sh"

for required in SDDS_SMOKE_PROJECT SDDS_SMOKE_COMPOSE_FILE SDDS_SMOKE_PREFLIGHT_COMPOSE_FILE SDDS_SMOKE_TMP \
  SDDS_API_BASE_URL SDDS_COMPOSE_RUSTFS_ROOT_ACCESS_KEY_FILE SDDS_COMPOSE_RUSTFS_ROOT_SECRET_KEY_FILE \
  SDDS_COMPOSE_SDDS_MEDIA_ACCESS_KEY_FILE SDDS_COMPOSE_SDDS_MEDIA_SECRET_KEY_FILE; do
  eval "required_value=\${$required:-}"
  [ -n "$required_value" ] || { printf 'rustfs-suite: %s is not set; run this suite through `pnpm smoke rustfs`\n' "$required" >&2; exit 1; }
done

COMPOSE_FILE=$SDDS_SMOKE_COMPOSE_FILE
PREFLIGHT_COMPOSE_FILE=$SDDS_SMOKE_PREFLIGHT_COMPOSE_FILE
PROJECT=$SDDS_SMOKE_PROJECT
TMP=$SDDS_SMOKE_TMP
API_URL=$SDDS_API_BASE_URL
root_access=$(tr -d '\r\n' <"$SDDS_COMPOSE_RUSTFS_ROOT_ACCESS_KEY_FILE")
root_secret=$(tr -d '\r\n' <"$SDDS_COMPOSE_RUSTFS_ROOT_SECRET_KEY_FILE")
media_access=$(tr -d '\r\n' <"$SDDS_COMPOSE_SDDS_MEDIA_ACCESS_KEY_FILE")
media_secret=$(tr -d '\r\n' <"$SDDS_COMPOSE_SDDS_MEDIA_SECRET_KEY_FILE")
die() { smoke_die "$1"; }
compose() { smoke_compose "$@"; }
wait_for_api_readiness() { smoke_wait_for_api_readiness; }
sha256_file() { shasum -a 256 "$1" | awk '{print $1}'; }

verify_preflight_runs_as_service_user() {
  output=$(docker compose --env-file /dev/null -f "$PREFLIGHT_COMPOSE_FILE" -p "$PROJECT" run --build --rm --no-deps --entrypoint id rustfs-init 2>&1) || { printf '%s\n' "$output" >&2; die 'preflight identity check failed'; }
  case "$output" in *'uid=10001 gid=10001'*) ;; *) printf '%s\n' "$output" >&2; die 'preflight did not run as the service user';; esac
}
run_preflight_without_secret() {
  (
    unset "$1"
    docker compose --env-file /dev/null -f "$PREFLIGHT_COMPOSE_FILE" -p "$PROJECT" run --build --rm --no-deps --entrypoint /usr/local/bin/validate-compose-secrets rustfs-init
  )
}
run_make_preflight_without_secret() {
  (
    unset "$1"
    make -C "$ROOT" "PREFLIGHT_COMPOSE=docker compose --env-file /dev/null -f $PREFLIGHT_COMPOSE_FILE -p $PROJECT" COMPOSE=false compose-up
  )
}
verify_compose_start_requires_secret_paths() {
  for required_secret_variable in SDDS_COMPOSE_RUSTFS_ROOT_ACCESS_KEY_FILE SDDS_COMPOSE_RUSTFS_ROOT_SECRET_KEY_FILE SDDS_COMPOSE_SDDS_MEDIA_ACCESS_KEY_FILE SDDS_COMPOSE_SDDS_MEDIA_SECRET_KEY_FILE; do
    output=$(run_preflight_without_secret "$required_secret_variable" 2>&1) && die "$required_secret_variable is optional"
    case "$output" in *"$required_secret_variable"*) ;; *) printf '%s\n' "$output" >&2; die "$required_secret_variable diagnostic drift";; esac
  done
}
verify_preflight_ignores_orphans() {
  orphan_check_variable=SDDS_COMPOSE_RUSTFS_ROOT_ACCESS_KEY_FILE
  output=$(run_make_preflight_without_secret "$orphan_check_variable" 2>&1) && die "$orphan_check_variable is optional"
  case "$output" in
    *'orphan containers'*) die "$orphan_check_variable emitted an orphan warning";;
    *"$orphan_check_variable"*) ;;
    *) printf '%s\n' "$output" >&2; die "$orphan_check_variable diagnostic drift";;
  esac
}
verify_preflight_rejects_invalid_media_secret() {
  invalid_secret_file=$1
  invalid_secret_label=$2
  output=$(SDDS_COMPOSE_SDDS_MEDIA_SECRET_KEY_FILE="$invalid_secret_file" docker compose --env-file /dev/null -f "$PREFLIGHT_COMPOSE_FILE" -p "$PROJECT" run --build --rm --no-deps --entrypoint /usr/local/bin/validate-compose-secrets rustfs-init 2>&1) && die "$invalid_secret_label secret was accepted"
  case "$output" in *'SDDS_COMPOSE_SDDS_MEDIA_SECRET_KEY_FILE contains an invalid character'*) ;; *) printf '%s\n' "$output" >&2; die "$invalid_secret_label secret diagnostic drift";; esac
}
verify_preflight_rejects_whitespace_secret() {
  whitespace_secret=$TMP/media-secret-whitespace
  printf 'invalid secret\n' >"$whitespace_secret"
  chmod 0444 "$whitespace_secret"
  verify_preflight_rejects_invalid_media_secret "$whitespace_secret" whitespace
}
verify_preflight_rejects_embedded_newline_secret() {
  embedded_newline_secret=$TMP/media-secret-embedded-newline
  printf 'first\nsecond\n' >"$embedded_newline_secret"
  chmod 0444 "$embedded_newline_secret"
  verify_preflight_rejects_invalid_media_secret "$embedded_newline_secret" embedded-newline
}
verify_preflight_rejects_nul_secret() {
  nul_secret=$TMP/media-secret-nul
  printf 'first\000second\n' >"$nul_secret"
  chmod 0444 "$nul_secret"
  verify_preflight_rejects_invalid_media_secret "$nul_secret" nul
}
verify_secret_reader_cleans_snapshot_on_signal() {
  snapshot_copy_bin=$TMP/snapshot-copy-bin
  mkdir -p "$snapshot_copy_bin"
  printf '%s\n' '#!/bin/sh' 'touch /tmp/sdds-test/snapshot-copy-started' 'while [ ! -e /tmp/sdds-test/snapshot-copy-release ]; do sleep 1; done' 'exec /bin/cat "$@"' >"$snapshot_copy_bin/cat"
  chmod 0555 "$snapshot_copy_bin/cat"
  output=$(compose run --rm --no-deps --volume "$TMP:/tmp/sdds-test" --entrypoint /bin/sh rustfs-init -ec '
    PATH=/tmp/sdds-test/snapshot-copy-bin:$PATH
    export PATH
    (
      die() { exit 1; }
      . /usr/local/lib/rustfs-init/secret-file.sh
      read_secret /tmp/sdds-test/media-secret snapshot-cleanup >/dev/null
    ) &
    reader_pid=$!
    while [ ! -e /tmp/sdds-test/snapshot-copy-started ]; do sleep 1; done
    kill -TERM "$reader_pid"
    touch /tmp/sdds-test/snapshot-copy-release
    if wait "$reader_pid"; then exit 1; fi
    set -- /tmp/rustfs-secret.*
    [ "$1" = "/tmp/rustfs-secret.*" ]
  ' 2>&1) || { printf '%s\n' "$output" >&2; die 'credential snapshot survived interruption'; }
}
aws_with_credentials() {
  aws_credentials_access_key=$1
  aws_credentials_secret_key=$2
  shift 2
  compose run --rm --no-deps --volume "$TMP:/tmp/sdds-test" -e AWS_ACCESS_KEY_ID="$aws_credentials_access_key" -e AWS_SECRET_ACCESS_KEY="$aws_credentials_secret_key" --entrypoint /usr/local/bin/aws rustfs-init --endpoint-url http://rustfs:9000 "$@"
}
root_aws() { aws_with_credentials "$root_access" "$root_secret" "$@"; }
api_aws() { aws_with_credentials "$media_access" "$media_secret" "$@"; }
run_root_mc_script() {
  root_mc_script=$1
  shift
  compose run --rm --no-deps -e ROOT_ACCESS="$root_access" -e ROOT_SECRET="$root_secret" -e API_ACCESS="$media_access" --entrypoint /bin/sh rustfs-init -ec "export MC_CONFIG_DIR=\$(mktemp -d); mc alias set root http://rustfs:9000 \"\$ROOT_ACCESS\" \"\$ROOT_SECRET\" --api S3v4 >/dev/null; $root_mc_script" sh "$@"
}
root_mc() {
  root_mc_command=$1
  shift
  run_root_mc_script 'mc "$@"' "$root_mc_command" "$@"
}
recreate_clean_stack() {
  compose down --volumes --remove-orphans >/dev/null 2>&1 || :
  compose up -d >/dev/null
  wait_for_api_readiness
}
assert_bootstrap_rejects_drift() {
  assert_drift_label=$1
  assert_drift_expected_diagnostic=$2
  assert_drift_function=$3
  "$assert_drift_function"
  output=$(compose run --rm --no-deps rustfs-init 2>&1) && die "$assert_drift_label accepted drift"
  case "$output" in *"$assert_drift_expected_diagnostic"*) ;; *) printf '%s\n' "$output" >&2; die "$assert_drift_label diagnostic drift";; esac
  recreate_clean_stack
}
introduce_public_access_block_drift() {
  root_aws s3api put-public-access-block --bucket sdds-media --public-access-block-configuration BlockPublicAcls=false,IgnorePublicAcls=true,BlockPublicPolicy=true,RestrictPublicBuckets=true >/dev/null
}
enable_bucket_versioning() {
  root_aws s3api put-bucket-versioning --bucket sdds-media --versioning-configuration Status=Enabled >/dev/null
}
add_bucket_lifecycle_rule() {
  root_aws s3api put-bucket-lifecycle-configuration --bucket sdds-media --lifecycle-configuration '{"Rules":[{"ID":"drift-v1","Status":"Enabled","Filter":{"Prefix":"drift/"},"Expiration":{"Days":1}}]}' >/dev/null
}
add_bucket_policy() {
  root_aws s3api put-bucket-policy --bucket sdds-media --policy '{"Version":"2012-10-17","Statement":[{"Effect":"Deny","Principal":"*","Action":"s3:GetObject","Resource":"arn:aws:s3:::sdds-media/*"}]}' >/dev/null
}
replace_api_policy() {
  run_root_mc_script 'd=$(mktemp -d); printf "%s\n" "{\"Version\":\"2012-10-17\",\"Statement\":[{\"Effect\":\"Allow\",\"Action\":[\"s3:GetObject\"],\"Resource\":[\"arn:aws:s3:::sdds-media/system/readiness\"]}]}" >"$d/p"; mc admin policy detach root sdds-media-api --user "$API_ACCESS"; mc admin policy remove root sdds-media-api; mc admin policy create root sdds-media-api "$d/p"'
}
replace_readiness_sentinel() {
  root_aws s3api put-object --bucket sdds-media --key system/readiness --body /etc/rustfs-init/api-policy.json >/dev/null
}
enable_anonymous_download() {
  root_aws s3api put-public-access-block --bucket sdds-media --public-access-block-configuration BlockPublicAcls=false,IgnorePublicAcls=false,BlockPublicPolicy=false,RestrictPublicBuckets=false >/dev/null
  root_mc anonymous set download root/sdds-media >/dev/null
}
detach_api_policy() {
  root_mc admin policy detach root sdds-media-api --user "$media_access" >/dev/null
}
verify_bootstrap_drift_recovery() {
  assert_bootstrap_rejects_drift pab-v1 'public-access-block drift' introduce_public_access_block_drift
  assert_bootstrap_rejects_drift versioning-v1 'bucket versioning is enabled' enable_bucket_versioning
  assert_bootstrap_rejects_drift lifecycle-v1 'bucket lifecycle is configured' add_bucket_lifecycle_rule
  assert_bootstrap_rejects_drift policy-v1 'anonymous bucket policy is configured' add_bucket_policy
  assert_bootstrap_rejects_drift api-policy-v1 'API policy drift' replace_api_policy
  assert_bootstrap_rejects_drift sentinel-v1 'readiness sentinel metadata drift' replace_readiness_sentinel
  assert_bootstrap_rejects_drift anonymous-v1 'anonymous bucket policy is configured' enable_anonymous_download
  assert_bootstrap_rejects_drift attachment-v1 'API user attachment drift' detach_api_policy
}
verify_bootstrap_idempotency() {
  compose run --rm --no-deps rustfs-init >/dev/null
}
verify_object_payload() {
  object_key=$1
  object_expected_source=$2
  object_container_destination=$3
  object_downloaded_path=$4
  object_expected_sha256=$5
  api_aws s3api get-object --bucket sdds-media --key "$object_key" "$object_container_destination" >/dev/null
  cmp -s "$object_expected_source" "$object_downloaded_path" || die "$object_key payload drift"
  [ "$(sha256_file "$object_downloaded_path")" = "$object_expected_sha256" ] || die "$object_key hash drift"
}
prepare_sentinel_fixture() {
  printf 'sdds-media-ready-v1\n' >"$TMP/sentinel"
  sentinel_hash=$(sha256_file "$TMP/sentinel")
}
verify_sentinel_payload() {
  sentinel_phase=$1
  case "$sentinel_phase" in
    before)
      sentinel_container_destination=/tmp/sdds-test/sentinel-before
      sentinel_downloaded_path="$TMP/sentinel-before"
      ;;
    after)
      sentinel_container_destination=/tmp/sdds-test/sentinel-after
      sentinel_downloaded_path="$TMP/sentinel-after"
      ;;
    *) die "unknown sentinel phase: $sentinel_phase";;
  esac
  verify_object_payload system/readiness "$TMP/sentinel" "$sentinel_container_destination" "$sentinel_downloaded_path" "$sentinel_hash"
}
verify_api_restart_outage_recovery() {
  export SDDS_API_BASE_URL="$API_URL" SDDS_RUSTFS_COMPOSE_FILE="$COMPOSE_FILE" SDDS_RUSTFS_COMPOSE_PROJECT="$PROJECT"
  pnpm test:api:runtime-boundaries
}
verify_migrate_without_media_dependencies() {
  migrate_project="${PROJECT}-migrate-fresh"
  docker compose --env-file /dev/null -f "$COMPOSE_FILE" -p "$migrate_project" down --volumes --remove-orphans >/dev/null 2>&1 || :
  (
    unset SDDS_COMPOSE_RUSTFS_ROOT_ACCESS_KEY_FILE SDDS_COMPOSE_RUSTFS_ROOT_SECRET_KEY_FILE SDDS_COMPOSE_SDDS_MEDIA_ACCESS_KEY_FILE SDDS_COMPOSE_SDDS_MEDIA_SECRET_KEY_FILE
    docker compose --env-file /dev/null -f "$COMPOSE_FILE" -p "$migrate_project" run --build --rm --no-deps api migrate >/dev/null
  )
  if ! docker compose --env-file /dev/null -f "$COMPOSE_FILE" -p "$migrate_project" down --volumes --remove-orphans >/dev/null 2>&1; then
    die "migrate-fresh project cleanup failed; volumes may remain"
  fi
}
run_rustfs_integration() {
  verify_preflight_runs_as_service_user
  verify_preflight_rejects_whitespace_secret
  verify_preflight_rejects_embedded_newline_secret
  verify_preflight_rejects_nul_secret
  verify_secret_reader_cleans_snapshot_on_signal
  verify_compose_start_requires_secret_paths
  verify_preflight_ignores_orphans
  verify_bootstrap_idempotency
  verify_bootstrap_drift_recovery
  prepare_sentinel_fixture
  verify_sentinel_payload before
  verify_api_restart_outage_recovery
  verify_sentinel_payload after
  verify_bootstrap_idempotency
  verify_migrate_without_media_dependencies
  printf '%s\n' 'rustfs integration verified'
}
run_rustfs_integration

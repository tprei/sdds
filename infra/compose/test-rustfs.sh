#!/bin/sh
set -eu
umask 077
ROOT=$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)
COMPOSE_FILE=$ROOT/infra/compose/compose.yaml
PROJECT=sdds-rustfs-$(date +%s)-$$
TMP=
API_URL=
cleanup() {
  status=$?
  trap - EXIT INT TERM
  if [ "$status" -ne 0 ] && [ -n "$PROJECT" ]; then
    compose ps >&2 || :
    compose logs --no-color --tail=80 api rustfs rustfs-init rustfs-permissions >&2 || :
  fi
  [ -z "$PROJECT" ] || docker compose -f "$COMPOSE_FILE" -p "$PROJECT" down --rmi local --volumes --remove-orphans >/dev/null 2>&1 || :
  [ -z "$TMP" ] || rm -rf "$TMP"
  exit "$status"
}
trap cleanup EXIT INT TERM
die() { printf '%s\n' "$1" >&2; exit 1; }
compose() { docker compose -f "$COMPOSE_FILE" -p "$PROJECT" "$@"; }
sha256_file() { shasum -a 256 "$1" | awk '{print $1}'; }
create_test_credentials() {
  create_credentials_work_dir=$1
  create_credentials_project=$2
  printf 'rustfs-root-access-%s\n' "$create_credentials_project" >"$create_credentials_work_dir/root-access"
  printf 'rustfs-root-secret-%s\n' "$create_credentials_project" >"$create_credentials_work_dir/root-secret"
  printf 'sdds-media-access-%s\n' "$create_credentials_project" >"$create_credentials_work_dir/media-access"
  printf 'sdds-media-secret-%s\n' "$create_credentials_project" >"$create_credentials_work_dir/media-secret"
  chmod 0444 "$create_credentials_work_dir"/root-access "$create_credentials_work_dir"/root-secret "$create_credentials_work_dir"/media-access "$create_credentials_work_dir"/media-secret
  root_access=$(tr -d '\r\n' <"$create_credentials_work_dir/root-access")
  root_secret=$(tr -d '\r\n' <"$create_credentials_work_dir/root-secret")
  media_access=$(tr -d '\r\n' <"$create_credentials_work_dir/media-access")
  media_secret=$(tr -d '\r\n' <"$create_credentials_work_dir/media-secret")
  export SDDS_COMPOSE_RUSTFS_ROOT_ACCESS_KEY_FILE=$create_credentials_work_dir/root-access SDDS_COMPOSE_RUSTFS_ROOT_SECRET_KEY_FILE=$create_credentials_work_dir/root-secret
  export SDDS_COMPOSE_SDDS_MEDIA_ACCESS_KEY_FILE=$create_credentials_work_dir/media-access SDDS_COMPOSE_SDDS_MEDIA_SECRET_KEY_FILE=$create_credentials_work_dir/media-secret SDDS_HTTP_PORT=0
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
wait_for_api_readiness() {
  API_URL=
  readiness_attempt=0
  while [ "$readiness_attempt" -lt 120 ]; do
    readiness_init_id=$(compose ps -aq rustfs-init 2>/dev/null || :)
    if [ -n "$readiness_init_id" ]; then
      readiness_init_state=$(docker inspect --format '{{.State.Status}} {{.State.ExitCode}}' "$readiness_init_id" 2>/dev/null || :)
      case "$readiness_init_state" in
        'exited 0') ;;
        'exited '*) die 'rustfs-init failed';;
      esac
    fi
    readiness_published=$(compose port api 8080 2>/dev/null || :)
    readiness_port=${readiness_published##*:}
    case "$readiness_port" in
      ''|*[!0-9]*) ;;
      *)
        API_URL=http://127.0.0.1:$readiness_port
        curl --silent --show-error --fail --max-time 2 "$API_URL/readyz" >/dev/null 2>&1 && return
        ;;
    esac
    readiness_attempt=$((readiness_attempt + 1))
    sleep 1
  done
  die 'api did not become ready'
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
start_compose_runtime() {
  compose up --build -d >/dev/null
  wait_for_api_readiness
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
  pnpm test:rustfs:api-runtime-boundaries
}
verify_migrate_without_media_dependencies() {
  compose run --build --rm --no-deps api migrate >/dev/null
}
run_rustfs_integration() {
  TMP=$(mktemp -d "${TMPDIR:-/tmp}/sdds-rustfs.XXXXXX")
  create_test_credentials "$TMP" "$PROJECT"
  start_compose_runtime
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

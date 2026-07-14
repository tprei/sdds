#!/bin/sh
set -eu
umask 077

ROOT=$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd); COMPOSE_FILE=$ROOT/infra/compose/compose.yaml
PROJECT=sdds-rustfs-$(date +%s)-$$; TMP=; API_URL=
cleanup() { status=$?; trap - EXIT INT TERM; [ -z "$PROJECT" ] || docker compose -f "$COMPOSE_FILE" -p "$PROJECT" down --rmi local --volumes --remove-orphans >/dev/null 2>&1 || :; [ -z "$TMP" ] || rm -rf "$TMP"; exit "$status"; }
trap cleanup EXIT INT TERM
die() { printf '%s\n' "$1" >&2; exit 1; }
compose() { docker compose -f "$COMPOSE_FILE" -p "$PROJECT" "$@"; }
sha256() { shasum -a 256 "$1" | awk '{print $1}'; }

TMP=$(mktemp -d "${TMPDIR:-/tmp}/sdds-rustfs.XXXXXX")
printf 'rustfs-root-access-%s\n' "$PROJECT" >"$TMP/root-access"; printf 'rustfs-root-secret-%s\n' "$PROJECT" >"$TMP/root-secret"
printf 'sdds-media-access-%s\n' "$PROJECT" >"$TMP/media-access"; printf 'sdds-media-secret-%s\n' "$PROJECT" >"$TMP/media-secret"
root_access=$(tr -d '\r\n' <"$TMP/root-access"); root_secret=$(tr -d '\r\n' <"$TMP/root-secret")
media_access=$(tr -d '\r\n' <"$TMP/media-access"); media_secret=$(tr -d '\r\n' <"$TMP/media-secret")
export SDDS_COMPOSE_RUSTFS_ROOT_ACCESS_KEY_FILE=$TMP/root-access SDDS_COMPOSE_RUSTFS_ROOT_SECRET_KEY_FILE=$TMP/root-secret
export SDDS_COMPOSE_SDDS_MEDIA_ACCESS_KEY_FILE=$TMP/media-access SDDS_COMPOSE_SDDS_MEDIA_SECRET_KEY_FILE=$TMP/media-secret SDDS_HTTP_PORT=0

aws_cmd() { access=$1; secret=$2; shift 2; compose run --rm --no-deps --volume "$TMP:/tmp/sdds-test" -e AWS_ACCESS_KEY_ID="$access" -e AWS_SECRET_ACCESS_KEY="$secret" --entrypoint /usr/local/bin/aws rustfs-init --endpoint-url http://rustfs:9000 "$@"; }
root_aws() { aws_cmd "$root_access" "$root_secret" "$@"; }
media_aws() { aws_cmd "$media_access" "$media_secret" "$@"; }
mc_root_script() { script=$1; shift; compose run --rm --no-deps -e ROOT_ACCESS="$root_access" -e ROOT_SECRET="$root_secret" -e API_ACCESS="$media_access" --entrypoint /bin/sh rustfs-init -ec "export MC_CONFIG_DIR=\$(mktemp -d); mc alias set root http://rustfs:9000 \"\$ROOT_ACCESS\" \"\$ROOT_SECRET\" --api S3v4 >/dev/null; $script" sh "$@"; }
mc_root() { command=$1; shift; mc_root_script 'mc "$@"' "$command" "$@"; }

wait_for_api() {
  API_URL=; i=0
  while [ "$i" -lt 120 ]; do
    init_id=$(compose ps -aq rustfs-init 2>/dev/null || :)
    if [ -n "$init_id" ]; then
      init_state=$(docker inspect --format '{{.State.Status}} {{.State.ExitCode}}' "$init_id" 2>/dev/null || :)
      case "$init_state" in 'exited 0') ;; 'exited '*) die 'rustfs-init failed';; esac
    fi
    published=$(compose port api 8080 2>/dev/null || :); port=${published##*:}
    case "$port" in ''|*[!0-9]*) ;; *) API_URL=http://127.0.0.1:$port; curl --silent --show-error --fail --max-time 2 "$API_URL/readyz" >/dev/null 2>&1 && return;; esac
    i=$((i + 1)); sleep 1
  done
  die 'api did not become ready'
}
reset_stack() { compose down --volumes --remove-orphans >/dev/null 2>&1 || :; compose up -d >/dev/null; wait_for_api; }
expect_drift() {
  label=$1; needle=$2; shift 2; "$@"
  output=$(compose run --rm --no-deps rustfs-init 2>&1) && die "$label accepted drift"
  case "$output" in *"$needle"*) ;; *) printf '%s\n' "$output" >&2; die "$label diagnostic drift";; esac
  reset_stack
}

drift_pab() { root_aws s3api put-public-access-block --bucket sdds-media --public-access-block-configuration BlockPublicAcls=false,IgnorePublicAcls=true,BlockPublicPolicy=true,RestrictPublicBuckets=true >/dev/null; }
drift_versioning() { root_aws s3api put-bucket-versioning --bucket sdds-media --versioning-configuration Status=Enabled >/dev/null; }
drift_lifecycle() { root_aws s3api put-bucket-lifecycle-configuration --bucket sdds-media --lifecycle-configuration '{"Rules":[{"ID":"drift-v1","Status":"Enabled","Filter":{"Prefix":"drift/"},"Expiration":{"Days":1}}]}' >/dev/null; }
drift_bucket_policy() { root_aws s3api put-bucket-policy --bucket sdds-media --policy '{"Version":"2012-10-17","Statement":[{"Effect":"Deny","Principal":"*","Action":"s3:GetObject","Resource":"arn:aws:s3:::sdds-media/*"}]}' >/dev/null; }
drift_api_policy() { mc_root_script 'd=$(mktemp -d); printf "%s\n" "{\"Version\":\"2012-10-17\",\"Statement\":[{\"Effect\":\"Allow\",\"Action\":[\"s3:GetObject\"],\"Resource\":[\"arn:aws:s3:::sdds-media/system/readiness\"]}]}" >"$d/p"; mc admin policy detach root sdds-media-api --user "$API_ACCESS"; mc admin policy remove root sdds-media-api; mc admin policy create root sdds-media-api "$d/p"'; }
drift_sentinel() { root_aws s3api put-object --bucket sdds-media --key system/readiness --body /etc/rustfs-init/api-policy.json >/dev/null; }
drift_anonymous() { root_aws s3api put-public-access-block --bucket sdds-media --public-access-block-configuration BlockPublicAcls=false,IgnorePublicAcls=false,BlockPublicPolicy=false,RestrictPublicBuckets=false >/dev/null; mc_root anonymous set download root/sdds-media >/dev/null; }
drift_attachment() { mc_root admin policy detach root sdds-media-api --user "$media_access" >/dev/null; }

compose up --build -d >/dev/null; wait_for_api; compose run --rm --no-deps rustfs-init >/dev/null
expect_drift pab-v1 'public-access-block drift' drift_pab
expect_drift versioning-v1 'bucket versioning is enabled' drift_versioning
expect_drift lifecycle-v1 'bucket lifecycle is configured' drift_lifecycle
expect_drift policy-v1 'anonymous bucket policy is configured' drift_bucket_policy
expect_drift api-policy-v1 'API policy drift' drift_api_policy
expect_drift sentinel-v1 'readiness sentinel metadata drift' drift_sentinel
expect_drift anonymous-v1 'anonymous bucket policy is configured' drift_anonymous
expect_drift attachment-v1 'API user attachment drift' drift_attachment

printf 'rustfs-persistence-marker-v1\n' >"$TMP/marker"; printf 'sdds-media-ready-v1\n' >"$TMP/sentinel"
marker_hash=$(sha256 "$TMP/marker"); sentinel_hash=$(sha256 "$TMP/sentinel")
check_object() { key=$1; source=$2; remote=$3; local=$4; expected=$5; media_aws s3api get-object --bucket sdds-media --key "$key" "$remote" >/dev/null; cmp -s "$source" "$local" || die "$key payload drift"; [ "$(sha256 "$local")" = "$expected" ] || die "$key hash drift"; }
media_aws s3api put-object --bucket sdds-media --key note-images/persist-marker --body /tmp/sdds-test/marker >/dev/null
check_object note-images/persist-marker "$TMP/marker" /tmp/sdds-test/marker-before "$TMP/marker-before" "$marker_hash"
check_object system/readiness "$TMP/sentinel" /tmp/sdds-test/sentinel-before "$TMP/sentinel-before" "$sentinel_hash"

username=rustfs$(date +%s)$$; password=rustfs-integration-password
session=$(printf '{"username":"%s","password":"%s","display_name":"RustFS integration"}' "$username" "$password" | curl --silent --show-error --fail --max-time 10 -H 'Content-Type: application/json' --data-binary @- "$API_URL/v1/auth/users")
token=$(printf '%s' "$session" | python3 -c 'import json,sys; print(json.load(sys.stdin)["token"])')
note=$(printf '%s' '{"title":"RustFS persistence marker","body":"Compose volume marker","category_slug":"food"}' | curl --silent --show-error --fail --max-time 10 -H 'Content-Type: application/json' -H "Authorization: Bearer $token" --data-binary @- "$API_URL/v1/notes")
note_id=$(printf '%s' "$note" | python3 -c 'import json,sys; print(json.load(sys.stdin)["id"])')
compose up --force-recreate -d api rustfs >/dev/null; wait_for_api
check_object note-images/persist-marker "$TMP/marker" /tmp/sdds-test/marker-after "$TMP/marker-after" "$marker_hash"
check_object system/readiness "$TMP/sentinel" /tmp/sdds-test/sentinel-after "$TMP/sentinel-after" "$sentinel_hash"
curl --silent --show-error --fail --max-time 10 "$API_URL/v1/notes/$note_id" | python3 -c 'import json,sys; value=json.load(sys.stdin); raise SystemExit(value.get("id") != sys.argv[1] or value.get("title") != "RustFS persistence marker")' "$note_id"
compose run --build --rm --no-deps api migrate >/dev/null
printf '%s\n' 'rustfs integration verified'

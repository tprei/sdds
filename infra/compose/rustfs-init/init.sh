#!/bin/sh
set -eu
umask 077

die() { printf 'rustfs-init: %s\n' "$1" >&2; exit 1; }
read_secret() {
  [ -r "$1" ] || die "secret file is unreadable"
  value=$(tr -d '\r\n' <"$1") || die "secret file cannot be read"
  [ -n "$value" ] || die "secret file is empty"
  case "$value" in *[![:graph:]]*) die "secret contains whitespace";; esac
  printf '%s' "$value"
}

ENDPOINT=${RUSTFS_ENDPOINT:-http://rustfs:9000}
REGION=${AWS_REGION:-us-east-1}
BUCKET=sdds-media
POLICY_NAME=sdds-media-api
ROOT_ACCESS_FILE=${RUSTFS_ROOT_ACCESS_KEY_FILE:-/run/secrets/rustfs_root_access_key}
ROOT_SECRET_FILE=${RUSTFS_ROOT_SECRET_KEY_FILE:-/run/secrets/rustfs_root_secret_key}
API_ACCESS_FILE=${SDDS_MEDIA_ACCESS_KEY_FILE:-/run/secrets/sdds_media_access_key}
API_SECRET_FILE=${SDDS_MEDIA_SECRET_KEY_FILE:-/run/secrets/sdds_media_secret_key}
POLICY=/etc/rustfs-init/api-policy.json
SENTINEL_SHA256=5aff33ce5e386989939a8a504923897432db5b5a818518ccd876dadf2ad7398f
SENTINEL_CHECKSUM=Wv8zzl44aYmTmopQSSOJdDLbW1qBhRjM2Hba3yrXOY8=
case "$ENDPOINT" in http://*|https://*) ;; *) die "endpoint must use HTTP or HTTPS";; esac
[ -r "$POLICY" ] || die "policy file is unreadable"
ROOT_ACCESS=$(read_secret "$ROOT_ACCESS_FILE")
ROOT_SECRET=$(read_secret "$ROOT_SECRET_FILE")
API_ACCESS=$(read_secret "$API_ACCESS_FILE")
API_SECRET=$(read_secret "$API_SECRET_FILE")
export AWS_REGION="$REGION" AWS_EC2_METADATA_DISABLED=true AWS_CONFIG_FILE=/dev/null AWS_SHARED_CREDENTIALS_FILE=/dev/null
unset AWS_ACCESS_KEY_ID AWS_SECRET_ACCESS_KEY AWS_SESSION_TOKEN AWS_PROFILE

tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' 0 1 2 3 15
MC_CONFIG_DIR="$tmp/mc"
export MC_CONFIG_DIR
mkdir -p "$MC_CONFIG_DIR"
configure_mc_alias() { mc alias set "$1" "$2" "$3" "$4" --api S3v4; }
root_aws() { AWS_ACCESS_KEY_ID="$ROOT_ACCESS" AWS_SECRET_ACCESS_KEY="$ROOT_SECRET" aws --endpoint-url "$ENDPOINT" "$@"; }
api_aws() { AWS_ACCESS_KEY_ID="$API_ACCESS" AWS_SECRET_ACCESS_KEY="$API_SECRET" aws --endpoint-url "$ENDPOINT" "$@"; }
api_aws_with_secret() { api_aws_with_secret_secret=$1; shift; AWS_ACCESS_KEY_ID="$API_ACCESS" AWS_SECRET_ACCESS_KEY="$api_aws_with_secret_secret" aws --endpoint-url "$ENDPOINT" "$@"; }
unsigned_aws() { env -u AWS_ACCESS_KEY_ID -u AWS_SECRET_ACCESS_KEY -u AWS_SESSION_TOKEN -u AWS_PROFILE AWS_CONFIG_FILE=/dev/null AWS_SHARED_CREDENTIALS_FILE=/dev/null AWS_EC2_METADATA_DISABLED=true aws --no-sign-request --endpoint-url "$ENDPOINT" "$@"; }
command_is_access_denied() {
  if "$@" >"$tmp/error" 2>&1; then return 1; fi
  grep -Eiq 'AccessDenied|Access Denied' "$tmp/error"
}
command_rejects_api_secret() {
  if "$@" >"$tmp/error" 2>&1; then return 1; fi
  grep -Eiq 'SignatureDoesNotMatch|AccessDenied|Access Denied|Forbidden|403' "$tmp/error"
}

policy_document_matches() {
  python - "$1" "$2" <<'PY'
import json, sys

def statement(value):
    if not isinstance(value, dict) or set(value) - {'Effect', 'Action', 'Resource', 'Condition', 'Sid'}:
        raise ValueError()
    return {
        'Effect': value.get('Effect'),
        'Action': sorted(value.get('Action', [])),
        'Resource': sorted(value.get('Resource', [])),
        'Condition': value.get('Condition', {}),
        'Sid': value.get('Sid', '')
    }

got_doc = json.load(open(sys.argv[1]))
want = json.load(open(sys.argv[2]))
got = got_doc['policyInfo']['Policy']['policy']
if set(got) - set(('ID', 'Version', 'Statement')) or got.get('ID', '') != '':
    raise SystemExit(1)
if got.get('Version') != want.get('Version'):
    raise SystemExit(1)
got_statements = sorted(statement(x) for x in got.get('Statement', []))
want_statements = sorted(statement(x) for x in want.get('Statement', []))
if got_statements != want_statements:
    raise SystemExit(1)
PY
}
mc_error_is_missing() {
  python - "$1" "$2" "$3" <<'PY'
import json, sys
needle = sys.argv[3]
for path in sys.argv[1:3]:
    try:
        value = json.load(open(path))
    except (OSError, ValueError):
        continue
    cause = value.get('error', {}).get('cause', {})
    if value.get('status') == 'error' and (cause.get('message') == needle or cause.get('error', {}).get('Code') == needle):
        raise SystemExit(0)
raise SystemExit(1)
PY
}
api_user_attachment_matches() {
  python - "$1" <<'PY'
import json, sys
value = json.load(open(sys.argv[1]))
if set(value) != set(('status', 'accessKey', 'policyName', 'userStatus')):
    raise SystemExit(1)
if value.get('status') != 'success' or value.get('policyName') != 'sdds-media-api' or value.get('userStatus') != 'enabled':
    raise SystemExit(1)
PY
}
bucket_acl_is_owner_only() {
  python - "$1" "$2" <<'PY'
import json, sys
value = json.load(open(sys.argv[1]))
expected = json.load(open(sys.argv[2]))
grants = value.get('Grants')
owner = value.get('Owner')
if not isinstance(owner, dict) or owner != expected:
    raise SystemExit(1)
if not isinstance(grants, list) or len(grants) != 1:
    raise SystemExit(1)
grant = grants[0]
grantee = grant.get('Grantee')
if grant.get('Permission') != 'FULL_CONTROL' or not isinstance(grantee, dict) or grantee.get('Type') != 'CanonicalUser':
    raise SystemExit(1)
if 'ID' in grantee and grantee['ID'] != owner.get('ID'):
    raise SystemExit(1)
PY
}
anonymous_access_is_private() {
  python - "$1" <<'PY'
import json, sys
value = json.load(open(sys.argv[1]))
if value.get('status') != 'success' or value.get('permission') != 'private':
    raise SystemExit(1)
PY
}
sentinel_metadata_matches() {
  python - "$1" "$2" "$3" <<'PY'
import json, sys
value = json.load(open(sys.argv[1]))
if value.get('ContentLength') != 20 or value.get('ContentType') != 'application/octet-stream' or value.get('ChecksumSHA256') != sys.argv[3]:
    raise SystemExit(1)
if value.get('Metadata') != {'sha256': sys.argv[2]}:
    raise SystemExit(1)
PY
}

ensure_bucket_exists() {
  if ! root_aws s3api head-bucket --bucket "$1" >"$2/out" 2>"$2/error"; then
    root_aws s3api create-bucket --bucket "$1" >"$2/out" 2>"$2/error" || die "bucket create failed"
  fi
}
verify_bucket_owner_acl() {
  root_aws s3api list-buckets --query Owner --output json >"$2/root-owner" 2>"$2/error" || die "root owner lookup failed"
  root_aws s3api get-bucket-acl --bucket "$1" >"$2/acl" 2>"$2/error" || die "bucket ACL lookup failed"
  bucket_acl_is_owner_only "$2/acl" "$2/root-owner" || die "bucket ownership or ACL drift"
}
verify_bucket_versioning_disabled() {
  verify_bucket_versioning_disabled_version=$(root_aws s3api get-bucket-versioning --bucket "$1" --query Status --output text 2>"$2/error") || die "versioning lookup failed"
  [ "$verify_bucket_versioning_disabled_version" = None ] || die "bucket versioning is enabled"
}
verify_bucket_object_lock_disabled() {
  if root_aws s3api get-object-lock-configuration --bucket "$1" >"$2/out" 2>"$2/error"; then
    die "object lock is enabled"
  fi
  grep -Eq 'ObjectLockConfigurationNotFoundError' "$2/error" || die "object lock status is unsupported"
}
verify_bucket_lifecycle_absent() {
  if root_aws s3api get-bucket-lifecycle-configuration --bucket "$1" >"$2/out" 2>"$2/error"; then
    die "bucket lifecycle is configured"
  fi
  grep -Eq 'NoSuchLifecycleConfiguration' "$2/error" || die "lifecycle status is unsupported"
}
verify_bucket_policy_absent() {
  if root_aws s3api get-bucket-policy --bucket "$1" >"$2/out" 2>"$2/error"; then
    die "anonymous bucket policy is configured"
  fi
  grep -Eq 'NoSuchBucketPolicy' "$2/error" || die "bucket policy status is unsupported"
}
verify_anonymous_access_private() {
  mc anonymous get "root/$1" --json >"$2/anonymous" 2>"$2/error" || die "anonymous access lookup failed"
  anonymous_access_is_private "$2/anonymous" || die "anonymous access is not private"
}
ensure_public_access_block() {
  if root_aws s3api get-public-access-block --bucket "$1" --query PublicAccessBlockConfiguration --output json >"$2/pab" 2>"$2/error"; then
    [ "$(tr -d '[:space:]' <"$2/pab")" = '{"BlockPublicAcls":true,"IgnorePublicAcls":true,"BlockPublicPolicy":true,"RestrictPublicBuckets":true}' ] || die "public-access-block drift"
  else
    root_aws s3api put-public-access-block --bucket "$1" --public-access-block-configuration BlockPublicAcls=true,IgnorePublicAcls=true,BlockPublicPolicy=true,RestrictPublicBuckets=true >"$2/out" 2>"$2/error" || die "public-access-block setup failed"
    root_aws s3api get-public-access-block --bucket "$1" --query PublicAccessBlockConfiguration --output json >"$2/pab" 2>"$2/error" || die "public-access-block verification failed"
    [ "$(tr -d '[:space:]' <"$2/pab")" = '{"BlockPublicAcls":true,"IgnorePublicAcls":true,"BlockPublicPolicy":true,"RestrictPublicBuckets":true}' ] || die "public-access-block setup drift"
  fi
}
bootstrap_private_bucket() {
  ensure_bucket_exists "$1" "$2"
  verify_bucket_owner_acl "$1" "$2"
  verify_bucket_versioning_disabled "$1" "$2"
  verify_bucket_object_lock_disabled "$1" "$2"
  verify_bucket_lifecycle_absent "$1" "$2"
  verify_bucket_policy_absent "$1" "$2"
  verify_anonymous_access_private "$1" "$2"
  ensure_public_access_block "$1" "$2"
}

ensure_api_policy() {
  if mc admin policy info root "$1" --json >"$3/policy" 2>"$3/error"; then
    policy_document_matches "$3/policy" "$2" || die "API policy drift"
  else
    mc_error_is_missing "$3/policy" "$3/error" "policy does not exist" || die "API policy lookup failed"
    mc admin policy create root "$1" "$2" >"$3/out" 2>"$3/error" || die "API policy create failed"
    mc admin policy info root "$1" --json >"$3/policy" 2>"$3/error" || die "API policy lookup failed"
    policy_document_matches "$3/policy" "$2" || die "API policy verification failed"
  fi
}
ensure_api_user() {
  if mc admin user info root "$1" --json >"$4/user" 2>"$4/error"; then
    api_user_attachment_matches "$4/user" || die "API user attachment drift"
  else
    mc_error_is_missing "$4/user" "$4/error" "NoSuchResource" || die "API user lookup failed"
    mc admin user add root "$1" "$2" >"$4/out" 2>"$4/error" || die "API user create failed"
  fi
  mc admin policy attach root "$3" --user "$1" >"$4/out" 2>"$4/error" || die "API policy attachment failed"
  mc admin user info root "$1" --json >"$4/user" 2>"$4/error" || die "API user lookup failed"
  api_user_attachment_matches "$4/user" || die "API user verification failed"
}

ensure_readiness_sentinel() {
  printf '%s\n' 'sdds-media-ready-v1' >"$2/sentinel"
  if root_aws s3api head-object --bucket "$1" --key system/readiness --checksum-mode ENABLED \
      --query '{ContentLength:ContentLength,ContentType:ContentType,ChecksumSHA256:ChecksumSHA256,Metadata:Metadata}' --output json >"$2/meta" 2>"$2/error"; then
    sentinel_metadata_matches "$2/meta" "$3" "$4" || die "readiness sentinel metadata drift"
  else
    grep -Eq '(NoSuchKey|NotFound|404)' "$2/error" || die "readiness sentinel lookup failed"
    root_aws s3api put-object --bucket "$1" --key system/readiness --body "$2/sentinel" --content-type application/octet-stream \
      --checksum-algorithm SHA256 --checksum-sha256 "$4" --metadata "sha256=$3" >"$2/out" 2>"$2/error" || die "readiness sentinel create failed"
    root_aws s3api head-object --bucket "$1" --key system/readiness --checksum-mode ENABLED \
      --query '{ContentLength:ContentLength,ContentType:ContentType,ChecksumSHA256:ChecksumSHA256,Metadata:Metadata}' --output json >"$2/meta" 2>"$2/error" || die "readiness sentinel verification failed"
    sentinel_metadata_matches "$2/meta" "$3" "$4" || die "readiness sentinel setup drift"
  fi
  root_aws s3api get-object --bucket "$1" --key system/readiness "$2/sentinel-root" >"$2/out" 2>"$2/error" || die "root sentinel read failed"
  cmp -s "$2/sentinel" "$2/sentinel-root" || die "readiness sentinel payload drift"
  [ "$(sha256sum "$2/sentinel-root" | awk '{print $1}')" = "$3" ] || die "readiness sentinel hash drift"
}
verify_api_secret_rotation() {
  verify_api_secret_rotation_secret="sdds-rotation-$2"
  mc admin user add root "$2" "$verify_api_secret_rotation_secret" >"$4/out" 2>"$4/error" || die "API secret rotation setup failed"
  api_aws_with_secret "$verify_api_secret_rotation_secret" s3api head-object --bucket "$1" --key system/readiness --checksum-mode ENABLED \
    --query '{ContentLength:ContentLength,ContentType:ContentType,ChecksumSHA256:ChecksumSHA256,Metadata:Metadata}' --output json >"$4/rotation-meta" 2>"$4/error" || die "rotated API credential failed"
  sentinel_metadata_matches "$4/rotation-meta" "$SENTINEL_SHA256" "$SENTINEL_CHECKSUM" || die "rotated API credential metadata failed"
  mc admin user add root "$2" "$3" >"$4/out" 2>"$4/error" || die "API secret reapply failed"
  mc admin policy attach root "$POLICY_NAME" --user "$2" >"$4/out" 2>"$4/error" || die "API policy reattachment failed"
  mc admin user info root "$2" --json >"$4/user" 2>"$4/error" || die "API user lookup failed"
  api_user_attachment_matches "$4/user" || die "API user verification failed"
  command_rejects_api_secret api_aws_with_secret "$verify_api_secret_rotation_secret" s3api head-object --bucket "$1" --key system/readiness || die "old API secret was accepted"
}
verify_api_readiness_access() {
  api_aws s3api head-object --bucket "$1" --key system/readiness --checksum-mode ENABLED \
    --query '{ContentLength:ContentLength,ContentType:ContentType,ChecksumSHA256:ChecksumSHA256,Metadata:Metadata}' --output json >"$2/api-meta" 2>"$2/error" || die "API readiness permission failed"
  sentinel_metadata_matches "$2/api-meta" "$SENTINEL_SHA256" "$SENTINEL_CHECKSUM" || die "API readiness metadata failed"
}
verify_api_admin_denials() {
  configure_mc_alias api "$ENDPOINT" "$2" "$API_SECRET" >"$3/out" 2>"$3/error" || die "API mc login unexpectedly failed"
  command_is_access_denied mc admin policy info api "$1" --json || die "API policy-info denial was not AccessDenied"; command_is_access_denied mc admin policy list api --json || die "API policy-list denial was not AccessDenied"; command_is_access_denied mc admin policy attach api "$1" --user "$2" || die "API policy-attach denial was not AccessDenied"; command_is_access_denied mc admin user add api sdds-denied-probe sdds-denied-secret || die "API user-add denial was not AccessDenied"
}
verify_api_object_permissions() {
  printf '%s\n' 'sdds-init-probe-v1' >"$2/probe"
  verify_api_object_permissions_probe_hash=$(sha256sum "$2/probe" | awk '{print $1}')
  api_aws s3api put-object --bucket "$1" --key note-images/.sdds-init-probe --body "$2/probe" --content-type application/octet-stream --metadata "sha256=$verify_api_object_permissions_probe_hash" >"$2/out" 2>"$2/error" || die "API note-images write permission failed"
  api_aws s3api get-object --bucket "$1" --key note-images/.sdds-init-probe "$2/probe-get" >"$2/out" 2>"$2/error" || die "API note-images read permission failed"
  cmp -s "$2/probe" "$2/probe-get" || die "API note-images payload mismatch"
  api_aws s3api delete-object --bucket "$1" --key note-images/.sdds-init-probe >"$2/out" 2>"$2/error" || die "API note-images delete permission failed"
  for verify_api_object_permissions_key in system/init-probe note-images-evil/init-probe; do
    command_is_access_denied api_aws s3api put-object --bucket "$1" --key "$verify_api_object_permissions_key" --body "$2/probe" || die "API write denial was not AccessDenied"
    command_is_access_denied api_aws s3api get-object --bucket "$1" --key "$verify_api_object_permissions_key" "$2/blocked" || die "API read denial was not AccessDenied"
    command_is_access_denied api_aws s3api delete-object --bucket "$1" --key "$verify_api_object_permissions_key" || die "API delete denial was not AccessDenied"
  done
  command_is_access_denied api_aws s3api list-objects-v2 --bucket "$1" || die "API list denial was not AccessDenied"
  if api_aws s3api head-bucket --bucket "$1" >"$2/out" 2>"$2/error"; then die "API bucket inspection succeeded"; fi
  grep -Eiq 'Forbidden|403' "$2/error" || die "API bucket denial was not HTTP 403"
}
verify_unsigned_private_access() {
  command_is_access_denied unsigned_aws s3api get-object --bucket "$1" --key system/readiness "$2/unsigned-get" || die "unsigned GET denial was not AccessDenied"
  command_is_access_denied unsigned_aws s3api list-objects-v2 --bucket "$1" || die "unsigned LIST denial was not AccessDenied"
}
run_bootstrap() {
  configure_mc_alias root "$ENDPOINT" "$ROOT_ACCESS" "$ROOT_SECRET" >"$1/out" 2>"$1/error" || die "root admin login failed"
  bootstrap_private_bucket "$BUCKET" "$1"
  ensure_api_policy "$POLICY_NAME" "$POLICY" "$1"
  ensure_api_user "$API_ACCESS" "$API_SECRET" "$POLICY_NAME" "$1"
  ensure_readiness_sentinel "$BUCKET" "$1" "$SENTINEL_SHA256" "$SENTINEL_CHECKSUM"
  verify_api_secret_rotation "$BUCKET" "$API_ACCESS" "$API_SECRET" "$1"
  verify_api_readiness_access "$BUCKET" "$1"
  verify_api_admin_denials "$POLICY_NAME" "$API_ACCESS" "$1"
  verify_api_object_permissions "$BUCKET" "$1"
  verify_unsigned_private_access "$BUCKET" "$1"
  printf '%s\n' 'rustfs bootstrap verified'
}
run_bootstrap "$tmp"

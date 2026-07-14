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
mc alias set root "$ENDPOINT" "$ROOT_ACCESS" "$ROOT_SECRET" --api S3v4 >"$tmp/out" 2>"$tmp/error" || die "root admin login failed"
aws_root() { AWS_ACCESS_KEY_ID="$ROOT_ACCESS" AWS_SECRET_ACCESS_KEY="$ROOT_SECRET" aws --endpoint-url "$ENDPOINT" "$@"; }
aws_api() { AWS_ACCESS_KEY_ID="$API_ACCESS" AWS_SECRET_ACCESS_KEY="$API_SECRET" aws --endpoint-url "$ENDPOINT" "$@"; }
aws_credentials() { access=$1; secret=$2; shift 2; AWS_ACCESS_KEY_ID="$access" AWS_SECRET_ACCESS_KEY="$secret" aws --endpoint-url "$ENDPOINT" "$@"; }
unsigned() { env -u AWS_ACCESS_KEY_ID -u AWS_SECRET_ACCESS_KEY -u AWS_SESSION_TOKEN -u AWS_PROFILE AWS_CONFIG_FILE=/dev/null AWS_SHARED_CREDENTIALS_FILE=/dev/null AWS_EC2_METADATA_DISABLED=true aws --no-sign-request --endpoint-url "$ENDPOINT" "$@"; }
expect_denied() {
  if "$@" >"$tmp/error" 2>&1; then return 1; fi
  grep -Eiq 'AccessDenied|Access Denied' "$tmp/error"
}
expect_secret_denied() {
  if "$@" >"$tmp/error" 2>&1; then return 1; fi
  grep -Eiq 'SignatureDoesNotMatch|AccessDenied|Access Denied|Forbidden|403' "$tmp/error"
}

check_policy() {
  python - "$1" "$POLICY" <<'PY'
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
mc_missing() {
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
check_user() {
  python - "$1" <<'PY'
import json, sys
value = json.load(open(sys.argv[1]))
if set(value) != set(('status', 'accessKey', 'policyName', 'userStatus')):
    raise SystemExit(1)
if value.get('status') != 'success' or value.get('policyName') != 'sdds-media-api' or value.get('userStatus') != 'enabled':
    raise SystemExit(1)
PY
}
check_acl() {
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
check_anonymous() {
  python - "$1" <<'PY'
import json, sys
value = json.load(open(sys.argv[1]))
if value.get('status') != 'success' or value.get('permission') != 'private':
    raise SystemExit(1)
PY
}
check_meta() {
  python - "$1" "$SENTINEL_SHA256" "$SENTINEL_CHECKSUM" <<'PY'
import json, sys
value = json.load(open(sys.argv[1]))
if value.get('ContentLength') != 20 or value.get('ContentType') != 'application/octet-stream' or value.get('ChecksumSHA256') != sys.argv[3]:
    raise SystemExit(1)
if value.get('Metadata') != {'sha256': sys.argv[2]}:
    raise SystemExit(1)
PY
}

if ! aws_root s3api head-bucket --bucket "$BUCKET" >"$tmp/out" 2>"$tmp/error"; then
  aws_root s3api create-bucket --bucket "$BUCKET" >"$tmp/out" 2>"$tmp/error" || die "bucket create failed"
fi
aws_root s3api list-buckets --query Owner --output json >"$tmp/root-owner" 2>"$tmp/error" || die "root owner lookup failed"
aws_root s3api get-bucket-acl --bucket "$BUCKET" >"$tmp/acl" 2>"$tmp/error" || die "bucket ACL lookup failed"
check_acl "$tmp/acl" "$tmp/root-owner" || die "bucket ownership or ACL drift"
version=$(aws_root s3api get-bucket-versioning --bucket "$BUCKET" --query Status --output text 2>"$tmp/error") || die "versioning lookup failed"
[ "$version" = None ] || die "bucket versioning is enabled"
if aws_root s3api get-object-lock-configuration --bucket "$BUCKET" >"$tmp/out" 2>"$tmp/error"; then
  die "object lock is enabled"
fi
grep -Eq 'ObjectLockConfigurationNotFoundError' "$tmp/error" || die "object lock status is unsupported"
if aws_root s3api get-bucket-lifecycle-configuration --bucket "$BUCKET" >"$tmp/out" 2>"$tmp/error"; then
  die "bucket lifecycle is configured"
fi
grep -Eq 'NoSuchLifecycleConfiguration' "$tmp/error" || die "lifecycle status is unsupported"
if aws_root s3api get-bucket-policy --bucket "$BUCKET" >"$tmp/out" 2>"$tmp/error"; then
  die "anonymous bucket policy is configured"
fi
grep -Eq 'NoSuchBucketPolicy' "$tmp/error" || die "bucket policy status is unsupported"
mc anonymous get "root/$BUCKET" --json >"$tmp/anonymous" 2>"$tmp/error" || die "anonymous access lookup failed"
check_anonymous "$tmp/anonymous" || die "anonymous access is not private"

if aws_root s3api get-public-access-block --bucket "$BUCKET" --query PublicAccessBlockConfiguration --output json >"$tmp/pab" 2>"$tmp/error"; then
  [ "$(tr -d '[:space:]' <"$tmp/pab")" = '{"BlockPublicAcls":true,"IgnorePublicAcls":true,"BlockPublicPolicy":true,"RestrictPublicBuckets":true}' ] || die "public-access-block drift"
else
  aws_root s3api put-public-access-block --bucket "$BUCKET" --public-access-block-configuration BlockPublicAcls=true,IgnorePublicAcls=true,BlockPublicPolicy=true,RestrictPublicBuckets=true >"$tmp/out" 2>"$tmp/error" || die "public-access-block setup failed"
  aws_root s3api get-public-access-block --bucket "$BUCKET" --query PublicAccessBlockConfiguration --output json >"$tmp/pab" 2>"$tmp/error" || die "public-access-block verification failed"
  [ "$(tr -d '[:space:]' <"$tmp/pab")" = '{"BlockPublicAcls":true,"IgnorePublicAcls":true,"BlockPublicPolicy":true,"RestrictPublicBuckets":true}' ] || die "public-access-block setup drift"
fi

if mc admin policy info root "$POLICY_NAME" --json >"$tmp/policy" 2>"$tmp/error"; then
  check_policy "$tmp/policy" || die "API policy drift"
else
  mc_missing "$tmp/policy" "$tmp/error" "policy does not exist" || die "API policy lookup failed"
  mc admin policy create root "$POLICY_NAME" "$POLICY" >"$tmp/out" 2>"$tmp/error" || die "API policy create failed"
  mc admin policy info root "$POLICY_NAME" --json >"$tmp/policy" 2>"$tmp/error" || die "API policy lookup failed"
  check_policy "$tmp/policy" || die "API policy verification failed"
fi
if mc admin user info root "$API_ACCESS" --json >"$tmp/user" 2>"$tmp/error"; then
  check_user "$tmp/user" || die "API user attachment drift"
else
  mc_missing "$tmp/user" "$tmp/error" "NoSuchResource" || die "API user lookup failed"
  mc admin user add root "$API_ACCESS" "$API_SECRET" >"$tmp/out" 2>"$tmp/error" || die "API user create failed"
fi
mc admin policy attach root "$POLICY_NAME" --user "$API_ACCESS" >"$tmp/out" 2>"$tmp/error" || die "API policy attachment failed"
mc admin user info root "$API_ACCESS" --json >"$tmp/user" 2>"$tmp/error" || die "API user lookup failed"
check_user "$tmp/user" || die "API user verification failed"

printf '%s\n' 'sdds-media-ready-v1' >"$tmp/sentinel"
if aws_root s3api head-object --bucket "$BUCKET" --key system/readiness --checksum-mode ENABLED \
    --query '{ContentLength:ContentLength,ContentType:ContentType,ChecksumSHA256:ChecksumSHA256,Metadata:Metadata}' --output json >"$tmp/meta" 2>"$tmp/error"; then
  check_meta "$tmp/meta" || die "readiness sentinel metadata drift"
else
  grep -Eq '(NoSuchKey|NotFound|404)' "$tmp/error" || die "readiness sentinel lookup failed"
  aws_root s3api put-object --bucket "$BUCKET" --key system/readiness --body "$tmp/sentinel" --content-type application/octet-stream \
    --checksum-algorithm SHA256 --checksum-sha256 "$SENTINEL_CHECKSUM" --metadata "sha256=$SENTINEL_SHA256" >"$tmp/out" 2>"$tmp/error" || die "readiness sentinel create failed"
  aws_root s3api head-object --bucket "$BUCKET" --key system/readiness --checksum-mode ENABLED \
    --query '{ContentLength:ContentLength,ContentType:ContentType,ChecksumSHA256:ChecksumSHA256,Metadata:Metadata}' --output json >"$tmp/meta" 2>"$tmp/error" || die "readiness sentinel verification failed"
  check_meta "$tmp/meta" || die "readiness sentinel setup drift"
fi
aws_root s3api get-object --bucket "$BUCKET" --key system/readiness "$tmp/sentinel-root" >"$tmp/out" 2>"$tmp/error" || die "root sentinel read failed"
cmp -s "$tmp/sentinel" "$tmp/sentinel-root" || die "readiness sentinel payload drift"
[ "$(sha256sum "$tmp/sentinel-root" | awk '{print $1}')" = "$SENTINEL_SHA256" ] || die "readiness sentinel hash drift"

ROTATION_SECRET="sdds-rotation-$API_ACCESS"
mc admin user add root "$API_ACCESS" "$ROTATION_SECRET" >"$tmp/out" 2>"$tmp/error" || die "API secret rotation setup failed"
aws_credentials "$API_ACCESS" "$ROTATION_SECRET" s3api head-object --bucket "$BUCKET" --key system/readiness --checksum-mode ENABLED \
  --query '{ContentLength:ContentLength,ContentType:ContentType,ChecksumSHA256:ChecksumSHA256,Metadata:Metadata}' --output json >"$tmp/rotation-meta" 2>"$tmp/error" || die "rotated API credential failed"
check_meta "$tmp/rotation-meta" || die "rotated API credential metadata failed"
mc admin user add root "$API_ACCESS" "$API_SECRET" >"$tmp/out" 2>"$tmp/error" || die "API secret reapply failed"
mc admin policy attach root "$POLICY_NAME" --user "$API_ACCESS" >"$tmp/out" 2>"$tmp/error" || die "API policy reattachment failed"
mc admin user info root "$API_ACCESS" --json >"$tmp/user" 2>"$tmp/error" || die "API user lookup failed"
check_user "$tmp/user" || die "API user verification failed"
expect_secret_denied aws_credentials "$API_ACCESS" "$ROTATION_SECRET" s3api head-object --bucket "$BUCKET" --key system/readiness || die "old API secret was accepted"
aws_api s3api head-object --bucket "$BUCKET" --key system/readiness --checksum-mode ENABLED \
  --query '{ContentLength:ContentLength,ContentType:ContentType,ChecksumSHA256:ChecksumSHA256,Metadata:Metadata}' --output json >"$tmp/api-meta" 2>"$tmp/error" || die "API readiness permission failed"
check_meta "$tmp/api-meta" || die "API readiness metadata failed"
mc alias set api "$ENDPOINT" "$API_ACCESS" "$API_SECRET" --api S3v4 >"$tmp/out" 2>"$tmp/error" || die "API mc login unexpectedly failed"
expect_denied mc admin policy info api "$POLICY_NAME" --json || die "API policy-info denial was not AccessDenied"; expect_denied mc admin policy list api --json || die "API policy-list denial was not AccessDenied"; expect_denied mc admin policy attach api "$POLICY_NAME" --user "$API_ACCESS" || die "API policy-attach denial was not AccessDenied"; expect_denied mc admin user add api sdds-denied-probe sdds-denied-secret || die "API user-add denial was not AccessDenied"

printf '%s\n' 'sdds-init-probe-v1' >"$tmp/probe"
probe_hash=$(sha256sum "$tmp/probe" | awk '{print $1}')
aws_api s3api put-object --bucket "$BUCKET" --key note-images/.sdds-init-probe --body "$tmp/probe" --content-type application/octet-stream --metadata "sha256=$probe_hash" >"$tmp/out" 2>"$tmp/error" || die "API note-images write permission failed"
aws_api s3api get-object --bucket "$BUCKET" --key note-images/.sdds-init-probe "$tmp/probe-get" >"$tmp/out" 2>"$tmp/error" || die "API note-images read permission failed"
cmp -s "$tmp/probe" "$tmp/probe-get" || die "API note-images payload mismatch"
aws_api s3api delete-object --bucket "$BUCKET" --key note-images/.sdds-init-probe >"$tmp/out" 2>"$tmp/error" || die "API note-images delete permission failed"
for key in system/init-probe note-images-evil/init-probe; do
  expect_denied aws_api s3api put-object --bucket "$BUCKET" --key "$key" --body "$tmp/probe" || die "API write denial was not AccessDenied"
  expect_denied aws_api s3api get-object --bucket "$BUCKET" --key "$key" "$tmp/blocked" || die "API read denial was not AccessDenied"
  expect_denied aws_api s3api delete-object --bucket "$BUCKET" --key "$key" || die "API delete denial was not AccessDenied"
done
expect_denied aws_api s3api list-objects-v2 --bucket "$BUCKET" || die "API list denial was not AccessDenied"
if aws_api s3api head-bucket --bucket "$BUCKET" >"$tmp/out" 2>"$tmp/error"; then die "API bucket inspection succeeded"; fi
grep -Eiq 'Forbidden|403' "$tmp/error" || die "API bucket denial was not HTTP 403"
expect_denied unsigned s3api get-object --bucket "$BUCKET" --key system/readiness "$tmp/unsigned-get" || die "unsigned GET denial was not AccessDenied"
expect_denied unsigned s3api list-objects-v2 --bucket "$BUCKET" || die "unsigned LIST denial was not AccessDenied"
printf '%s\n' 'rustfs bootstrap verified'

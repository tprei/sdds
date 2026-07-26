#!/bin/sh
set -eu
umask 077

die() { printf 'compose-secret-check: %s\n' "$1" >&2; exit 1; }
. /usr/local/lib/rustfs-init/secret-file.sh
read_secret /run/secrets/rustfs_root_access_key SDDS_COMPOSE_RUSTFS_ROOT_ACCESS_KEY_FILE >/dev/null
read_secret /run/secrets/rustfs_root_secret_key SDDS_COMPOSE_RUSTFS_ROOT_SECRET_KEY_FILE >/dev/null
read_secret /run/secrets/sdds_media_access_key SDDS_COMPOSE_SDDS_MEDIA_ACCESS_KEY_FILE >/dev/null
read_secret /run/secrets/sdds_media_secret_key SDDS_COMPOSE_SDDS_MEDIA_SECRET_KEY_FILE >/dev/null

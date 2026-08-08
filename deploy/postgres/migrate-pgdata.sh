#!/bin/sh
set -eu

root=${POSTGRES_DATA_ROOT:-/var/lib/postgresql/data}
target=$root/pgdata
uid=${POSTGRES_UID:-70}
gid=${POSTGRES_GID:-70}

legacy=false
destination=false
[ -f "$root/PG_VERSION" ] && legacy=true
[ -f "$target/PG_VERSION" ] && destination=true

if [ "$legacy" = true ] && [ "$destination" = true ]; then
  echo "refusing to merge legacy and pgdata PostgreSQL clusters" >&2
  exit 1
fi

if [ "$legacy" = true ]; then
  mkdir -p "$target"
  find "$root" -mindepth 1 -maxdepth 1 \
    ! -name pgdata ! -name lost+found ! -name .snapshot \
    -exec mv {} "$target"/ \;
fi

mkdir -p "$target"
chown -R "$uid:$gid" "$target"
chmod 0700 "$target"
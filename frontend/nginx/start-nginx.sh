#!/bin/sh
set -eu

TEMPLATE="${NGINX_CONF_TEMPLATE:-/etc/nginx/nginx.conf.template}"
RENDERED="${NGINX_CONF_RENDERED:-/tmp/nginx.conf}"

resolver_ip=$(awk '/^nameserver[[:space:]]/ { print $2; exit }' /etc/resolv.conf)
if [ -z "$resolver_ip" ]; then
  echo "start-nginx: no nameserver found in /etc/resolv.conf" >&2
  exit 1
fi

sed "s/__DNS_RESOLVER__/${resolver_ip}/g" "$TEMPLATE" > "$RENDERED"

exec nginx -c "$RENDERED" -g 'daemon off;'

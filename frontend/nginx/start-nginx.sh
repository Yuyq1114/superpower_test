#!/bin/sh
set -eu

TEMPLATE="${NGINX_CONF_TEMPLATE:-/etc/nginx/nginx.conf.template}"
RENDERED="${NGINX_CONF_RENDERED:-/tmp/nginx.conf}"

resolver_raw=$(awk '/^nameserver[[:space:]]/ { print $2; exit }' /etc/resolv.conf)
if [ -z "$resolver_raw" ]; then
  echo "start-nginx: no nameserver found in /etc/resolv.conf" >&2
  exit 1
fi

# Strict IPv4 validator/normalizer: exactly four decimal octets, each 0-255.
# Anything else (extra/missing dots, letters, CIDR suffixes, shell/sed
# metacharacters, ...) is rejected before it can reach sed.
normalize_ipv4() {
  addr=$1
  oldifs=$IFS
  IFS=.
  # Intentionally unquoted: IFS=. field-splits addr into positional params.
  set -- $addr
  IFS=$oldifs
  if [ "$#" -ne 4 ]; then
    return 1
  fi
  for octet in "$@"; do
    case "$octet" in
      ''|*[!0-9]*) return 1 ;;
    esac
    if [ "${#octet}" -gt 3 ] || [ "$octet" -gt 255 ]; then
      return 1
    fi
  done
  printf '%s' "$addr"
}

# IPv6 validator/normalizer: strips an optional %zone suffix (scoped
# link-local addresses), rejects an empty body, allows only hex digits,
# colons, and dots (for IPv4-mapped tails like ::ffff:192.0.2.1) in what
# remains, and wraps the result in brackets as nginx's resolver directive
# requires. Any other character (especially '/', '|', '&', or other
# shell/sed metacharacters) is rejected before it can reach sed.
normalize_ipv6() {
  addr=$1
  body=${addr%%%*}
  if [ -z "$body" ]; then
    return 1
  fi
  case "$body" in
    *[!0-9A-Fa-f:.]*) return 1 ;;
  esac
  printf '[%s]' "$body"
}

case "$resolver_raw" in
  *:*)
    if ! resolver_ip=$(normalize_ipv6 "$resolver_raw"); then
      echo "start-nginx: invalid IPv6 nameserver address: $resolver_raw" >&2
      exit 1
    fi
    ;;
  *)
    if ! resolver_ip=$(normalize_ipv4 "$resolver_raw"); then
      echo "start-nginx: invalid IPv4 nameserver address: $resolver_raw" >&2
      exit 1
    fi
    ;;
esac

# '#' is used as the sed delimiter (instead of '/') because it can never
# appear in a validated resolver_ip (plain IPv4 octets, or a bracketed IPv6
# address restricted to hex digits, colons, and dots), so the substitution
# cannot be broken out of or reinterpreted as extra sed commands.
sed "s#__DNS_RESOLVER__#${resolver_ip}#g" "$TEMPLATE" > "$RENDERED"

exec nginx -c "$RENDERED" -g 'daemon off;'

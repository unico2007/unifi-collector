#!/usr/bin/env bash
# Discovers which UniFi events endpoint this controller supports.
# Usage:
#   UNIFI_PASSWORD='your-password' bash scripts/find-event-endpoint.sh
# Optional overrides: UNIFI_URL, UNIFI_USERNAME, UNIFI_SITE
set -u

BASE="${UNIFI_URL:-https://10.10.0.3}"
USER_NAME="${UNIFI_USERNAME:-admin}"
PASS="${UNIFI_PASSWORD:?set UNIFI_PASSWORD}"
SITE="${UNIFI_SITE:-default}"
COOKIES="$(mktemp)"
OUT="$(mktemp)"

echo "Logging in to $BASE as $USER_NAME ..."
CSRF="$(curl -sk -c "$COOKIES" -D - -o /dev/null \
  -X POST "$BASE/api/auth/login" \
  -H 'Content-Type: application/json' \
  -d "{\"username\":\"$USER_NAME\",\"password\":\"$PASS\"}" \
  | awk 'tolower($1)=="x-csrf-token:"{print $2}' | tr -d '\r')"
echo "CSRF token: ${CSRF:-(none)}"
echo "-------------------------------------------------------------"

try() {
  local method="$1" path="$2" body="${3:-}"
  local url="$BASE/proxy/network/api/s/$SITE/$path"
  local code
  if [ "$method" = "GET" ]; then
    code="$(curl -sk -b "$COOKIES" -o "$OUT" -w '%{http_code}' "$url")"
  else
    code="$(curl -sk -b "$COOKIES" -H "X-CSRF-Token: $CSRF" -H 'Content-Type: application/json' \
      -o "$OUT" -w '%{http_code}' -X POST "$url" -d "$body")"
  fi
  printf "%-4s %-24s -> %s   %s\n" "$method" "$path" "$code" "$(head -c 90 "$OUT" | tr -d '\n')"
}

echo "Trying candidate event endpoints:"
try GET  "stat/event"
try POST "stat/event" '{"_limit":100,"within":24}'
try POST "stat/event" '{}'
try GET  "rest/event"
try GET  "stat/alarm"
try POST "stat/alarm" '{"_limit":100,"within":24}'
echo "-------------------------------------------------------------"
echo "A line ending in 200 with JSON data is the working endpoint."

rm -f "$COOKIES" "$OUT"

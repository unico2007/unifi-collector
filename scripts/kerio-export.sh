#!/usr/bin/env bash
# Kerio Control log exporter (macOS/Linux). Fetches all log categories via
# Logs.get and saves each to kerio-logs/<name>.json. The last-2-weeks filter +
# Loki import happen afterward on the collector host.
#
# Usage:
#   KERIO_URL='https://10.10.0.1:4081' KERIO_USERNAME='admin' \
#   KERIO_PASSWORD='yourpass' bash scripts/kerio-export.sh
#
# Optional: KERIO_COUNTLINES=500000  (recent lines per log), OUTDIR=kerio-logs
set -u

KERIO_URL="${KERIO_URL:?set KERIO_URL, e.g. https://10.10.0.1:4081}"
KERIO_USERNAME="${KERIO_USERNAME:-admin}"
KERIO_PASSWORD="${KERIO_PASSWORD:?set KERIO_PASSWORD}"
COUNT="${KERIO_COUNTLINES:-50000}" # Kerio caps Logs.get at 50000 lines/request
OUTDIR="${OUTDIR:-kerio-logs}"
ENDPOINT="${KERIO_URL%/}/admin/api/jsonrpc/"
CJ="$(mktemp)"
mkdir -p "$OUTDIR"

LOGS=(alert config connection debug dial error filter http security sslvpn warning web)

echo "Endpoint: $ENDPOINT"
echo "Output:   $OUTDIR   (lines/log: $COUNT)"
echo "-------------------------------------------------------------"

# --- login ---
LOGIN_BODY=$(cat <<JSON
{"jsonrpc":"2.0","id":1,"method":"Session.login","params":{"userName":"$KERIO_USERNAME","password":"$KERIO_PASSWORD","application":{"name":"log-export","vendor":"murad","version":"1.0"}}}
JSON
)
HDRS=$(/usr/bin/curl -sk -c "$CJ" -D - -o /tmp/klogin.json \
  -X POST "$ENDPOINT" -H 'Content-Type: application/json' -d "$LOGIN_BODY")
TOKEN=$(printf '%s' "$HDRS" | awk 'tolower($1)=="x-token:"{print $2}' | tr -d '\r')
[ -z "$TOKEN" ] && TOKEN=$(grep -o '"token":"[^"]*"' /tmp/klogin.json | head -1 | cut -d'"' -f4)
if [ -z "$TOKEN" ]; then
  echo "LOGIN FAILED. Response:"; head -c 300 /tmp/klogin.json; echo
  rm -f "$CJ" /tmp/klogin.json; exit 1
fi
echo "Login OK."
echo "-------------------------------------------------------------"

for name in "${LOGS[@]}"; do
  printf 'Fetching %-12s ... ' "$name"
  body="{\"jsonrpc\":\"2.0\",\"id\":2,\"method\":\"Logs.get\",\"params\":{\"logName\":\"$name\",\"fromLine\":-1,\"countLines\":$COUNT}}"
  if /usr/bin/curl -sk -b "$CJ" -X POST "$ENDPOINT" \
       -H 'Content-Type: application/json' -H "X-Token: $TOKEN" \
       -d "$body" -o "$OUTDIR/$name.json"; then
    sz=$(wc -c < "$OUTDIR/$name.json" | tr -d ' ')
    echo "OK (${sz} bytes)"
  else
    echo "SKIPPED"
  fi
done

rm -f "$CJ" /tmp/klogin.json
echo "-------------------------------------------------------------"
echo "Done. Files in: $OUTDIR/"
echo "Next: I'll read $OUTDIR/alert.json to confirm the line format,"
echo "then finalize the 2-week filter + Loki import."

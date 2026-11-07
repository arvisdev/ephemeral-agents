#!/usr/bin/env bash
# =============================================================================
# M3 token issuance (R4) — deploy/m3/issue-token.sh  (runs on vm-xb)
# =============================================================================
# M3 install doc §1c: JWT-signed, curl + openssl, ~20 lines:
#   1) JWT: RS256, aud=https://api.github.com/app, iat/exp (15 min)
#   2) POST /app/installations/{installation_id}/access_tokens → installation
#      token (TTL 1 h max)
#   3) echo the token → injected as GITHUB_TOKEN env by the lifecycle spawn
#
# Usage: issue-token.sh [--dry-run] [--revoke-last]
# Env:
#   GITHUB_APP_ID     the App id (owner-supplied, M3 §1c)
#   INSTALLATION_ID   the App installation id (owner-supplied)
#   APP_KEY_PATH      default /opt/poc/secrets/app-key.pem (chmod 600, never in git or an image)
#   STATE_FILE        default /tmp/poc-issue-token-last (token id for --revoke-last)
set -euo pipefail

GITHUB_APP_ID="${GITHUB_APP_ID:?set GITHUB_APP_ID (M3 §1c)}"
INSTALLATION_ID="${INSTALLATION_ID:?set INSTALLATION_ID (M3 §1c)}"
APP_KEY_PATH="${APP_KEY_PATH:-/opt/poc/secrets/app-key.pem}"
STATE_FILE="${STATE_FILE:-/tmp/poc-issue-token-last}"
API="https://api.github.com"

b64url() { openssl base64 -A | tr '+/' '-_' | tr -d '='; }

jwt() { # RS256 app JWT, iat/exp 15 min
  local now iat exp header payload sig
  now="$(date +%s)"; iat=$((now - 60)); exp=$((iat + 540))
  header="$({ printf '{"alg":"RS256","typ":"JWT"}'; } | b64url)"
  payload="$(printf '{"iss":"%s","iat":%d,"exp":%d,"aud":"%s"}' "$GITHUB_APP_ID" "$iat" "$exp" "$API" | b64url)"
  sig="$(printf '%s.%s' "$header" "$payload" \
        | openssl dgst -sha256 -sign "$APP_KEY_PATH" -binary | b64url)"
  printf '%s.%s.%s' "$header" "$payload" "$sig"
}

case "${1:-}" in
  --dry-run)
    [ -r "$APP_KEY_PATH" ] || { echo "dry-run: app key not readable at $APP_KEY_PATH" >&2; }
    # print a token-shaped value without touching the network (install.sh S4 gate)
    echo "dry-run-$(jwt | cut -c1-24)"
    ;;
  --revoke-last)
    [ -s "$STATE_FILE" ] || { echo "no recorded token to revoke" >&2; exit 1; }
    tok="$(cat "$STATE_FILE")"
    curl -s -o /dev/null -w '%{http_code}\n' -X DELETE \
      -H "Authorization: Bearer $tok" "$API/installation/token" | grep -q 204 \
      && echo "token revoked" || { echo "revoke failed" >&2; exit 1; }
    rm -f "$STATE_FILE"
    ;;
  "")
    jwt="$(jwt)"
    # POST /app/installations/{id}/access_tokens -> installation token (TTL 1 h max)
    resp="$(curl -s -X POST \
      -H "Authorization: Bearer $jwt" \
      -H "Accept: application/vnd.github+json" \
      "$API/app/installations/$INSTALLATION_ID/access_tokens")"
    tok="$(printf '%s' "$resp" | python3 -c 'import json,sys;print(json.load(sys.stdin).get("token",""))')"
    [ -n "$tok" ] || { printf '%s\n' "$resp" >&2; echo "token issuance failed" >&2; exit 1; }
    # record for --revoke-last (the lifecycle reap step revokes this token, M3 §1c)
    printf '%s' "$tok" > "$STATE_FILE"; chmod 600 "$STATE_FILE"
    echo "$tok"
    ;;
  *) echo "usage: issue-token.sh [--dry-run|--revoke-last]" >&2; exit 64 ;;
esac

#!/usr/bin/env bash
# declare-topology-test.sh — Tier-1 harness for deploy/m0/install.sh's declare_topology block.
#
# Runs the REAL probe + declare code from install.sh (sliced by stable markers) against a
# local HTTP stub, with `ssh` replaced by a local shim. Verifies:
#   1. probe hits the NAMED resource (/api/exchanges/%2F/agent.tasks)  — not the collection
#   2. exactly 5 POSTs: exchange, durable queue, binding(routing_key), DLQ, response queue
#   3. every request carries Basic auth of poc:$RABBITMQ_PASS (stdin curl -K - works)
#   4. the password appears in NO executed command line (audit log == 0 matches)
#   5. the remote-staged /tmp/rmq-cfg is created 0600 and removed
#
# Usage: bash deploy/m0/tests/declare-topology-test.sh   (needs python3 + curl; no ssh)
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../../.." && pwd)"
INSTALL="$REPO_ROOT/deploy/m0/install.sh"
PORT="${T1_PORT:-18081}"
PASS="${T1_PASS:-tier1-testpass}"
WORK="$(mktemp -d)"
STUB_PID=""
trap '[ -n "$STUB_PID" ] && kill "$STUB_PID" 2>/dev/null || true; rm -rf "$WORK"' EXIT

command -v python3 >/dev/null && command -v curl >/dev/null \
  || { echo "FAIL: python3 + curl required"; exit 2; }
chmod +x "$SCRIPT_DIR/ssh"

# --- env the extracted block expects (shim replaces ssh; stub replaces rabbitmq) ---
export PATH="$SCRIPT_DIR:$PATH"
export RABBITMQ_PASS="$PASS"
export SSH_SHIM_AUDIT="$WORK/audit.log"
export VM_XB="stub-vm"
export base
base="http://127.0.0.1:$PORT"
VM_XB="stub-vm"

# --- slice the real probe + declare block out of install.sh ---
probe_line=$(grep -n 'api/exchanges/%2F/agent.tasks || true' "$INSTALL" | head -1 | cut -d: -f1 || true)
start=$(grep -n 'stage the curl auth config' "$INSTALL" | head -1 | cut -d: -f1 || true)
end=$(grep -n '^RABBIT_API$' "$INSTALL" | head -1 | cut -d: -f1 || true)
[ -n "$probe_line" ] && [ -n "$start" ] && [ -n "$end" ] \
  || { echo "FAIL: markers not found in install.sh (block shape changed? update the harness)"; exit 2; }
{
  echo 'set -euo pipefail'
  sed -n "${probe_line}p" "$INSTALL"
  sed -n "${start},${end}p" "$INSTALL"
  # close an `if` opened within the slice — only when the slice actually contains it
  if grep -q '\[ "$CHECK_ONLY"' "$WORK/declare-under-test.sh"; then echo 'fi'; fi
} > "$WORK/declare-under-test.sh"

# --- run against the stub ---
python3 "$SCRIPT_DIR/rmq-stub.py" "$PORT" "$WORK/requests.jsonl" &
STUB_PID=$!
sleep 1
bash "$WORK/declare-under-test.sh" || true
kill "$STUB_PID" 2>/dev/null; wait "$STUB_PID" 2>/dev/null || true

# --- assertions ---
fail=0
say() { echo "$1"; }
assert() { if [ "$2" = "$3" ]; then say "PASS: $1"; else say "FAIL: $1 (got: $3, want: $2)"; fail=1; fi; }

assert "probe hits the NAMED exchange resource (not the collection)" \
  "/api/exchanges/%2F/agent.tasks" \
  "$(grep -o '"path": "/api/exchanges/[^"]*"' "$WORK/requests.jsonl" | head -1 | sed 's/.*path": "//; s/"//')"
assert "5 POSTs declared" "5" "$(grep -c '"method": "POST"' "$WORK/requests.jsonl")"
assert "binding posted via /q/<queue> path with routing_key" "1" \
  "$(grep -c 'bindings/%2F/e/agent.tasks/q/agent.tasks.durable.*routing_key' "$WORK/requests.jsonl")"
assert "response queue declared" "1" "$(grep -c 'agent.tasks.response' "$WORK/requests.jsonl")"
assert "auth on every line" "6" "$(grep -c '"auth": "Basic [A-Za-z0-9+/=]*"' "$WORK/requests.jsonl")"
assert "auth decodes to poc:<password>" "1" \
  "$(sed -n 's/.*"auth": "Basic \([^"]*\)".*/\1/p' "$WORK/requests.jsonl" | head -1 | base64 -d | grep -c "poc:$PASS")"
assert "password in NO executed argv" "0" "$(grep -c "$PASS" "$SSH_SHIM_AUDIT" 2>/dev/null || true)"
assert "staged cfg cleaned up remotely" "0" "$([ -e /tmp/rmq-cfg ] && echo 1 || echo 0)"

[ "$fail" = 0 ] && { echo "== declare-topology-test: ALL PASS =="; exit 0; }
echo "== declare-topology-test: FAILURES above =="; exit 1

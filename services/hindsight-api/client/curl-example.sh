#!/usr/bin/env bash
# curl examples for the Hindsight memory API (M4 §1b deliverable).
# Run from any host that can resolve vm-hindsight, or with HINDSIGHT_URL set.
set -euo pipefail
HINDSIGHT_URL="${HINDSIGHT_URL:-http://vm-hindsight:8081}"

echo "== healthz =="
curl -s "$HINDSIGHT_URL/healthz"; echo

echo "== write (cross-task: visible to later tasks) =="
curl -s -XPOST "$HINDSIGHT_URL/memory/write" \
  -H 'Content-Type: application/json' \
  -d '{"task_id":"t1","agent_ref":"agents/coder","scope":"cross-task",
       "body":{"summary":"deploy X with blue-green, not in-place","learning":"in-place upgrades break service Y"}}'
echo

echo "== write (session: private to this run) =="
curl -s -XPOST "$HINDSIGHT_URL/memory/write" \
  -H 'Content-Type: application/json' \
  -d '{"task_id":"t1","agent_ref":"agents/coder","scope":"session",
       "body":{"summary":"scratch note: build takes ~4min"}}'
echo

echo "== read back cross-task from a LATER task (G2 cross-task read-back) =="
curl -s "$HINDSIGHT_URL/memory/read?task_id=t2&scope=cross-task&q=deploy"
echo

echo "== read own session rows (t1 sees its scratch note) =="
curl -s "$HINDSIGHT_URL/memory/read?task_id=t1&scope=session"
echo

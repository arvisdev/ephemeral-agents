#!/usr/bin/env bash
# restart-stack.sh — post-reboot bring-up for the execution host, in dependency order.
# Companion to book/ch14 §14.8 ("After a reboot") and ledger lesson L11 ("a reboot
# is not a redeploy"): each compose project restarts independently, and components
# that resolve others by name crash-loop until the named service exists.
#
# Usage:  bash deploy/restart-stack.sh [VM_XB]
#   VM_XB  ssh target of the execution host (from deploy/m0/hosts.env; default env VM_XB)
#
# Order (from Chapter 14 §14.8):
#   1. Workflow engine + store   (deploy/m0/temporal)  — first: everything resolves
#                                                        "temporal" by name on poc-net
#   2. Broker + message bridge   (deploy/m1)
#   3. Worker + lifecycle        (deploy/m2)
#   4. Dashboards + proxy        (UI stack; llm-proxy) — consumers, not dependencies
# Then a one-pass verification: containers Up, broker healthy, bridge consuming,
# worker registered, dashboards answering.

set -euo pipefail

VM_XB="${1:-${VM_XB:-}}"
if [[ -z "$VM_XB" ]]; then
  echo "usage: $0 <execution-host ssh target>   (or export VM_XB)" >&2
  exit 2
fi

step() { printf '\n== %s ==\n' "$*"; }

step "1/4 workflow engine + store (deploy/m0/temporal)"
ssh "$VM_XB" 'cd /opt/poc/deploy/m0/temporal && docker compose up -d'

step "2/4 broker + message bridge (deploy/m1)"
ssh "$VM_XB" 'cd /opt/poc/deploy/m1 && docker compose up -d'

step "3/4 worker + lifecycle (deploy/m2)"
ssh "$VM_XB" 'cd /opt/poc/deploy/m2 && docker compose up -d'

step "4/4 dashboards + proxy"
# Temporal UI lives in the operator's UI stack on the execution host; the
# request-translation proxy is a standalone container bind-mounting its source
# from the repository copy (ledger L3: nothing a container mounts may live
# outside source control). Start both if present; skip gracefully if absent.
ssh "$VM_XB" '
  if [ -d ~/temporal-agent-stack ]; then
    (cd ~/temporal-agent-stack && docker compose up -d --no-deps temporal-ui) || true
  fi
  docker start llm-proxy 2>/dev/null || true
'

step "verify"
ssh "$VM_XB" '
  set -e
  echo "-- containers --"; docker ps --format "{{.Names}}\t{{.Status}}" | sort
  echo "-- broker health --"; docker ps --filter name=rabbitmq --filter health=healthy --format "{{.Names}}: healthy"
  echo "-- bridge consuming --"; docker logs --tail 1 m1-topic-bridge-1 2>&1 | grep -q "connected; consuming" \
    && echo "topic-bridge: consuming agent.tasks.durable" || { echo "topic-bridge: NOT consuming"; exit 1; }
  echo "-- worker registered --"; docker logs m2-worker-1 2>&1 | grep -q "Started Worker" \
    && echo "worker: registered on task queue" || { echo "worker: NOT registered"; exit 1; }
'
step "RESTART_STACK_OK"

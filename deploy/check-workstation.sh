#!/usr/bin/env bash
# check-workstation.sh — verify the build workstation is ready for the POC.
#
# Run this BEFORE deploy/m0/install.sh. It checks every prerequisite the
# install scripts assume (see deploy/README.md, "First time on this POC?"):
#   - local tools: docker (+ buildx), go, temporal CLI, gh, rsync
#   - SSH key auth as user 'poc' to PVE_HOST and the four POC VMs
#   - passwordless sudo on those hosts (ssh poc@host 'sudo -n true')
# Exits non-zero on the first hard failure; exits 0 when everything checked.
#
# Usage:
#   bash deploy/check-workstation.sh            # full check
#   bash deploy/check-workstation.sh --help     # this text
set -euo pipefail

if [[ "${1:-}" == "-h" || "${1:-}" == "--help" ]]; then
  sed -n '2,15p' "$0" | sed 's/^# \{0,1\}//'
  exit 0
fi

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
HOSTS_ENV="$REPO_ROOT/deploy/m0/hosts.env"

fail() { echo "FAIL: $*" >&2; exit 1; }
ok()   { echo "PASS: $*"; }

# --- local tools -------------------------------------------------------------
command -v docker >/dev/null 2>&1 || fail "docker not found on PATH — install Docker (see deploy/README.md)"
docker buildx version >/dev/null 2>&1 || fail "docker buildx not available — install the buildx plugin"
ok "docker + buildx present"

command -v go >/dev/null 2>&1 || fail "go not found on PATH — install a Go toolchain (M0 requires >= 1.25)"
ok "go present ($(go version 2>/dev/null || echo 'version check skipped'))"

command -v temporal >/dev/null 2>&1 || fail "temporal CLI not found — curl -sSL https://cli.temporal.io/install.sh | bash"
ok "temporal CLI present"

command -v gh >/dev/null 2>&1 || fail "gh not found on PATH — install the GitHub CLI and log in (gh auth login)"
ok "gh present"

command -v rsync >/dev/null 2>&1 || fail "rsync not found on PATH — the install scripts use rsync to stage deploy/"
ok "rsync present"

# --- hosts.env ---------------------------------------------------------------
if [[ ! -f "$HOSTS_ENV" ]]; then
  echo "SKIP: $HOSTS_ENV not found — copy deploy/m0/hosts.env.example to"
  echo "      deploy/m0/hosts.env and fill it in, then re-run this script."
  echo "      (Tool checks above still ran; SSH/sudo checks need hosts.env.)"
  exit 0
fi
# shellcheck disable=SC1090
source "$HOSTS_ENV"

ALL_HOSTS=("$PVE_HOST" "$VM_XA" "$VM_XB" "$VM_2" "$VM_3")

# --- ssh key auth as poc -----------------------------------------------------
for host in "${ALL_HOSTS[@]}"; do
  # PVE host is reached as root-on-host per the M0 doc; VMs as poc.
  if [[ "$host" == "$PVE_HOST" ]]; then
    user="$PVE_HOST"; target="$PVE_HOST"
  else
    target="poc@$host"
  fi
  ssh -o BatchMode=yes -o ConnectTimeout=10 "$target" true 2>/dev/null \
    || fail "ssh key auth to '$target' failed — authorize your public key for that host (deploy/README.md)"
  ok "ssh key auth to $target"
  # --- passwordless sudo (the VMs need it: netplan in m0, /etc/hosts in m4) ---
  if [[ "$host" != "$PVE_HOST" ]]; then
    ssh -o BatchMode=yes -o ConnectTimeout=10 "poc@$host" 'sudo -n true' 2>/dev/null \
      || fail "poc@$host lacks passwordless sudo — 'echo \"poc ALL=(ALL) NOPASSWD:ALL\" | sudo tee /etc/sudoers.d/poc'"
    ok "passwordless sudo on poc@$host"
  fi
done

echo "== workstation check: ALL PASS =="

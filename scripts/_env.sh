#!/usr/bin/env bash
# Sourced by every other script in this directory. Loads user config from
# Makefile.env and the IPs from terraform/.env.json (written by tf-apply.sh).
# Sets STORAGE_PUB/CCAAS_PUB/BENCH_PUB and the matching *_PRIV variants.

set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$REPO_ROOT"

if [[ ! -f Makefile.env ]]; then
  echo "missing Makefile.env — copy Makefile.env.example and edit it" >&2
  exit 1
fi
# shellcheck disable=SC1091
source Makefile.env

: "${SSH_KEY:?SSH_KEY not set in Makefile.env}"
: "${YCSB_DIR:?YCSB_DIR not set in Makefile.env}"

# Resolve the SSH key path (handle ~ in case the user wrote it literally).
# Existence is checked lazily in require_ssh_key — build.sh doesn't need it.
SSH_KEY="${SSH_KEY/#\~/$HOME}"

# YCSB_DIR may be relative to repo root. Resolve once.
if [[ ! "$YCSB_DIR" = /* ]]; then
  YCSB_DIR="$(cd "$REPO_ROOT/$YCSB_DIR" 2>/dev/null && pwd || true)"
fi
if [[ -z "${YCSB_DIR:-}" || ! -d "$YCSB_DIR" ]]; then
  echo "YCSB_DIR does not exist: ${YCSB_DIR:-<empty>}" >&2
  exit 1
fi

TF_OUT="$REPO_ROOT/terraform/.env.json"

require_ssh_key() {
  if [[ ! -f $SSH_KEY ]]; then
    echo "SSH key not found at $SSH_KEY (set SSH_KEY in Makefile.env)" >&2
    exit 1
  fi
}

# load_ips: populate STORAGE_*, CCAAS_*, BENCH_* from terraform output.
# Required for every target except doctor/tf-apply/build.
load_ips() {
  require_ssh_key
  if [[ ! -f "$TF_OUT" ]]; then
    echo "missing $TF_OUT — run 'make tf-apply' first" >&2
    exit 1
  fi
  STORAGE_PUB=$(jq -er '.scp_targets.value."storage-vm"'    "$TF_OUT")
  CCAAS_PUB=$(jq -er   '.scp_targets.value."ccaas-vm"'      "$TF_OUT")
  BENCH_PUB=$(jq -er   '.scp_targets.value."bench-vm"'      "$TF_OUT")
  STORAGE_PRIV=$(jq -er '.storage_private_ip.value'         "$TF_OUT")
  CCAAS_PRIV=$(jq -er   '.ccaas_private_ip.value'           "$TF_OUT")
  BENCH_PRIV=$(jq -er   '.bench_private_ip.value'           "$TF_OUT")
  export STORAGE_PUB CCAAS_PUB BENCH_PUB STORAGE_PRIV CCAAS_PRIV BENCH_PRIV
}

# Standard ssh/scp options — strict host checking off because EC2 host keys
# rotate when instances are recreated.
SSH_OPTS=(-i "$SSH_KEY" -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -o LogLevel=ERROR)

ssh_to() {
  local host=$1; shift
  ssh "${SSH_OPTS[@]}" "ubuntu@${host}" "$@"
}

scp_to() {
  scp "${SSH_OPTS[@]}" "$@"
}

export REPO_ROOT YCSB_DIR SSH_KEY TF_OUT

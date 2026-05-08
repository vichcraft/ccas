#!/usr/bin/env bash
# Pull results CSVs and server logs back to ./results/.

set -euo pipefail
source "$(dirname "$0")/_env.sh"
load_ips
mkdir -p "$REPO_ROOT/results"

rsync -e "ssh ${SSH_OPTS[*]}" -av "ubuntu@${BENCH_PUB}:results/" "$REPO_ROOT/results/" || true
ssh_to "$STORAGE_PUB" "cat storage.log 2>/dev/null" > "$REPO_ROOT/results/storage.log" || true
ssh_to "$CCAAS_PUB"   "cat ccaas.log   2>/dev/null" > "$REPO_ROOT/results/ccaas.log"   || true

echo
echo "Pulled to $REPO_ROOT/results/:"
ls -lh "$REPO_ROOT/results/"

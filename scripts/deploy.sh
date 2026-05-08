#!/usr/bin/env bash
# scp binaries and workload property files to the three VMs in parallel.

set -euo pipefail
source "$(dirname "$0")/_env.sh"
load_ips
cd "$REPO_ROOT"

for f in build/storage-server build/ccaas-server build/go-ycsb build/go-ycsb-txn; do
  [[ -f $f ]] || { echo "missing $f — run 'make build' first" >&2; exit 1; }
done

echo "Deploying to:"
echo "  storage-vm = $STORAGE_PUB"
echo "  ccaas-vm   = $CCAAS_PUB"
echo "  bench-vm   = $BENCH_PUB"

(
  # Linux refuses to overwrite a running binary (ETXTBSY); stop first.
  ssh_to "$STORAGE_PUB" "pkill -x storage-server 2>/dev/null || true; sleep 0.3"
  scp_to build/storage-server "ubuntu@${STORAGE_PUB}:~/" \
    && ssh_to "$STORAGE_PUB" "chmod +x storage-server"
) &
S=$!

(
  ssh_to "$CCAAS_PUB" "pkill -x ccaas-server 2>/dev/null || true; sleep 0.3"
  scp_to build/ccaas-server "ubuntu@${CCAAS_PUB}:~/" \
    && ssh_to "$CCAAS_PUB" "chmod +x ccaas-server"
) &
C=$!

(
  scp_to build/go-ycsb build/go-ycsb-txn "ubuntu@${BENCH_PUB}:~/"
  scp_to -r "$YCSB_DIR/workloads" "ubuntu@${BENCH_PUB}:~/"
  ssh_to "$BENCH_PUB" "chmod +x go-ycsb go-ycsb-txn && mkdir -p results"
) &
B=$!

wait $S $C $B
echo "Deploy OK."

#!/usr/bin/env bash
# Drive the full experiment matrix from the laptop. For each cell:
#   - if --epoch-ms differs from previous, restart ccaas-server
#   - run go-ycsb-txn on bench-vm, appending to results/all.csv on bench-vm
#
# Each row schema (workload distribution threads ops_per_txn epoch_ms read_prop tag):
CELLS=(
  # === Phase D follow-up: epoch sweep below the RTT threshold (3) ===
  # Measured RTT ~4.2 ms; epoch=10ms lost. Try epochs < RTT to find the
  # cross-over and show both sides of the paper's argument.
  "ycsba zipfian 32 10 1 0.5 base-A-zip-ep1"
  "ycsba zipfian 32 10 2 0.5 base-A-zip-ep2"
  "ycsba zipfian 32 10 5 0.5 base-A-zip-ep5"
)

set -euo pipefail
source "$(dirname "$0")/_env.sh"
load_ips

OPERATIONS=${OPERATIONS:-1000000}
RECORDS=${RECORDS:-1000000}

current_epoch=""

for cell in "${CELLS[@]}"; do
  read -r WORKLOAD DIST THREADS OPS_PER_TXN EPOCH_MS READ_PROP TAG <<<"$cell"

  if [[ "$EPOCH_MS" != "$current_epoch" ]]; then
    echo
    echo "=== switching ccaas-server to --epoch-ms=$EPOCH_MS ==="
    "$REPO_ROOT/scripts/restart-ccaas.sh" "$EPOCH_MS"
    current_epoch="$EPOCH_MS"
  fi

  echo
  echo "=== cell: $TAG  (workload=$WORKLOAD dist=$DIST threads=$THREADS ops/txn=$OPS_PER_TXN epoch=$EPOCH_MS) ==="
  ssh_to "$BENCH_PUB" "
    ./go-ycsb-txn \
      --storage-addr ${STORAGE_PRIV}:50051 --ccaas-addr ${CCAAS_PRIV}:50052 \
      --records ${RECORDS} --operations ${OPERATIONS} --threads ${THREADS} \
      --ops-per-txn ${OPS_PER_TXN} \
      --read-proportion ${READ_PROP} --distribution ${DIST} \
      --workload ${WORKLOAD} --epoch-ms ${EPOCH_MS} \
      --run-tag ${TAG} --results-csv results/all.csv 2>&1 | tail -3
  "
done

echo
echo "Matrix done. Pull results with: make pull"

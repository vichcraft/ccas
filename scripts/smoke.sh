#!/usr/bin/env bash
# Single go-ycsb-txn cell to verify wiring before running the full matrix.

set -euo pipefail
source "$(dirname "$0")/_env.sh"
load_ips

RECORDS=${RECORDS:-10000}
OPS=${OPS:-50000}
THREADS=${THREADS:-8}

echo "Smoke: ycsba zipfian, threads=$THREADS, ops=$OPS, immediate mode"
ssh_to "$BENCH_PUB" "
  ./go-ycsb-txn \
    --storage-addr ${STORAGE_PRIV}:50051 --ccaas-addr ${CCAAS_PRIV}:50052 \
    --records ${RECORDS} --operations ${OPS} --threads ${THREADS} \
    --read-proportion 0.5 --distribution zipfian \
    --workload ycsba --epoch-ms 0 \
    --run-tag smoke --results-csv results/smoke.csv 2>&1 | tail -14
"

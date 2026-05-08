#!/usr/bin/env bash
# Bulk-load YCSB records into storage. Defaults to 1M records.
# Usage: load.sh [recordcount]

set -euo pipefail
source "$(dirname "$0")/_env.sh"
load_ips

RECORDS=${1:-1000000}
THREADS=${LOAD_THREADS:-32}

echo "Loading $RECORDS records via go-ycsb on bench-vm..."
ssh_to "$BENCH_PUB" "
  ./go-ycsb load ccaas \
    -P workloads/ccaas_workloada_zipfian \
    -P workloads/ccaas_config.properties \
    -p ccaas.storage_addr=${STORAGE_PRIV}:50051 \
    -p ccaas.cc_addr=${CCAAS_PRIV}:50052 \
    -p recordcount=${RECORDS} -p insertcount=${RECORDS} \
    --threads ${THREADS} 2>&1 | tail -3
"

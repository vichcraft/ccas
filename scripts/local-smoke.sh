#!/usr/bin/env bash
# Runs the bank-transfer demo client against the locally-running servers.
# Demonstrates the full 3-process flow end-to-end.
set -euo pipefail

cd "$(dirname "$0")/.."

STORAGE_PORT="${STORAGE_PORT:-50051}"
CCAAS_PORT="${CCAAS_PORT:-50052}"

if ! lsof -iTCP:"$STORAGE_PORT" -sTCP:LISTEN >/dev/null 2>&1; then
  echo "ERROR: storage server not running on :$STORAGE_PORT. Run 'make local-run' first." >&2
  exit 1
fi
if ! lsof -iTCP:"$CCAAS_PORT" -sTCP:LISTEN >/dev/null 2>&1; then
  echo "ERROR: ccaas server not running on :$CCAAS_PORT. Run 'make local-run' first." >&2
  exit 1
fi

./build/local/demo \
  -mode=remote \
  -storage-addr="localhost:$STORAGE_PORT" \
  -ccaas-addr="localhost:$CCAAS_PORT"

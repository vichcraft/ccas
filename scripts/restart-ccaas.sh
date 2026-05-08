#!/usr/bin/env bash
# Restart ccaas-server on ccaas-vm with a new --epoch-ms value.
# Usage: restart-ccaas.sh <epoch_ms>

set -euo pipefail
source "$(dirname "$0")/_env.sh"
load_ips

EPOCH_MS=${1:?usage: restart-ccaas.sh <epoch_ms>}

ssh_to "$CCAAS_PUB" "
  [ -f srv.pid ] && kill \$(cat srv.pid) 2>/dev/null
  pkill -x ccaas-server 2>/dev/null || true
  sleep 0.3
  nohup ./ccaas-server --port 50052 --storage-addr ${STORAGE_PRIV}:50051 --epoch-ms ${EPOCH_MS} < /dev/null > ccaas.log 2>&1 &
  echo \$! > srv.pid
  exit 0
"
sleep 1
ssh_to "$CCAAS_PUB" "
  if pgrep -x ccaas-server > /dev/null; then
    echo \"ccaas-server up (--epoch-ms ${EPOCH_MS})\"
  else
    echo 'ccaas-server FAILED. Last 20 lines of log:'
    tail -20 ccaas.log 2>/dev/null
    exit 1
  fi
"

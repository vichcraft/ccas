#!/usr/bin/env bash
# Launch storage-server and ccaas-server in the background on their VMs.
# Default ccaas mode: immediate (--epoch-ms 0). matrix.sh restarts it as needed.

set -euo pipefail
source "$(dirname "$0")/_env.sh"
load_ips

# When you ssh and run `nohup foo &`, ssh will sometimes hang or exit 255
# waiting for the backgrounded process's file descriptors to close. The
# fix is to (a) redirect stdin from /dev/null too, (b) end the remote
# command with an explicit `exit 0`, and (c) verify in a second ssh call.

echo "Starting storage-server on $STORAGE_PUB..."
ssh_to "$STORAGE_PUB" "
  [ -f srv.pid ] && kill \$(cat srv.pid) 2>/dev/null
  pkill -x storage-server 2>/dev/null || true
  sleep 0.3
  nohup ./storage-server --port 50051 < /dev/null > storage.log 2>&1 &
  echo \$! > srv.pid
  exit 0
"
sleep 1
ssh_to "$STORAGE_PUB" "
  if pgrep -x storage-server > /dev/null; then
    echo 'storage-server up'
  else
    echo 'storage-server FAILED. Last 20 lines of log:'
    tail -20 storage.log 2>/dev/null
    exit 1
  fi
"

echo "Starting ccaas-server on $CCAAS_PUB (immediate mode)..."
ssh_to "$CCAAS_PUB" "
  [ -f srv.pid ] && kill \$(cat srv.pid) 2>/dev/null
  pkill -x ccaas-server 2>/dev/null || true
  sleep 0.3
  nohup ./ccaas-server --port 50052 --storage-addr ${STORAGE_PRIV}:50051 --epoch-ms 0 < /dev/null > ccaas.log 2>&1 &
  echo \$! > srv.pid
  exit 0
"
sleep 1
ssh_to "$CCAAS_PUB" "
  if pgrep -x ccaas-server > /dev/null; then
    echo 'ccaas-server up'
  else
    echo 'ccaas-server FAILED. Last 20 lines of log:'
    tail -20 ccaas.log 2>/dev/null
    exit 1
  fi
"

echo
echo "Smoke test: bench-vm -> servers..."
ssh_to "$BENCH_PUB" "nc -zv ${STORAGE_PRIV} 50051 && nc -zv ${CCAAS_PRIV} 50052" 2>&1 | tail -2
echo "Servers ready."

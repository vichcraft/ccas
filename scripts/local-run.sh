#!/usr/bin/env bash
# Starts the storage server and CCaaS server locally on the host machine.
# Logs go to ./local-logs/, PIDs to ./local-logs/*.pid.
set -euo pipefail

cd "$(dirname "$0")/.."

LOG_DIR="local-logs"
mkdir -p "$LOG_DIR"

STORAGE_PORT="${STORAGE_PORT:-50051}"
CCAAS_PORT="${CCAAS_PORT:-50052}"
EPOCH_MS="${EPOCH_MS:-0}"

if lsof -iTCP:"$STORAGE_PORT" -sTCP:LISTEN >/dev/null 2>&1; then
  echo "ERROR: port $STORAGE_PORT already in use. Run 'make local-stop' first." >&2
  exit 1
fi
if lsof -iTCP:"$CCAAS_PORT" -sTCP:LISTEN >/dev/null 2>&1; then
  echo "ERROR: port $CCAAS_PORT already in use. Run 'make local-stop' first." >&2
  exit 1
fi

echo ">> starting storage-server on :$STORAGE_PORT"
./build/local/storage-server -port "$STORAGE_PORT" \
  >"$LOG_DIR/storage.log" 2>&1 &
echo $! >"$LOG_DIR/storage.pid"

# Give storage a moment to bind before ccaas dials it.
sleep 1

echo ">> starting ccaas-server on :$CCAAS_PORT (storage=localhost:$STORAGE_PORT, epoch-ms=$EPOCH_MS)"
./build/local/ccaas-server \
  -port "$CCAAS_PORT" \
  -storage-addr "localhost:$STORAGE_PORT" \
  -epoch-ms "$EPOCH_MS" \
  >"$LOG_DIR/ccaas.log" 2>&1 &
echo $! >"$LOG_DIR/ccaas.pid"

sleep 1

# Verify both are still alive.
for name in storage ccaas; do
  pid=$(cat "$LOG_DIR/$name.pid")
  if ! kill -0 "$pid" 2>/dev/null; then
    echo "ERROR: $name-server failed to start. Tail of log:" >&2
    tail -n 20 "$LOG_DIR/$name.log" >&2
    exit 1
  fi
done

echo
echo "Both servers running."
echo "  storage-server  pid=$(cat "$LOG_DIR/storage.pid")  log=$LOG_DIR/storage.log"
echo "  ccaas-server    pid=$(cat "$LOG_DIR/ccaas.pid")    log=$LOG_DIR/ccaas.log"
echo
echo "Next: 'make local-smoke' to run a transaction workload against them."
echo "Stop with: 'make local-stop'"

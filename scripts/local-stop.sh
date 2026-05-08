#!/usr/bin/env bash
# Stops the local storage-server and ccaas-server started by local-run.sh.
set -euo pipefail

cd "$(dirname "$0")/.."

LOG_DIR="local-logs"

stop_one() {
  local name="$1"
  local pidfile="$LOG_DIR/$name.pid"
  if [[ ! -f "$pidfile" ]]; then
    echo "no pidfile for $name (already stopped?)"
    return
  fi
  local pid
  pid=$(cat "$pidfile")
  if kill -0 "$pid" 2>/dev/null; then
    echo ">> stopping $name (pid=$pid)"
    kill "$pid" 2>/dev/null || true
    # Give it a beat to exit gracefully.
    for _ in 1 2 3 4 5; do
      kill -0 "$pid" 2>/dev/null || break
      sleep 0.2
    done
    kill -9 "$pid" 2>/dev/null || true
  else
    echo "$name (pid=$pid) not running"
  fi
  rm -f "$pidfile"
}

stop_one ccaas
stop_one storage
echo "done."

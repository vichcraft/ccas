#!/usr/bin/env bash
# Cross-compile all four binaries for Linux/amd64 into ./build/.

set -euo pipefail
source "$(dirname "$0")/_env.sh"
cd "$REPO_ROOT"
mkdir -p build

export GOOS=linux GOARCH=amd64 CGO_ENABLED=0

echo "[1/4] storage-server"
go build -o build/storage-server ./cmd/storage-server

echo "[2/4] ccaas-server"
go build -o build/ccaas-server   ./cmd/ccaas-server

echo "[3/4] go-ycsb (from $YCSB_DIR)"
( cd "$YCSB_DIR" && go build -o "$REPO_ROOT/build/go-ycsb" ./cmd/go-ycsb )

echo "[4/4] go-ycsb-txn (from $YCSB_DIR)"
( cd "$YCSB_DIR" && go build -o "$REPO_ROOT/build/go-ycsb-txn" ./cmd/go-ycsb-txn )

echo
ls -lh build/

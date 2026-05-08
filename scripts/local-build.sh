#!/usr/bin/env bash
# Compiles the 3 binaries (storage-server, ccaas-server, demo client) for the
# host OS/arch into ./build/local/.
set -euo pipefail

cd "$(dirname "$0")/.."

OUT="build/local"
mkdir -p "$OUT"

echo ">> building storage-server"
go build -o "$OUT/storage-server" ./cmd/storage-server

echo ">> building ccaas-server"
go build -o "$OUT/ccaas-server" ./cmd/ccaas-server

echo ">> building demo client"
go build -o "$OUT/demo" ./cmd/demo

echo
echo "Built into $OUT/:"
ls -lh "$OUT"

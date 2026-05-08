#!/usr/bin/env bash
# Stop the storage and ccaas servers. Idempotent.

set -euo pipefail
source "$(dirname "$0")/_env.sh"
load_ips

ssh_to "$STORAGE_PUB" "pkill -x storage-server 2>/dev/null || true; echo 'storage stopped'"
ssh_to "$CCAAS_PUB"   "pkill -x ccaas-server   2>/dev/null || true; echo 'ccaas stopped'"

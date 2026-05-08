#!/usr/bin/env bash
# Quick health check before any destructive command.

set -euo pipefail
source "$(dirname "$0")/_env.sh"

echo "== Toolchain =="
for cmd in terraform go ssh scp rsync jq; do
  if command -v "$cmd" >/dev/null 2>&1; then
    printf "  %-10s %s\n" "$cmd" "$(command -v "$cmd")"
  else
    printf "  %-10s MISSING\n" "$cmd"
    exit 1
  fi
done

echo
echo "== Config =="
echo "  REPO_ROOT     = $REPO_ROOT"
echo "  YCSB_DIR      = $YCSB_DIR"
if [[ -f $SSH_KEY ]]; then
  echo "  SSH_KEY       = $SSH_KEY (ok)"
else
  echo "  SSH_KEY       = $SSH_KEY (MISSING — needed for deploy/start/etc)"
fi
echo "  Public IP     = $(curl -fsS4 ifconfig.me 2>/dev/null || echo unknown)"

echo
echo "== Terraform state =="
if [[ -f "$TF_OUT" ]]; then
  echo "  $TF_OUT exists"
  jq -r '.scp_targets.value | to_entries[] | "  \(.key) public=\(.value)"' "$TF_OUT"
else
  echo "  no terraform output captured yet — run 'make tf-apply'"
fi

echo
echo "OK"

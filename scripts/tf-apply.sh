#!/usr/bin/env bash
# Run terraform apply and snapshot output as JSON for the other scripts.

set -euo pipefail
source "$(dirname "$0")/_env.sh"

cd "$REPO_ROOT/terraform"
terraform init -input=false -upgrade=false >/dev/null
terraform apply -auto-approve

terraform output -json > .env.json
echo
echo "Captured $(realpath .env.json):"
jq -r '. | to_entries[] | select(.value.value != null) | "  \(.key) = \(.value.value)"' .env.json | head -20

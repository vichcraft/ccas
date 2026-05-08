#!/usr/bin/env bash
# Tear down the EC2 instances and security group.

set -euo pipefail
source "$(dirname "$0")/_env.sh"
cd "$REPO_ROOT/terraform"

terraform destroy -auto-approve
rm -f .env.json
echo "destroyed"

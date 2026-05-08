#!/usr/bin/env bash
# Capture intra-VPC TCP-connect RTT to the gRPC ports. ICMP is blocked by the
# security group, so we time TCP handshakes (which is closer to what gRPC pays
# anyway).

set -euo pipefail
source "$(dirname "$0")/_env.sh"
load_ips
mkdir -p "$REPO_ROOT/results"

# tcp_rtt: 50 sequential TCP handshakes from the source VM to host:port,
# print min/avg/max in ms. Uses bash's /dev/tcp under SECONDS to avoid relying
# on remote tools we haven't installed.
tcp_rtt_remote() {
  local from=$1 to_host=$2 to_port=$3
  ssh_to "$from" "
    samples=()
    for i in \$(seq 1 50); do
      t0=\$(date +%s%N)
      timeout 1 bash -c '</dev/tcp/${to_host}/${to_port}' 2>/dev/null || continue
      t1=\$(date +%s%N)
      samples+=(\$(( (t1 - t0) / 1000 )))   # microseconds
    done
    n=\${#samples[@]}
    if [ \$n -eq 0 ]; then echo 'no successful handshakes'; exit 1; fi
    min=\${samples[0]}; max=\${samples[0]}; sum=0
    for s in \${samples[@]}; do
      [ \$s -lt \$min ] && min=\$s
      [ \$s -gt \$max ] && max=\$s
      sum=\$((sum + s))
    done
    avg=\$((sum / n))
    awk -v mn=\$min -v av=\$avg -v mx=\$max -v n=\$n \
      'BEGIN { printf \"  n=%d  min=%.3f ms  avg=%.3f ms  max=%.3f ms\n\", n, mn/1000, av/1000, mx/1000 }'
  "
}

{
  echo "# Captured $(date -u +'%Y-%m-%d %H:%M:%S UTC')"
  echo "# TCP-connect RTT (50 samples per pair)"
  echo
  echo "# ccaas-vm -> storage-vm (${STORAGE_PRIV}:50051)"
  tcp_rtt_remote "$CCAAS_PUB" "$STORAGE_PRIV" 50051
  echo
  echo "# bench-vm -> ccaas-vm (${CCAAS_PRIV}:50052)"
  tcp_rtt_remote "$BENCH_PUB" "$CCAAS_PRIV" 50052
  echo
  echo "# bench-vm -> storage-vm (${STORAGE_PRIV}:50051)"
  tcp_rtt_remote "$BENCH_PUB" "$STORAGE_PRIV" 50051
} | tee "$REPO_ROOT/results/rtt.txt"

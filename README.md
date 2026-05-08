# CCaaS — Certification-as-a-Service Prototype

A Go implementation of the three-layer **Certification-as-a-Service** architecture from
_"Don't Look Back, Look into the Future: Certification-as-a-Service Transactions"_
(Aguilera et al.). The system disaggregates a transactional KV system into three independent
processes that communicate over gRPC:

| Layer     | Binary           | Default port | Responsibility                                              |
| --------- | ---------------- | ------------ | ----------------------------------------------------------- |
| Storage   | `storage-server` | `:50051`     | In-memory KV store, single source of truth                  |
| Certifier | `ccaas-server`   | `:50052`     | Optimistic concurrency control (immediate or epoch-batched) |
| Client    | `demo` (or YCSB) | —            | Runs transactions, talks to both layers                     |

The companion paper PDF is at [`CCaS_Paper.pdf`](CCaS_Paper.pdf) and a mapping from paper to
code lives in [`paper-vs-implementation.md`](paper-vs-implementation.md).

---

## Quick start (single-machine demo)

The fastest way to see the system end-to-end. Builds the three binaries, starts the two
servers as background processes, runs a multi-client bank-transfer workload through the
gRPC stack, then tears everything down.

**Prerequisites:** Go 1.25+ and `make`. Nothing else — the storage layer is in-memory, no
database, no Docker.

```bash
make local-all
```

Expected output ends with something like:

```
Running with REMOTE certification (storage=localhost:50051, ccaas=localhost:50052)...
  Committed: 327  Aborted: 473  Abort Rate: 59.1%
  Final balances: [1338 1626 806 827 936 812 1164 1709 71 711]
  Balance sum: 10000 (expected: 10000) -- PASS
```

The `Balance sum ... PASS` line is the correctness check: 8 concurrent clients perform 100
random transfers each (800 transactions total) across 10 accounts; conservation of money
is verified at the end. Aborts are expected — they are how the certifier rejects
non-serialisable interleavings.

---

## Step-by-step (if you want to inspect each process)

```bash
# 1. Compile storage-server, ccaas-server, demo into ./build/local/
make local-build

# 2. Launch the two servers in the background.
#    Logs go to ./local-logs/{storage,ccaas}.log, PIDs to *.pid.
make local-run

# 3. Run the demo client against the running servers.
make local-smoke

# 4. (Optional) inspect server logs.
tail -f local-logs/storage.log
tail -f local-logs/ccaas.log

# 5. Stop the servers.
make local-stop
```

### Running the unit tests

```bash
make local-test    # equivalent to: go test ./...
```

Covers the storage layer, the immediate and epoch certifiers, the gRPC wrappers, and the
execution engine.

### Trying epoch-batched certification

By default the CCaaS server runs in **immediate** mode (each transaction is certified on
arrival). To run the paper's **epoch** mode, set `EPOCH_MS` before `local-run`:

```bash
make local-stop                 # if servers are running
EPOCH_MS=10 make local-run      # 10 ms epoch window
make local-smoke
```

You should see the same `PASS` line — epoch mode trades latency for higher commit
throughput under contention.

---

## What's in the repo

```
cmd/
  storage-server/   # process 1 — in-memory KV over gRPC
  ccaas-server/     # process 2 — certifier (immediate or epoch)
  demo/             # process 3 — bank-transfer client (single-machine demo)
  benchmark/        # YCSB-style benchmark client (used in cloud experiments)
pkg/
  storage/          # MemStore + gRPC server/client
  ccaas/            # immediate & epoch certifiers + gRPC wrappers
  execution/        # transaction execution engine on the client side
  benchmark/        # workload generation
proto/ccaspb/       # gRPC service definitions
scripts/            # local-* helpers (this README) and cloud orchestration
terraform/          # GCP provisioning for the cloud benchmark (optional)
```

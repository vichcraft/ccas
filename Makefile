SHELL := /usr/bin/env bash

# All real work lives in scripts/. The Makefile is just a switchboard.
.PHONY: help doctor tf-apply build deploy start stop load smoke matrix pull rtt destroy all \
        local-build local-run local-stop local-smoke local-test local-all

help:
	@echo "CCaaS local workflow (single machine, no cloud)"
	@echo ""
	@echo "  make local-all    One-shot: build + run servers + smoke test + stop"
	@echo "  make local-build  Compile storage-server, ccaas-server, demo -> ./build/local/"
	@echo "  make local-run    Start storage-server + ccaas-server in background"
	@echo "  make local-smoke  Run the bank-transfer demo against the running servers"
	@echo "  make local-stop   Stop the local servers"
	@echo "  make local-test   Run the Go unit-test suite"
	@echo ""
	@echo "CCaaS YCSB cloud workflow"
	@echo ""
	@echo "  make doctor      Check toolchain, config, terraform state, public IP"
	@echo "  make tf-apply    terraform apply + capture IPs to terraform/.env.json"
	@echo "  make build       Cross-compile 4 binaries (Linux/amd64) -> ./build/"
	@echo "  make deploy      scp binaries + workloads/ to the 3 VMs (parallel)"
	@echo "  make start       Launch storage-server + ccaas-server (immediate mode)"
	@echo "  make load        Bulk-load 1M YCSB records into storage"
	@echo "  make smoke       One go-ycsb-txn cell to verify wiring"
	@echo "  make matrix      Run the full experiment sweep (~5-10 min)"
	@echo "  make rtt         Capture intra-VPC ping RTT to results/rtt.txt"
	@echo "  make pull        rsync results/ from bench-vm to laptop"
	@echo "  make stop        Kill the servers (idempotent)"
	@echo "  make destroy     terraform destroy + clear local state"
	@echo ""
	@echo "Happy path: make tf-apply build deploy start && make load matrix pull"

doctor:
	@bash scripts/doctor.sh

tf-apply:
	@bash scripts/tf-apply.sh

build:
	@bash scripts/build.sh

deploy:
	@bash scripts/deploy.sh

start:
	@bash scripts/start.sh

stop:
	@bash scripts/stop.sh

load:
	@bash scripts/load.sh $(RECORDS)

smoke:
	@bash scripts/smoke.sh

matrix:
	@bash scripts/matrix.sh

rtt:
	@bash scripts/rtt.sh

pull:
	@bash scripts/pull.sh

destroy:
	@bash scripts/destroy.sh

# Convenience: bring up everything from scratch.
all: tf-apply build deploy start

# ---- Local (single-machine) workflow ----------------------------------------

local-build:
	@bash scripts/local-build.sh

local-run:
	@bash scripts/local-run.sh

local-stop:
	@bash scripts/local-stop.sh

local-smoke:
	@bash scripts/local-smoke.sh

local-test:
	go test ./...

# One-shot demo path for graders: build, start servers, run smoke, tear down.
local-all: local-build local-run
	@bash scripts/local-smoke.sh; status=$$?; bash scripts/local-stop.sh; exit $$status

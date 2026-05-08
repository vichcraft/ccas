#!/usr/bin/env -S uv run --script
# /// script
# requires-python = ">=3.11"
# dependencies = [
#   "pandas>=2.2",
#   "matplotlib>=3.8",
#   "tabulate>=0.9",
# ]
# ///
"""
Read results/all.csv and emit:
  - results/plots/workload_modes.png       (bar: throughput per workload x mode)
  - results/plots/abort_rates.png          (bar: abort rate per workload x mode)
  - results/plots/client_scaling.png       (line: throughput + abort vs threads)
  - results/plots/ops_per_txn.png          (line: txn/s, ops/s, abort vs ops_per_txn)
  - results/plots/epoch_sweep.png          (line: throughput vs epoch_ms)
  - results/summary_tables.md              (markdown tables ready to paste)

Run with:  uv run scripts/parse_results.py
"""

from __future__ import annotations

import sys
from pathlib import Path

import pandas as pd
import matplotlib.pyplot as plt

REPO_ROOT = Path(__file__).resolve().parent.parent
CSV = REPO_ROOT / "results" / "all.csv"
PLOTS = REPO_ROOT / "results" / "plots"
TABLES = REPO_ROOT / "results" / "summary_tables.md"


def load() -> pd.DataFrame:
    if not CSV.exists():
        sys.exit(f"missing {CSV} — pull results first")
    df = pd.read_csv(CSV)
    df["mode"] = df["epoch_ms"].apply(lambda e: "immediate" if e == 0 else f"epoch={e}ms")
    return df


def plot_workload_modes(df: pd.DataFrame) -> None:
    base = df[df["run_tag"].str.startswith("base-")].copy()
    base["cell"] = base["workload"].str.upper().str[-1] + "/" + base["distribution"].str[:3]

    pivot = base.pivot_table(
        index="cell", columns="mode", values="throughput_txn_s"
    )[["immediate", "epoch=10ms"]]

    ax = pivot.plot(kind="bar", figsize=(7, 4), rot=0,
                    color=["#3a7", "#d62"])
    ax.set_ylabel("Throughput (txn/s)")
    ax.set_title("Throughput by workload × distribution × mode\n(threads=32, ops/txn=10, 1M records)")
    ax.legend(title="")
    ax.grid(axis="y", linestyle=":", alpha=0.4)
    for c in ax.containers:
        ax.bar_label(c, fmt="%.0f", padding=2, fontsize=8)
    plt.tight_layout()
    plt.savefig(PLOTS / "workload_modes.png", dpi=150)
    plt.close()


def plot_abort_rates(df: pd.DataFrame) -> None:
    base = df[df["run_tag"].str.startswith("base-")].copy()
    base["cell"] = base["workload"].str.upper().str[-1] + "/" + base["distribution"].str[:3]

    pivot = base.pivot_table(
        index="cell", columns="mode", values="abort_rate"
    )[["immediate", "epoch=10ms"]] * 100

    ax = pivot.plot(kind="bar", figsize=(7, 4), rot=0,
                    color=["#3a7", "#d62"])
    ax.set_ylabel("Abort rate (%)")
    ax.set_title("Abort rate by workload × distribution × mode")
    ax.legend(title="")
    ax.grid(axis="y", linestyle=":", alpha=0.4)
    for c in ax.containers:
        ax.bar_label(c, fmt="%.1f%%", padding=2, fontsize=8)
    plt.tight_layout()
    plt.savefig(PLOTS / "abort_rates.png", dpi=150)
    plt.close()


def plot_client_scaling(df: pd.DataFrame) -> None:
    scale = df[df["run_tag"].str.startswith("scale-c")].copy().sort_values("threads")

    fig, ax1 = plt.subplots(figsize=(7, 4))
    ax1.plot(scale["threads"], scale["throughput_txn_s"], "o-", color="#3a7", label="throughput")
    ax1.set_xlabel("Client threads")
    ax1.set_ylabel("Throughput (txn/s)", color="#3a7")
    ax1.tick_params(axis="y", labelcolor="#3a7")
    ax1.set_xscale("log", base=2)
    ax1.set_xticks(scale["threads"])
    ax1.set_xticklabels(scale["threads"])
    ax1.grid(True, linestyle=":", alpha=0.4)

    ax2 = ax1.twinx()
    ax2.plot(scale["threads"], scale["abort_rate"] * 100, "s--", color="#d62", label="abort rate")
    ax2.set_ylabel("Abort rate (%)", color="#d62")
    ax2.tick_params(axis="y", labelcolor="#d62")

    plt.title("Client scaling — YCSB-A zipfian, epoch=10ms\n(throughput sublinear, aborts climb)")
    fig.tight_layout()
    plt.savefig(PLOTS / "client_scaling.png", dpi=150)
    plt.close()


def plot_ops_per_txn(df: pd.DataFrame) -> None:
    ops = df[df["run_tag"].str.startswith("ops-")].copy().sort_values("ops_per_txn")

    fig, ax1 = plt.subplots(figsize=(7, 4))
    ax1.plot(ops["ops_per_txn"], ops["throughput_ops_s"], "o-", color="#3a7", label="ops/s")
    ax1.plot(ops["ops_per_txn"], ops["throughput_txn_s"], "v-", color="#28a", label="txn/s")
    ax1.set_xlabel("Operations per transaction")
    ax1.set_ylabel("Throughput", color="#3a7")
    ax1.set_xscale("log")
    ax1.set_xticks(ops["ops_per_txn"])
    ax1.set_xticklabels(ops["ops_per_txn"])
    ax1.grid(True, linestyle=":", alpha=0.4)
    ax1.legend(loc="upper right")

    ax2 = ax1.twinx()
    ax2.plot(ops["ops_per_txn"], ops["abort_rate"] * 100, "s--", color="#d62", label="abort %")
    ax2.set_ylabel("Abort rate (%)", color="#d62")
    ax2.tick_params(axis="y", labelcolor="#d62")

    plt.title("Throughput collapse with larger transactions — YCSB-A zipfian, epoch=10ms")
    fig.tight_layout()
    plt.savefig(PLOTS / "ops_per_txn.png", dpi=150)
    plt.close()


def plot_epoch_sweep(df: pd.DataFrame) -> None:
    # Combine the immediate baseline with the epoch sweep for a complete picture.
    rows = pd.concat([
        df[df["run_tag"] == "base-A-zip-im"][["epoch_ms", "throughput_txn_s", "abort_rate", "commit_avg_ms"]],
        df[df["run_tag"].str.match(r"base-A-zip-ep\d+$")][["epoch_ms", "throughput_txn_s", "abort_rate", "commit_avg_ms"]],
    ]).sort_values("epoch_ms").reset_index(drop=True)

    fig, ax1 = plt.subplots(figsize=(7, 4))
    ax1.plot(rows["epoch_ms"], rows["throughput_txn_s"], "o-", color="#3a7", label="throughput")
    ax1.set_xlabel("Epoch duration (ms);  0 = immediate")
    ax1.set_ylabel("Throughput (txn/s)", color="#3a7")
    ax1.tick_params(axis="y", labelcolor="#3a7")
    ax1.grid(True, linestyle=":", alpha=0.4)
    for x, y in zip(rows["epoch_ms"], rows["throughput_txn_s"]):
        ax1.annotate(f"{y:.0f}", (x, y), textcoords="offset points", xytext=(4, 6), fontsize=8)

    ax2 = ax1.twinx()
    ax2.plot(rows["epoch_ms"], rows["commit_avg_ms"], "^--", color="#888", label="commit avg ms")
    ax2.set_ylabel("Commit RPC avg (ms)", color="#888")
    ax2.tick_params(axis="y", labelcolor="#888")

    plt.title("Epoch sweep — YCSB-A zipfian, threads=32, ops/txn=10\n(immediate dominates at every duration)")
    fig.tight_layout()
    plt.savefig(PLOTS / "epoch_sweep.png", dpi=150)
    plt.close()


def write_tables(df: pd.DataFrame) -> None:
    parts: list[str] = []
    parts.append("# Summary Tables\n\n")
    parts.append("Generated from `results/all.csv` by `scripts/parse_results.py`.\n\n")

    base = df[df["run_tag"].str.startswith("base-")].copy()
    base["cell"] = base["workload"] + " " + base["distribution"]
    base = base[["cell", "mode", "throughput_txn_s", "abort_rate",
                 "avg_ms", "p95_ms", "commit_avg_ms"]].sort_values(["cell", "mode"])
    parts.append("## Base matrix (workload × distribution × mode)\n\n")
    parts.append(base.to_markdown(index=False, floatfmt=".3f"))
    parts.append("\n\n")

    scale = df[df["run_tag"].str.startswith("scale-c")].copy().sort_values("threads")
    scale = scale[["threads", "throughput_txn_s", "throughput_ops_s",
                   "abort_rate", "avg_ms", "p95_ms"]]
    parts.append("## Client scaling (YCSB-A zipfian, epoch=10ms)\n\n")
    parts.append(scale.to_markdown(index=False, floatfmt=".3f"))
    parts.append("\n\n")

    ops = df[df["run_tag"].str.startswith("ops-")].copy().sort_values("ops_per_txn")
    ops = ops[["ops_per_txn", "throughput_txn_s", "throughput_ops_s",
               "abort_rate", "avg_rs_bytes", "avg_ws_bytes"]]
    parts.append("## ops_per_txn sweep (YCSB-A zipfian, epoch=10ms, threads=32)\n\n")
    parts.append(ops.to_markdown(index=False, floatfmt=".3f"))
    parts.append("\n\n")

    epoch = df[df["run_tag"].str.match(r"base-A-zip-(im|ep\d+)$")].copy().sort_values("epoch_ms")
    epoch = epoch[["epoch_ms", "throughput_txn_s", "abort_rate",
                   "avg_ms", "commit_avg_ms"]]
    parts.append("## Epoch sweep (YCSB-A zipfian, threads=32, ops/txn=10)\n\n")
    parts.append(epoch.to_markdown(index=False, floatfmt=".3f"))
    parts.append("\n")

    TABLES.write_text("".join(parts))


def main() -> None:
    PLOTS.mkdir(parents=True, exist_ok=True)
    df = load()
    print(f"Loaded {len(df)} rows from {CSV.relative_to(REPO_ROOT)}")

    plot_workload_modes(df)
    plot_abort_rates(df)
    plot_client_scaling(df)
    plot_ops_per_txn(df)
    plot_epoch_sweep(df)
    write_tables(df)

    print(f"Wrote {len(list(PLOTS.glob('*.png')))} plots to {PLOTS.relative_to(REPO_ROOT)}/")
    print(f"Wrote tables to {TABLES.relative_to(REPO_ROOT)}")


if __name__ == "__main__":
    main()

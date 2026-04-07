package benchmark

import (
	"testing"
)

func TestDriver_SmallWorkload(t *testing.T) {
	config := DefaultConfig()
	config.NumClients = 2
	config.TotalTxnsPerClient = 10
	config.NumKeys = 50

	report := Run(config)

	if report.TotalCommitted == 0 {
		t.Fatal("expected at least one committed transaction")
	}
}

func TestDriver_MetricsConsistent(t *testing.T) {
	config := DefaultConfig()
	config.NumClients = 4
	config.TotalTxnsPerClient = 50
	config.NumKeys = 100
	config.RetryOnAbort = false // don't retry so total == clients * txnsPerClient

	report := Run(config)

	expectedTotal := uint64(config.NumClients * config.TotalTxnsPerClient)
	actualTotal := report.TotalCommitted + report.TotalAborted
	if actualTotal != expectedTotal {
		t.Fatalf("expected committed+aborted=%d, got %d (committed=%d, aborted=%d)",
			expectedTotal, actualTotal, report.TotalCommitted, report.TotalAborted)
	}
}

func TestDriver_ImmediateMode(t *testing.T) {
	config := DefaultConfig()
	config.NumClients = 2
	config.TotalTxnsPerClient = 20
	config.NumKeys = 100
	config.EpochDurationMs = 0

	report := Run(config)

	if report.Throughput <= 0 {
		t.Fatal("expected positive throughput")
	}
	if report.AvgLatency <= 0 {
		t.Fatal("expected positive avg latency")
	}
}

func TestDriver_EpochMode(t *testing.T) {
	config := DefaultConfig()
	config.NumClients = 2
	config.TotalTxnsPerClient = 20
	config.NumKeys = 100
	config.EpochDurationMs = 10

	report := Run(config)

	if report.Throughput <= 0 {
		t.Fatal("expected positive throughput")
	}
	if report.AvgLatency <= 0 {
		t.Fatal("expected positive avg latency")
	}
}

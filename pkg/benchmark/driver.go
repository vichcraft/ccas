package benchmark

import (
	"fmt"
	"math/rand"
	"strconv"
	"sync"
	"time"

	"github.com/vichcraft/ccas/pkg/ccaas"
	"github.com/vichcraft/ccas/pkg/execution"
	"github.com/vichcraft/ccas/pkg/metrics"
	"github.com/vichcraft/ccas/pkg/storage"
)

// RunConfig allows injecting external Store and Certifier (e.g., gRPC clients).
// If Store or Certifier is nil, they are created locally.
type RunConfig struct {
	Config    WorkloadConfig
	Store     storage.Store
	Certifier ccaas.Certifier
}

// Run executes a benchmark with the given configuration and returns the metrics report.
func Run(config WorkloadConfig) metrics.Report {
	return RunWith(RunConfig{Config: config})
}

// RunWith executes a benchmark with optional injected dependencies.
func RunWith(rc RunConfig) metrics.Report {
	config := rc.Config
	store := rc.Store
	var epochCertifier *ccaas.EpochCertifier

	if store == nil {
		ms := storage.NewMemStore()
		data := make(map[string]string, config.NumKeys)
		for i := range config.NumKeys {
			data[fmt.Sprintf("key-%d", i)] = "0"
		}
		ms.LoadInitialData(data)
		store = ms
	}

	certifier := rc.Certifier
	if certifier == nil {
		if config.EpochDurationMs > 0 {
			epochCertifier = ccaas.NewEpochCertifier(store, time.Duration(config.EpochDurationMs)*time.Millisecond)
			epochCertifier.Start()
			certifier = epochCertifier
		} else {
			certifier = ccaas.NewImmediateCertifier(store)
		}
	}

	engine := execution.NewEngine(store, certifier)
	collector := metrics.NewCollector()

	var wg sync.WaitGroup
	for i := range config.NumClients {
		wg.Add(1)
		go func(clientID int) {
			defer wg.Done()
			rng := rand.New(rand.NewSource(int64(clientID) + time.Now().UnixNano()))
			runClient(engine, config, collector, rng)
		}(i)
	}
	wg.Wait()

	if epochCertifier != nil {
		epochCertifier.Stop()
	}

	return collector.Report()
}

func runClient(engine *execution.Engine, config WorkloadConfig, collector *metrics.Collector, rng *rand.Rand) {
	completed := 0
	for completed < config.TotalTxnsPerClient {
		start := time.Now()

		txn := engine.Begin()

		// Pick keys to read.
		readKeys := pickKeys(config, rng, config.ReadsPerTxn)
		values := make(map[string]string)
		for _, key := range readKeys {
			val, _ := engine.Read(txn, key)
			values[key] = val
		}

		// Pick keys to write from the read set (read-modify-write pattern).
		writeKeys := pickFromSlice(readKeys, rng, config.WritesPerTxn)
		for _, key := range writeKeys {
			oldVal := values[key]
			n, _ := strconv.Atoi(oldVal)
			_ = engine.Write(txn, key, strconv.Itoa(n+1))
		}

		result, _ := engine.Commit(txn)
		latency := time.Since(start)

		if result.Committed {
			collector.RecordCommit(latency)
			completed++
		} else {
			collector.RecordAbort(latency)
			collector.RecordConflict()
			if !config.RetryOnAbort {
				completed++
			}
			// If RetryOnAbort, loop again without incrementing completed.
		}
	}
}

func pickKeys(config WorkloadConfig, rng *rand.Rand, count int) []string {
	seen := make(map[string]bool, count)
	keys := make([]string, 0, count)

	for len(keys) < count {
		var idx int
		if config.ContentionMode == "hotspot" {
			hotKeys := int(float64(config.NumKeys) * config.HotKeyFraction)
			if hotKeys < 1 {
				hotKeys = 1
			}
			if rng.Float64() < config.HotKeyAccessProb {
				idx = rng.Intn(hotKeys)
			} else {
				idx = rng.Intn(config.NumKeys)
			}
		} else {
			idx = rng.Intn(config.NumKeys)
		}

		key := fmt.Sprintf("key-%d", idx)
		if !seen[key] {
			seen[key] = true
			keys = append(keys, key)
		}
	}
	return keys
}

func pickFromSlice(keys []string, rng *rand.Rand, count int) []string {
	if count >= len(keys) {
		return keys
	}
	// Shuffle and take first `count`.
	shuffled := make([]string, len(keys))
	copy(shuffled, keys)
	rng.Shuffle(len(shuffled), func(i, j int) { shuffled[i], shuffled[j] = shuffled[j], shuffled[i] })
	return shuffled[:count]
}

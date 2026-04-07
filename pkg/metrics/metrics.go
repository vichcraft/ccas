package metrics

import (
	"sort"
	"sync"
	"time"
)

// Collector gathers benchmark metrics in a thread-safe manner.
type Collector struct {
	mu             sync.Mutex
	totalCommitted uint64
	totalAborted   uint64
	latencies      []time.Duration
	epochSizes     []int
	conflictCount  uint64
	startTime      time.Time
}

// Report holds the computed benchmark results.
type Report struct {
	TotalCommitted uint64
	TotalAborted   uint64
	AbortRate      float64
	Throughput     float64
	AvgLatency     time.Duration
	P95Latency     time.Duration
	AvgEpochSize   float64
	ConflictCount  uint64
	ElapsedTime    time.Duration
}

// NewCollector creates a new Collector and starts the elapsed time clock.
func NewCollector() *Collector {
	return &Collector{
		startTime: time.Now(),
	}
}

func (c *Collector) RecordCommit(latency time.Duration) {
	c.mu.Lock()
	c.totalCommitted++
	c.latencies = append(c.latencies, latency)
	c.mu.Unlock()
}

func (c *Collector) RecordAbort(latency time.Duration) {
	c.mu.Lock()
	c.totalAborted++
	c.latencies = append(c.latencies, latency)
	c.mu.Unlock()
}

func (c *Collector) RecordEpochSize(size int) {
	c.mu.Lock()
	c.epochSizes = append(c.epochSizes, size)
	c.mu.Unlock()
}

func (c *Collector) RecordConflict() {
	c.mu.Lock()
	c.conflictCount++
	c.mu.Unlock()
}

// Report computes and returns the final benchmark metrics.
func (c *Collector) Report() Report {
	c.mu.Lock()
	defer c.mu.Unlock()

	elapsed := time.Since(c.startTime)
	total := c.totalCommitted + c.totalAborted

	r := Report{
		TotalCommitted: c.totalCommitted,
		TotalAborted:   c.totalAborted,
		ConflictCount:  c.conflictCount,
		ElapsedTime:    elapsed,
	}

	if total > 0 {
		r.AbortRate = float64(c.totalAborted) / float64(total)
	}

	if elapsed > 0 {
		r.Throughput = float64(c.totalCommitted) / elapsed.Seconds()
	}

	if len(c.latencies) > 0 {
		var sum time.Duration
		for _, l := range c.latencies {
			sum += l
		}
		r.AvgLatency = sum / time.Duration(len(c.latencies))

		sorted := make([]time.Duration, len(c.latencies))
		copy(sorted, c.latencies)
		sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })
		p95idx := int(float64(len(sorted)) * 0.95)
		if p95idx >= len(sorted) {
			p95idx = len(sorted) - 1
		}
		r.P95Latency = sorted[p95idx]
	}

	if len(c.epochSizes) > 0 {
		sum := 0
		for _, s := range c.epochSizes {
			sum += s
		}
		r.AvgEpochSize = float64(sum) / float64(len(c.epochSizes))
	}

	return r
}

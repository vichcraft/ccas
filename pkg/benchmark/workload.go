package benchmark

// WorkloadConfig defines the parameters for a benchmark run.
type WorkloadConfig struct {
	NumClients         int
	NumKeys            int
	ReadsPerTxn        int
	WritesPerTxn       int
	TotalTxnsPerClient int
	ContentionMode     string  // "uniform" or "hotspot"
	HotKeyFraction     float64 // fraction of keys that are "hot"
	HotKeyAccessProb   float64 // probability of accessing a hot key
	EpochDurationMs    int     // 0 = immediate mode
	RetryOnAbort       bool
}

// DefaultConfig returns a balanced default workload.
func DefaultConfig() WorkloadConfig {
	return WorkloadConfig{
		NumClients:         4,
		NumKeys:            1000,
		ReadsPerTxn:        3,
		WritesPerTxn:       2,
		TotalTxnsPerClient: 1000,
		ContentionMode:     "uniform",
		HotKeyFraction:     0.1,
		HotKeyAccessProb:   0.9,
		EpochDurationMs:    0,
		RetryOnAbort:       true,
	}
}

// LowContentionReadHeavy returns a read-heavy uniform workload.
func LowContentionReadHeavy() WorkloadConfig {
	c := DefaultConfig()
	c.NumKeys = 1000
	c.ReadsPerTxn = 8
	c.WritesPerTxn = 2
	c.ContentionMode = "uniform"
	return c
}

// HighContentionWriteHeavy returns a write-heavy hotspot workload.
func HighContentionWriteHeavy() WorkloadConfig {
	c := DefaultConfig()
	c.NumKeys = 100
	c.ReadsPerTxn = 5
	c.WritesPerTxn = 5
	c.ContentionMode = "hotspot"
	c.HotKeyFraction = 0.1
	c.HotKeyAccessProb = 0.9
	return c
}

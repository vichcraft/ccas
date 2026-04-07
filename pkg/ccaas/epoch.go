package ccaas

import (
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/vichcraft/ccas/pkg/storage"
	"github.com/vichcraft/ccas/pkg/types"
)

type pendingTxn struct {
	req      types.TxnRequest
	resultCh chan types.TxnResult
}

// EpochCertifier batches transactions over a configurable epoch duration
// and certifies them together using deterministic conflict resolution.
type EpochCertifier struct {
	mu            sync.Mutex // protects latestVersion and commitSeq during epoch processing
	latestVersion map[string]uint64
	commitSeq     uint64
	store         storage.Store

	epochDuration time.Duration
	batchMu       sync.Mutex
	currentBatch  []pendingTxn
	ticker        *time.Ticker
	stopCh        chan struct{}
	doneCh        chan struct{}
}

// NewEpochCertifier creates an EpochCertifier initialized from storage state.
// Call Start() to begin epoch processing.
func NewEpochCertifier(store storage.Store, epochDuration time.Duration) *EpochCertifier {
	state := store.DumpState()
	latestVersion := make(map[string]uint64, len(state))

	var maxVersion uint64
	for key, result := range state {
		latestVersion[key] = result.Version
		if result.Version > maxVersion {
			maxVersion = result.Version
		}
	}

	return &EpochCertifier{
		latestVersion: latestVersion,
		commitSeq:     maxVersion,
		store:         store,
		epochDuration: epochDuration,
		stopCh:        make(chan struct{}),
		doneCh:        make(chan struct{}),
	}
}

// SyncFromStorage re-initializes the version map from the current storage state.
func (c *EpochCertifier) SyncFromStorage() {
	c.mu.Lock()
	defer c.mu.Unlock()

	state := c.store.DumpState()
	c.latestVersion = make(map[string]uint64, len(state))
	var maxVersion uint64
	for key, result := range state {
		c.latestVersion[key] = result.Version
		if result.Version > maxVersion {
			maxVersion = result.Version
		}
	}
	if maxVersion > c.commitSeq {
		c.commitSeq = maxVersion
	}
}

// Start begins the background epoch processing goroutine.
func (c *EpochCertifier) Start() {
	c.ticker = time.NewTicker(c.epochDuration)
	go c.run()
}

// Stop halts epoch processing and drains any remaining batch.
func (c *EpochCertifier) Stop() {
	c.ticker.Stop()
	close(c.stopCh)
	<-c.doneCh
}

// SubmitCommit queues a transaction for the next epoch and blocks until the result is ready.
func (c *EpochCertifier) SubmitCommit(req types.TxnRequest) types.TxnResult {
	p := pendingTxn{
		req:      req,
		resultCh: make(chan types.TxnResult, 1),
	}

	c.batchMu.Lock()
	c.currentBatch = append(c.currentBatch, p)
	c.batchMu.Unlock()

	return <-p.resultCh
}

func (c *EpochCertifier) run() {
	defer close(c.doneCh)
	for {
		select {
		case <-c.ticker.C:
			c.processEpoch()
		case <-c.stopCh:
			c.processEpoch() // drain remaining batch
			return
		}
	}
}

func (c *EpochCertifier) processEpoch() {
	// Swap out the current batch.
	c.batchMu.Lock()
	batch := c.currentBatch
	c.currentBatch = nil
	c.batchMu.Unlock()

	if len(batch) == 0 {
		return
	}

	// Panic safety: abort all pending txns if something goes wrong.
	defer func() {
		if r := recover(); r != nil {
			for _, p := range batch {
				select {
				case p.resultCh <- types.TxnResult{
					TxnID:  p.req.TxnID,
					Reason: fmt.Sprintf("epoch panic: %v", r),
				}:
				default:
				}
			}
		}
	}()

	// Sort by TxnID for deterministic ordering.
	sort.Slice(batch, func(i, j int) bool {
		return batch[i].req.TxnID < batch[j].req.TxnID
	})

	c.mu.Lock()
	defer c.mu.Unlock()

	claimedKeys := make(map[string]string) // key -> winning txnID

	for _, p := range batch {
		result := c.certifyTxn(p.req, claimedKeys)
		p.resultCh <- result
	}
}

func (c *EpochCertifier) certifyTxn(req types.TxnRequest, claimedKeys map[string]string) types.TxnResult {
	// Read-set validation against latestVersion (cross-epoch).
	for _, read := range req.ReadSet {
		currentVersion, ok := c.latestVersion[read.Key]
		if ok {
			if currentVersion != read.VersionSeen {
				return types.TxnResult{
					TxnID:  req.TxnID,
					Reason: fmt.Sprintf("stale read on key %s: expected version %d, found %d", read.Key, read.VersionSeen, currentVersion),
				}
			}
		} else if read.VersionSeen != 0 {
			return types.TxnResult{
				TxnID:  req.TxnID,
				Reason: fmt.Sprintf("stale read on key %s: expected version %d, found 0", read.Key, read.VersionSeen),
			}
		}
	}

	// Write-write conflict detection within the epoch.
	for _, write := range req.WriteSet {
		if winner, taken := claimedKeys[write.Key]; taken && winner != req.TxnID {
			return types.TxnResult{
				TxnID:  req.TxnID,
				Reason: fmt.Sprintf("write-write conflict on key %s: claimed by %s", write.Key, winner),
			}
		}
	}

	// Commit: claim keys, assign version, apply.
	c.commitSeq++
	commitSeq := c.commitSeq

	for _, write := range req.WriteSet {
		claimedKeys[write.Key] = req.TxnID
	}

	record := types.CommitRecord{
		CommitSeq: commitSeq,
		TxnID:     req.TxnID,
		Writes:    make([]types.CommitWrite, 0, len(req.WriteSet)),
	}
	for _, write := range req.WriteSet {
		record.Writes = append(record.Writes, types.CommitWrite{
			Key:        write.Key,
			Value:      write.Value,
			NewVersion: commitSeq,
		})
		c.latestVersion[write.Key] = commitSeq
	}

	_ = c.store.ApplyCommit(record)

	return types.TxnResult{
		TxnID:     req.TxnID,
		Committed: true,
		CommitSeq: commitSeq,
	}
}

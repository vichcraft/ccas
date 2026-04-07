package ccaas

import (
	"fmt"
	"sync"

	"github.com/vichcraft/ccas/pkg/storage"
	"github.com/vichcraft/ccas/pkg/types"
)

// ImmediateCertifier certifies one transaction at a time, synchronously.
type ImmediateCertifier struct {
	mu            sync.Mutex
	latestVersion map[string]uint64
	commitSeq     uint64
	store         storage.Store
}

// NewImmediateCertifier creates an ImmediateCertifier initialized from storage state.
func NewImmediateCertifier(store storage.Store) *ImmediateCertifier {
	state := store.DumpState()
	latestVersion := make(map[string]uint64, len(state))

	var maxVersion uint64
	for key, result := range state {
		latestVersion[key] = result.Version
		if result.Version > maxVersion {
			maxVersion = result.Version
		}
	}

	return &ImmediateCertifier{
		latestVersion: latestVersion,
		commitSeq:     maxVersion,
		store:         store,
	}
}

// SyncFromStorage re-initializes the version map from the current storage state.
func (c *ImmediateCertifier) SyncFromStorage() {
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

// SubmitCommit validates a transaction's read set and, if valid, commits its writes.
func (c *ImmediateCertifier) SubmitCommit(req types.TxnRequest) types.TxnResult {
	c.mu.Lock()
	defer c.mu.Unlock()

	for _, read := range req.ReadSet {
		currentVersion, ok := c.latestVersion[read.Key]
		if ok {
			if currentVersion != read.VersionSeen {
				return types.TxnResult{
					TxnID:     req.TxnID,
					Committed: false,
					Reason: fmt.Sprintf(
						"stale read on key %s: expected version %d, found %d",
						read.Key, read.VersionSeen, currentVersion,
					),
				}
			}
			continue
		}

		if read.VersionSeen != 0 {
			return types.TxnResult{
				TxnID:     req.TxnID,
				Committed: false,
				Reason: fmt.Sprintf(
					"stale read on key %s: expected version %d, found 0",
					read.Key, read.VersionSeen,
				),
			}
		}
	}

	c.commitSeq++
	commitSeq := c.commitSeq
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
	}

	for _, write := range req.WriteSet {
		c.latestVersion[write.Key] = commitSeq
	}

	if err := c.store.ApplyCommit(record); err != nil {
		return types.TxnResult{
			TxnID:     req.TxnID,
			Committed: false,
			Reason:    err.Error(),
		}
	}

	return types.TxnResult{
		TxnID:     req.TxnID,
		Committed: true,
		CommitSeq: commitSeq,
	}
}

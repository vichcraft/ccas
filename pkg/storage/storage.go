package storage

import (
	"github.com/vichcraft/ccas/pkg/types"
	"fmt"
	"sync"
)

// Store is the storage layer interface.
// In a multi-process setup, a gRPC client would implement this same interface.
type Store interface {
	Get(key string) types.ReadResult
	ApplyCommit(record types.CommitRecord) error
	LoadInitialData(data map[string]string)
	DumpState() map[string]types.ReadResult
}

type entry struct {
	value   string
	version uint64
}

// MemStore is an in-memory implementation of Store.
type MemStore struct {
	mu   sync.RWMutex
	data map[string]entry
}

// NewMemStore creates a new empty MemStore.
func NewMemStore() *MemStore {
	return &MemStore{
		data: make(map[string]entry),
	}
}

func (s *MemStore) Get(key string) types.ReadResult {
	s.mu.RLock()
	defer s.mu.RUnlock()

	e, ok := s.data[key]
	if !ok {
		return types.ReadResult{Found: false}
	}
	return types.ReadResult{
		Value:   e.value,
		Version: e.version,
		Found:   true,
	}
}

func (s *MemStore) ApplyCommit(record types.CommitRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Validate version monotonicity before applying any writes.
	for _, w := range record.Writes {
		if existing, ok := s.data[w.Key]; ok {
			if w.NewVersion <= existing.version {
				return fmt.Errorf(
					"version monotonicity violation on key %q: new version %d <= current version %d",
					w.Key, w.NewVersion, existing.version,
				)
			}
		}
	}

	// All checks passed — apply all writes.
	for _, w := range record.Writes {
		s.data[w.Key] = entry{value: w.Value, version: w.NewVersion}
	}
	return nil
}

func (s *MemStore) LoadInitialData(data map[string]string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for k, v := range data {
		s.data[k] = entry{value: v, version: 1}
	}
}

func (s *MemStore) DumpState() map[string]types.ReadResult {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make(map[string]types.ReadResult, len(s.data))
	for k, e := range s.data {
		result[k] = types.ReadResult{
			Value:   e.value,
			Version: e.version,
			Found:   true,
		}
	}
	return result
}

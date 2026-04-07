package ccaas

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/vichcraft/ccas/pkg/storage"
	"github.com/vichcraft/ccas/pkg/types"
)

func setupEpoch(data map[string]string, epochDuration time.Duration) (*EpochCertifier, *storage.MemStore) {
	store := storage.NewMemStore()
	if data != nil {
		store.LoadInitialData(data)
	}
	certifier := NewEpochCertifier(store, epochDuration)
	return certifier, store
}

func TestEpoch_SingleTxn(t *testing.T) {
	certifier, store := setupEpoch(map[string]string{"x": "old"}, 10*time.Millisecond)
	certifier.Start()
	defer certifier.Stop()

	result := certifier.SubmitCommit(types.TxnRequest{
		TxnID:    "txn-1",
		ReadSet:  []types.ReadEntry{{Key: "x", VersionSeen: 1}},
		WriteSet: []types.WriteEntry{{Key: "x", Value: "new"}},
	})

	if !result.Committed {
		t.Fatalf("expected commit, got abort: %s", result.Reason)
	}

	r := store.Get("x")
	if r.Value != "new" {
		t.Fatalf("expected storage value %q, got %q", "new", r.Value)
	}
}

func TestEpoch_BatchMultiple(t *testing.T) {
	certifier, _ := setupEpoch(map[string]string{
		"a": "1", "b": "2", "c": "3", "d": "4", "e": "5",
	}, 50*time.Millisecond)
	certifier.Start()
	defer certifier.Stop()

	var wg sync.WaitGroup
	results := make([]types.TxnResult, 5)

	for i := range 5 {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			key := string(rune('a' + idx))
			results[idx] = certifier.SubmitCommit(types.TxnRequest{
				TxnID:    fmt.Sprintf("txn-%d", idx),
				ReadSet:  []types.ReadEntry{{Key: key, VersionSeen: 1}},
				WriteSet: []types.WriteEntry{{Key: key, Value: "updated"}},
			})
		}(i)
	}
	wg.Wait()

	for i, r := range results {
		if !r.Committed {
			t.Fatalf("txn-%d expected commit, got abort: %s", i, r.Reason)
		}
	}
}

func TestEpoch_WriteConflictInBatch(t *testing.T) {
	certifier, _ := setupEpoch(map[string]string{"x": "v0"}, 50*time.Millisecond)
	certifier.Start()
	defer certifier.Stop()

	var wg sync.WaitGroup
	results := make([]types.TxnResult, 2)

	// Both txns write key "x". Since sorted by TxnID, "txn-a" < "txn-b", so txn-a wins.
	for i, id := range []string{"txn-a", "txn-b"} {
		wg.Add(1)
		go func(idx int, txnID string) {
			defer wg.Done()
			results[idx] = certifier.SubmitCommit(types.TxnRequest{
				TxnID:    txnID,
				ReadSet:  []types.ReadEntry{{Key: "x", VersionSeen: 1}},
				WriteSet: []types.WriteEntry{{Key: "x", Value: txnID}},
			})
		}(i, id)
	}
	wg.Wait()

	if !results[0].Committed {
		t.Fatalf("txn-a should commit, got abort: %s", results[0].Reason)
	}
	if results[1].Committed {
		t.Fatal("txn-b should abort due to write-write conflict")
	}
}

func TestEpoch_StaleReadInBatch(t *testing.T) {
	certifier, store := setupEpoch(map[string]string{"x": "v0"}, 50*time.Millisecond)

	// Pre-commit to advance version of x.
	_ = store.ApplyCommit(types.CommitRecord{
		CommitSeq: 2,
		TxnID:     "setup",
		Writes:    []types.CommitWrite{{Key: "x", Value: "v1", NewVersion: 2}},
	})
	// Re-create certifier to pick up new state.
	certifier = NewEpochCertifier(store, 50*time.Millisecond)
	certifier.Start()
	defer certifier.Stop()

	result := certifier.SubmitCommit(types.TxnRequest{
		TxnID:    "txn-stale",
		ReadSet:  []types.ReadEntry{{Key: "x", VersionSeen: 1}}, // stale
		WriteSet: []types.WriteEntry{{Key: "x", Value: "v2"}},
	})

	if result.Committed {
		t.Fatal("expected abort due to stale read")
	}
}

func TestEpoch_CrossEpochConflict(t *testing.T) {
	certifier, _ := setupEpoch(map[string]string{"x": "v0"}, 10*time.Millisecond)
	certifier.Start()
	defer certifier.Stop()

	// Epoch 1: commit a write to x.
	r1 := certifier.SubmitCommit(types.TxnRequest{
		TxnID:    "txn-1",
		ReadSet:  []types.ReadEntry{{Key: "x", VersionSeen: 1}},
		WriteSet: []types.WriteEntry{{Key: "x", Value: "v1"}},
	})
	if !r1.Committed {
		t.Fatalf("epoch 1 txn should commit: %s", r1.Reason)
	}

	// Epoch 2: txn that read x at old version should abort.
	r2 := certifier.SubmitCommit(types.TxnRequest{
		TxnID:    "txn-2",
		ReadSet:  []types.ReadEntry{{Key: "x", VersionSeen: 1}}, // stale
		WriteSet: []types.WriteEntry{{Key: "x", Value: "v2"}},
	})
	if r2.Committed {
		t.Fatal("epoch 2 txn with stale read should abort")
	}
}

func TestEpoch_DeterministicOrder(t *testing.T) {
	certifier, _ := setupEpoch(map[string]string{"x": "v0"}, 50*time.Millisecond)
	certifier.Start()
	defer certifier.Stop()

	var wg sync.WaitGroup
	results := make(map[string]types.TxnResult)
	var mu sync.Mutex

	// Submit txns in non-sorted order. All write to "x" so only one can win.
	// Deterministic order by TxnID: "txn-1" < "txn-2" < "txn-3", so txn-1 wins.
	for _, id := range []string{"txn-3", "txn-1", "txn-2"} {
		wg.Add(1)
		go func(txnID string) {
			defer wg.Done()
			r := certifier.SubmitCommit(types.TxnRequest{
				TxnID:    txnID,
				ReadSet:  []types.ReadEntry{{Key: "x", VersionSeen: 1}},
				WriteSet: []types.WriteEntry{{Key: "x", Value: txnID}},
			})
			mu.Lock()
			results[txnID] = r
			mu.Unlock()
		}(id)
	}
	wg.Wait()

	if !results["txn-1"].Committed {
		t.Fatalf("txn-1 (lowest ID) should win, got abort: %s", results["txn-1"].Reason)
	}
	if results["txn-2"].Committed {
		t.Fatal("txn-2 should abort")
	}
	if results["txn-3"].Committed {
		t.Fatal("txn-3 should abort")
	}
}

func TestEpoch_EmptyEpoch(t *testing.T) {
	certifier, _ := setupEpoch(nil, 10*time.Millisecond)
	certifier.Start()

	// Let a few empty epochs pass.
	time.Sleep(50 * time.Millisecond)

	certifier.Stop() // should not hang or crash
}

func TestEpoch_ConcurrentSubmitters(t *testing.T) {
	data := make(map[string]string)
	for i := range 50 {
		data[fmt.Sprintf("key-%d", i)] = "0"
	}
	certifier, _ := setupEpoch(data, 10*time.Millisecond)
	certifier.Start()
	defer certifier.Stop()

	var wg sync.WaitGroup
	results := make([]types.TxnResult, 50)

	for i := range 50 {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			key := fmt.Sprintf("key-%d", idx)
			results[idx] = certifier.SubmitCommit(types.TxnRequest{
				TxnID:    fmt.Sprintf("txn-%03d", idx),
				ReadSet:  []types.ReadEntry{{Key: key, VersionSeen: 1}},
				WriteSet: []types.WriteEntry{{Key: key, Value: "updated"}},
			})
		}(i)
	}
	wg.Wait()

	committed := 0
	for _, r := range results {
		if r.Committed {
			committed++
		}
	}

	// Each txn writes a unique key, so all should commit.
	if committed != 50 {
		t.Fatalf("expected all 50 txns to commit (unique keys), got %d", committed)
	}
}

func TestEpoch_StopDrainsBatch(t *testing.T) {
	certifier, _ := setupEpoch(map[string]string{"a": "1", "b": "2", "c": "3"}, 1*time.Hour)
	certifier.Start()

	// Submit txns — they won't be processed because epoch is 1 hour.
	var wg sync.WaitGroup
	results := make([]types.TxnResult, 3)

	for i := range 3 {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			key := string(rune('a' + idx))
			results[idx] = certifier.SubmitCommit(types.TxnRequest{
				TxnID:    fmt.Sprintf("txn-%d", idx),
				ReadSet:  []types.ReadEntry{{Key: key, VersionSeen: 1}},
				WriteSet: []types.WriteEntry{{Key: key, Value: "drained"}},
			})
		}(i)
	}

	// Give goroutines time to submit, then stop.
	time.Sleep(20 * time.Millisecond)
	certifier.Stop()

	wg.Wait()

	for i, r := range results {
		if !r.Committed {
			t.Fatalf("txn-%d should have been drained and committed on Stop, got abort: %s", i, r.Reason)
		}
	}
}

package storage

import (
	"github.com/vichcraft/ccas/pkg/types"
	"sync"
	"testing"
)

func TestGet_Empty(t *testing.T) {
	s := NewMemStore()
	result := s.Get("nonexistent")
	if result.Found {
		t.Fatal("expected Found=false for missing key")
	}
	if result.Value != "" {
		t.Fatalf("expected empty value, got %q", result.Value)
	}
	if result.Version != 0 {
		t.Fatalf("expected version 0, got %d", result.Version)
	}
}

func TestLoadInitialData(t *testing.T) {
	s := NewMemStore()
	s.LoadInitialData(map[string]string{
		"a": "100",
		"b": "200",
		"c": "300",
	})

	tests := []struct {
		key   string
		value string
	}{
		{"a", "100"},
		{"b", "200"},
		{"c", "300"},
	}

	for _, tt := range tests {
		r := s.Get(tt.key)
		if !r.Found {
			t.Fatalf("key %q not found", tt.key)
		}
		if r.Value != tt.value {
			t.Fatalf("key %q: expected value %q, got %q", tt.key, tt.value, r.Value)
		}
		if r.Version != 1 {
			t.Fatalf("key %q: expected version 1, got %d", tt.key, r.Version)
		}
	}
}

func TestApplyCommit_Single(t *testing.T) {
	s := NewMemStore()
	s.LoadInitialData(map[string]string{"x": "old"})

	err := s.ApplyCommit(types.CommitRecord{
		CommitSeq: 1,
		TxnID:     "txn-1",
		Writes: []types.CommitWrite{
			{Key: "x", Value: "new", NewVersion: 2},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	r := s.Get("x")
	if !r.Found {
		t.Fatal("key x not found after commit")
	}
	if r.Value != "new" {
		t.Fatalf("expected value %q, got %q", "new", r.Value)
	}
	if r.Version != 2 {
		t.Fatalf("expected version 2, got %d", r.Version)
	}
}

func TestApplyCommit_MultipleWrites(t *testing.T) {
	s := NewMemStore()
	s.LoadInitialData(map[string]string{"a": "1", "b": "2"})

	err := s.ApplyCommit(types.CommitRecord{
		CommitSeq: 1,
		TxnID:     "txn-1",
		Writes: []types.CommitWrite{
			{Key: "a", Value: "10", NewVersion: 2},
			{Key: "b", Value: "20", NewVersion: 2},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ra := s.Get("a")
	rb := s.Get("b")
	if ra.Value != "10" || ra.Version != 2 {
		t.Fatalf("key a: got value=%q version=%d", ra.Value, ra.Version)
	}
	if rb.Value != "20" || rb.Version != 2 {
		t.Fatalf("key b: got value=%q version=%d", rb.Value, rb.Version)
	}
}

func TestApplyCommit_Sequential(t *testing.T) {
	s := NewMemStore()
	s.LoadInitialData(map[string]string{"x": "v0"})

	err := s.ApplyCommit(types.CommitRecord{
		CommitSeq: 1,
		TxnID:     "txn-1",
		Writes:    []types.CommitWrite{{Key: "x", Value: "v1", NewVersion: 2}},
	})
	if err != nil {
		t.Fatalf("first commit failed: %v", err)
	}

	err = s.ApplyCommit(types.CommitRecord{
		CommitSeq: 2,
		TxnID:     "txn-2",
		Writes:    []types.CommitWrite{{Key: "x", Value: "v2", NewVersion: 3}},
	})
	if err != nil {
		t.Fatalf("second commit failed: %v", err)
	}

	r := s.Get("x")
	if r.Value != "v2" || r.Version != 3 {
		t.Fatalf("expected value=%q version=3, got value=%q version=%d", "v2", r.Value, r.Version)
	}
}

func TestApplyCommit_VersionMonotonicity(t *testing.T) {
	s := NewMemStore()
	s.LoadInitialData(map[string]string{"x": "v0"})

	// First commit: version 1 -> 2
	err := s.ApplyCommit(types.CommitRecord{
		CommitSeq: 1,
		TxnID:     "txn-1",
		Writes:    []types.CommitWrite{{Key: "x", Value: "v1", NewVersion: 2}},
	})
	if err != nil {
		t.Fatalf("first commit failed: %v", err)
	}

	// Try to apply with version <= current (2). Should fail.
	err = s.ApplyCommit(types.CommitRecord{
		CommitSeq: 2,
		TxnID:     "txn-2",
		Writes:    []types.CommitWrite{{Key: "x", Value: "bad", NewVersion: 2}},
	})
	if err == nil {
		t.Fatal("expected error for version monotonicity violation, got nil")
	}

	// Try with version less than current.
	err = s.ApplyCommit(types.CommitRecord{
		CommitSeq: 3,
		TxnID:     "txn-3",
		Writes:    []types.CommitWrite{{Key: "x", Value: "bad", NewVersion: 1}},
	})
	if err == nil {
		t.Fatal("expected error for version monotonicity violation, got nil")
	}

	// Value should be unchanged.
	r := s.Get("x")
	if r.Value != "v1" || r.Version != 2 {
		t.Fatalf("expected value=%q version=2 (unchanged), got value=%q version=%d", "v1", r.Value, r.Version)
	}
}

func TestDumpState(t *testing.T) {
	s := NewMemStore()
	s.LoadInitialData(map[string]string{"a": "1", "b": "2"})

	_ = s.ApplyCommit(types.CommitRecord{
		CommitSeq: 1,
		TxnID:     "txn-1",
		Writes:    []types.CommitWrite{{Key: "a", Value: "10", NewVersion: 2}},
	})

	state := s.DumpState()

	if len(state) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(state))
	}
	if state["a"].Value != "10" || state["a"].Version != 2 {
		t.Fatalf("key a: got value=%q version=%d", state["a"].Value, state["a"].Version)
	}
	if state["b"].Value != "2" || state["b"].Version != 1 {
		t.Fatalf("key b: got value=%q version=%d", state["b"].Value, state["b"].Version)
	}
}

func TestConcurrentReads(t *testing.T) {
	s := NewMemStore()
	s.LoadInitialData(map[string]string{
		"x": "100",
		"y": "200",
		"z": "300",
	})

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				rx := s.Get("x")
				ry := s.Get("y")
				rz := s.Get("z")
				if !rx.Found || !ry.Found || !rz.Found {
					t.Error("key not found during concurrent read")
					return
				}
			}
		}()
	}
	wg.Wait()
}

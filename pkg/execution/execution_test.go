package execution

import (
	"testing"

	"github.com/vichcraft/ccas/pkg/ccaas"
	"github.com/vichcraft/ccas/pkg/storage"
	"github.com/vichcraft/ccas/pkg/types"
)

func setup(data map[string]string) (*Engine, *storage.MemStore) {
	store := storage.NewMemStore()
	if data != nil {
		store.LoadInitialData(data)
	}
	certifier := ccaas.NewImmediateCertifier(store)
	engine := NewEngine(store, certifier)
	return engine, store
}

func TestBegin_UniqueIDs(t *testing.T) {
	engine, _ := setup(nil)

	seen := make(map[string]bool)
	for i := 0; i < 100; i++ {
		txn := engine.Begin()
		if seen[txn.ID] {
			t.Fatalf("duplicate txn ID: %s", txn.ID)
		}
		seen[txn.ID] = true
	}
}

func TestRead_RecordsReadSet(t *testing.T) {
	engine, _ := setup(map[string]string{"x": "100"})

	txn := engine.Begin()
	val, err := engine.Read(txn, "x")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if val != "100" {
		t.Fatalf("expected %q, got %q", "100", val)
	}
	if len(txn.ReadSet) != 1 {
		t.Fatalf("expected 1 read-set entry, got %d", len(txn.ReadSet))
	}
	if txn.ReadSet[0].Key != "x" || txn.ReadSet[0].VersionSeen != 1 {
		t.Fatalf("unexpected read-set entry: %+v", txn.ReadSet[0])
	}
}

func TestWrite_BuffersLocally(t *testing.T) {
	engine, store := setup(map[string]string{"x": "old"})

	txn := engine.Begin()
	err := engine.Write(txn, "x", "new")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(txn.WriteSet) != 1 {
		t.Fatalf("expected 1 write-set entry, got %d", len(txn.WriteSet))
	}
	if txn.WriteSet[0].Key != "x" || txn.WriteSet[0].Value != "new" {
		t.Fatalf("unexpected write-set entry: %+v", txn.WriteSet[0])
	}

	// Storage should be unchanged.
	r := store.Get("x")
	if r.Value != "old" {
		t.Fatalf("expected storage unchanged, got %q", r.Value)
	}
}

func TestRead_OwnWrite(t *testing.T) {
	engine, _ := setup(map[string]string{"x": "old"})

	txn := engine.Begin()
	_ = engine.Write(txn, "x", "buffered")

	val, err := engine.Read(txn, "x")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if val != "buffered" {
		t.Fatalf("expected %q, got %q", "buffered", val)
	}

	// No read-set entry should exist — read was served from write buffer.
	if len(txn.ReadSet) != 0 {
		t.Fatalf("expected 0 read-set entries, got %d", len(txn.ReadSet))
	}
}

func TestRead_SameKeyTwice(t *testing.T) {
	engine, _ := setup(map[string]string{"x": "100"})

	txn := engine.Begin()
	_, _ = engine.Read(txn, "x")
	_, _ = engine.Read(txn, "x")

	if len(txn.ReadSet) != 1 {
		t.Fatalf("expected 1 read-set entry after two reads, got %d", len(txn.ReadSet))
	}
}

func TestCommit_BuildsCorrectRequest(t *testing.T) {
	// Use a recording certifier to inspect the request.
	store := storage.NewMemStore()
	store.LoadInitialData(map[string]string{"a": "1", "b": "2"})

	var captured types.TxnRequest
	recorder := &recordingCertifier{
		inner:    ccaas.NewImmediateCertifier(store),
		onSubmit: func(req types.TxnRequest) { captured = req },
	}

	engine := NewEngine(store, recorder)
	txn := engine.Begin()
	_, _ = engine.Read(txn, "a")
	_, _ = engine.Read(txn, "b")
	_ = engine.Write(txn, "a", "10")
	_, _ = engine.Commit(txn)

	if captured.TxnID != txn.ID {
		t.Fatalf("expected TxnID %q, got %q", txn.ID, captured.TxnID)
	}
	if len(captured.ReadSet) != 2 {
		t.Fatalf("expected 2 read-set entries, got %d", len(captured.ReadSet))
	}
	if len(captured.WriteSet) != 1 {
		t.Fatalf("expected 1 write-set entry, got %d", len(captured.WriteSet))
	}
	if captured.WriteSet[0].Key != "a" || captured.WriteSet[0].Value != "10" {
		t.Fatalf("unexpected write-set entry: %+v", captured.WriteSet[0])
	}
}

func TestAbort_MarksState(t *testing.T) {
	engine, _ := setup(map[string]string{"x": "1"})

	txn := engine.Begin()
	engine.Abort(txn)

	_, err := engine.Read(txn, "x")
	if err != ErrTxnAborted {
		t.Fatalf("expected ErrTxnAborted, got %v", err)
	}

	err = engine.Write(txn, "x", "2")
	if err != ErrTxnAborted {
		t.Fatalf("expected ErrTxnAborted, got %v", err)
	}

	_, err = engine.Commit(txn)
	if err != ErrTxnAborted {
		t.Fatalf("expected ErrTxnAborted, got %v", err)
	}
}

func TestCommit_ThenOpsFail(t *testing.T) {
	engine, _ := setup(map[string]string{"x": "1"})

	txn := engine.Begin()
	_, _ = engine.Read(txn, "x")
	_ = engine.Write(txn, "x", "2")
	_, err := engine.Commit(txn)
	if err != nil {
		t.Fatalf("unexpected commit error: %v", err)
	}

	_, err = engine.Read(txn, "x")
	if err != ErrTxnCommitted {
		t.Fatalf("expected ErrTxnCommitted, got %v", err)
	}

	err = engine.Write(txn, "x", "3")
	if err != ErrTxnCommitted {
		t.Fatalf("expected ErrTxnCommitted, got %v", err)
	}
}

// recordingCertifier wraps a real certifier and records the request.
type recordingCertifier struct {
	inner    ccaas.Certifier
	onSubmit func(types.TxnRequest)
}

func (r *recordingCertifier) SubmitCommit(req types.TxnRequest) types.TxnResult {
	if r.onSubmit != nil {
		r.onSubmit(req)
	}
	return r.inner.SubmitCommit(req)
}

func (r *recordingCertifier) SyncFromStorage() {
	r.inner.SyncFromStorage()
}

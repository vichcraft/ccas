package ccaas

import (
	"testing"

	"github.com/vichcraft/ccas/pkg/storage"
	"github.com/vichcraft/ccas/pkg/types"
)

func TestCommit_NoConflict(t *testing.T) {
	store := storage.NewMemStore()
	store.LoadInitialData(map[string]string{"x": "old"})
	certifier := NewImmediateCertifier(store)

	result := certifier.SubmitCommit(types.TxnRequest{
		TxnID:   "txn-1",
		ReadSet: []types.ReadEntry{{Key: "x", VersionSeen: 1}},
		WriteSet: []types.WriteEntry{
			{Key: "x", Value: "new"},
		},
	})

	if !result.Committed {
		t.Fatalf("expected commit, got abort: %s", result.Reason)
	}
	if result.CommitSeq != 2 {
		t.Fatalf("expected commit seq 2, got %d", result.CommitSeq)
	}
}

func TestCommit_StaleRead(t *testing.T) {
	store := storage.NewMemStore()
	store.LoadInitialData(map[string]string{"x": "v1"})
	certifier := NewImmediateCertifier(store)

	first := certifier.SubmitCommit(types.TxnRequest{
		TxnID:    "txn-1",
		ReadSet:  []types.ReadEntry{{Key: "x", VersionSeen: 1}},
		WriteSet: []types.WriteEntry{{Key: "x", Value: "v2"}},
	})
	if !first.Committed {
		t.Fatalf("expected first commit, got abort: %s", first.Reason)
	}

	second := certifier.SubmitCommit(types.TxnRequest{
		TxnID:    "txn-2",
		ReadSet:  []types.ReadEntry{{Key: "x", VersionSeen: 1}},
		WriteSet: []types.WriteEntry{{Key: "x", Value: "v3"}},
	})
	if second.Committed {
		t.Fatal("expected stale read transaction to abort")
	}
}

func TestCommit_WriteOnly(t *testing.T) {
	store := storage.NewMemStore()
	certifier := NewImmediateCertifier(store)

	result := certifier.SubmitCommit(types.TxnRequest{
		TxnID:    "txn-1",
		WriteSet: []types.WriteEntry{{Key: "x", Value: "value"}},
	})

	if !result.Committed {
		t.Fatalf("expected write-only transaction to commit, got abort: %s", result.Reason)
	}
	if result.CommitSeq != 1 {
		t.Fatalf("expected commit seq 1, got %d", result.CommitSeq)
	}
}

func TestCommit_WriteConflict_Sequential(t *testing.T) {
	store := storage.NewMemStore()
	store.LoadInitialData(map[string]string{"x": "v1"})
	certifier := NewImmediateCertifier(store)

	resultA := certifier.SubmitCommit(types.TxnRequest{
		TxnID:    "txn-a",
		ReadSet:  []types.ReadEntry{{Key: "x", VersionSeen: 1}},
		WriteSet: []types.WriteEntry{{Key: "x", Value: "v2"}},
	})
	if !resultA.Committed {
		t.Fatalf("expected txn A to commit, got abort: %s", resultA.Reason)
	}

	resultB := certifier.SubmitCommit(types.TxnRequest{
		TxnID:    "txn-b",
		ReadSet:  []types.ReadEntry{{Key: "x", VersionSeen: 1}},
		WriteSet: []types.WriteEntry{{Key: "x", Value: "v3"}},
	})
	if resultB.Committed {
		t.Fatal("expected txn B to abort on stale read")
	}
}

func TestCommit_VersionProgression(t *testing.T) {
	store := storage.NewMemStore()
	store.LoadInitialData(map[string]string{"a": "1", "b": "2"})
	certifier := NewImmediateCertifier(store)

	result1 := certifier.SubmitCommit(types.TxnRequest{
		TxnID:    "txn-1",
		ReadSet:  []types.ReadEntry{{Key: "a", VersionSeen: 1}},
		WriteSet: []types.WriteEntry{{Key: "a", Value: "10"}},
	})
	if !result1.Committed {
		t.Fatalf("expected first commit, got abort: %s", result1.Reason)
	}

	result2 := certifier.SubmitCommit(types.TxnRequest{
		TxnID:    "txn-2",
		ReadSet:  []types.ReadEntry{{Key: "b", VersionSeen: 1}},
		WriteSet: []types.WriteEntry{{Key: "b", Value: "20"}},
	})
	if !result2.Committed {
		t.Fatalf("expected second commit, got abort: %s", result2.Reason)
	}

	if certifier.commitSeq != 3 {
		t.Fatalf("expected commitSeq 3, got %d", certifier.commitSeq)
	}
	if certifier.latestVersion["a"] != 2 {
		t.Fatalf("expected latest version for a to be 2, got %d", certifier.latestVersion["a"])
	}
	if certifier.latestVersion["b"] != 3 {
		t.Fatalf("expected latest version for b to be 3, got %d", certifier.latestVersion["b"])
	}
}

func TestCommit_MultipleKeys(t *testing.T) {
	store := storage.NewMemStore()
	store.LoadInitialData(map[string]string{
		"a": "1",
		"b": "2",
		"c": "3",
	})
	certifier := NewImmediateCertifier(store)

	update := certifier.SubmitCommit(types.TxnRequest{
		TxnID:    "txn-update",
		ReadSet:  []types.ReadEntry{{Key: "b", VersionSeen: 1}},
		WriteSet: []types.WriteEntry{{Key: "b", Value: "20"}},
	})
	if !update.Committed {
		t.Fatalf("expected update commit, got abort: %s", update.Reason)
	}

	result := certifier.SubmitCommit(types.TxnRequest{
		TxnID: "txn-1",
		ReadSet: []types.ReadEntry{
			{Key: "a", VersionSeen: 1},
			{Key: "b", VersionSeen: 1},
			{Key: "c", VersionSeen: 1},
		},
		WriteSet: []types.WriteEntry{{Key: "a", Value: "10"}},
	})

	if result.Committed {
		t.Fatal("expected transaction with one stale key to abort")
	}
}

func TestCommit_StorageUpdated(t *testing.T) {
	store := storage.NewMemStore()
	store.LoadInitialData(map[string]string{"x": "old"})
	certifier := NewImmediateCertifier(store)

	result := certifier.SubmitCommit(types.TxnRequest{
		TxnID:    "txn-1",
		ReadSet:  []types.ReadEntry{{Key: "x", VersionSeen: 1}},
		WriteSet: []types.WriteEntry{{Key: "x", Value: "new"}},
	})
	if !result.Committed {
		t.Fatalf("expected commit, got abort: %s", result.Reason)
	}

	read := store.Get("x")
	if !read.Found {
		t.Fatal("expected key x to exist after commit")
	}
	if read.Value != "new" {
		t.Fatalf("expected value %q, got %q", "new", read.Value)
	}
	if read.Version != result.CommitSeq {
		t.Fatalf("expected version %d, got %d", result.CommitSeq, read.Version)
	}
}

func TestCommit_ReadNonExistentKey(t *testing.T) {
	store := storage.NewMemStore()
	certifier := NewImmediateCertifier(store)

	result := certifier.SubmitCommit(types.TxnRequest{
		TxnID:   "txn-1",
		ReadSet: []types.ReadEntry{{Key: "missing", VersionSeen: 0}},
		WriteSet: []types.WriteEntry{
			{Key: "x", Value: "value"},
		},
	})

	if !result.Committed {
		t.Fatalf("expected commit when reading non-existent key at version 0, got abort: %s", result.Reason)
	}
}

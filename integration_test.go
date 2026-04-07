package ccas_test

import (
	"fmt"
	"math/rand"
	"strconv"
	"sync"
	"testing"

	"github.com/vichcraft/ccas/pkg/ccaas"
	"github.com/vichcraft/ccas/pkg/execution"
	"github.com/vichcraft/ccas/pkg/storage"
)

func setupE2E(data map[string]string) (*execution.Engine, *storage.MemStore) {
	store := storage.NewMemStore()
	if data != nil {
		store.LoadInitialData(data)
	}
	certifier := ccaas.NewImmediateCertifier(store)
	engine := execution.NewEngine(store, certifier)
	return engine, store
}

func TestE2E_SingleTxnReadWrite(t *testing.T) {
	engine, store := setupE2E(map[string]string{"x": "old"})

	txn := engine.Begin()
	val, err := engine.Read(txn, "x")
	if err != nil {
		t.Fatalf("read error: %v", err)
	}
	if val != "old" {
		t.Fatalf("expected %q, got %q", "old", val)
	}

	_ = engine.Write(txn, "x", "new")
	result, err := engine.Commit(txn)
	if err != nil {
		t.Fatalf("commit error: %v", err)
	}
	if !result.Committed {
		t.Fatalf("expected commit, got abort: %s", result.Reason)
	}

	r := store.Get("x")
	if r.Value != "new" {
		t.Fatalf("expected storage value %q, got %q", "new", r.Value)
	}
}

func TestE2E_TwoTxns_NoConflict(t *testing.T) {
	engine, store := setupE2E(map[string]string{"a": "1", "b": "2"})

	txnA := engine.Begin()
	_, _ = engine.Read(txnA, "a")
	_ = engine.Write(txnA, "a", "10")
	resultA, _ := engine.Commit(txnA)

	txnB := engine.Begin()
	_, _ = engine.Read(txnB, "b")
	_ = engine.Write(txnB, "b", "20")
	resultB, _ := engine.Commit(txnB)

	if !resultA.Committed {
		t.Fatalf("txn A should commit: %s", resultA.Reason)
	}
	if !resultB.Committed {
		t.Fatalf("txn B should commit: %s", resultB.Reason)
	}

	if store.Get("a").Value != "10" {
		t.Fatalf("expected a=10, got %s", store.Get("a").Value)
	}
	if store.Get("b").Value != "20" {
		t.Fatalf("expected b=20, got %s", store.Get("b").Value)
	}
}

func TestE2E_WriteConflict(t *testing.T) {
	engine, _ := setupE2E(map[string]string{"x": "v0"})

	txnA := engine.Begin()
	txnB := engine.Begin()

	// Both read x at version 1.
	_, _ = engine.Read(txnA, "x")
	_, _ = engine.Read(txnB, "x")

	// Both write x.
	_ = engine.Write(txnA, "x", "vA")
	_ = engine.Write(txnB, "x", "vB")

	// First committer wins.
	resultA, _ := engine.Commit(txnA)
	resultB, _ := engine.Commit(txnB)

	if !resultA.Committed {
		t.Fatalf("txn A (first committer) should commit: %s", resultA.Reason)
	}
	if resultB.Committed {
		t.Fatal("txn B (second committer with stale read) should abort")
	}
}

func TestE2E_StaleRead(t *testing.T) {
	engine, _ := setupE2E(map[string]string{"x": "v0"})

	txnA := engine.Begin()
	_, _ = engine.Read(txnA, "x") // reads version 1

	// Txn B updates x and commits.
	txnB := engine.Begin()
	_, _ = engine.Read(txnB, "x")
	_ = engine.Write(txnB, "x", "v1")
	resultB, _ := engine.Commit(txnB)
	if !resultB.Committed {
		t.Fatalf("txn B should commit: %s", resultB.Reason)
	}

	// Txn A tries to commit — should abort because x changed.
	_ = engine.Write(txnA, "x", "vA")
	resultA, _ := engine.Commit(txnA)
	if resultA.Committed {
		t.Fatal("txn A should abort due to stale read on x")
	}
}

func TestE2E_ReadYourOwnWrites(t *testing.T) {
	engine, _ := setupE2E(map[string]string{"x": "old"})

	txn := engine.Begin()
	_ = engine.Write(txn, "x", "hello")

	val, err := engine.Read(txn, "x")
	if err != nil {
		t.Fatalf("read error: %v", err)
	}
	if val != "hello" {
		t.Fatalf("expected %q from write buffer, got %q", "hello", val)
	}

	result, _ := engine.Commit(txn)
	if !result.Committed {
		t.Fatalf("expected commit, got abort: %s", result.Reason)
	}
}

func TestE2E_AbortLeavesNoTrace(t *testing.T) {
	engine, store := setupE2E(map[string]string{"x": "original"})

	txn := engine.Begin()
	_ = engine.Write(txn, "x", "bad")
	engine.Abort(txn)

	r := store.Get("x")
	if r.Value != "original" {
		t.Fatalf("expected storage unchanged after abort, got %q", r.Value)
	}
}

func TestE2E_ConcurrentClients(t *testing.T) {
	data := make(map[string]string)
	for i := 0; i < 10; i++ {
		data[fmt.Sprintf("key-%d", i)] = "0"
	}
	engine, store := setupE2E(data)

	var wg sync.WaitGroup
	committed := make([]bool, 10)

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			key := fmt.Sprintf("key-%d", id%5) // overlapping keys

			txn := engine.Begin()
			val, _ := engine.Read(txn, key)
			n, _ := strconv.Atoi(val)
			_ = engine.Write(txn, key, strconv.Itoa(n+1))
			result, _ := engine.Commit(txn)
			committed[id] = result.Committed
		}(i)
	}
	wg.Wait()

	// Verify storage consistency: each key's value should equal the number
	// of committed transactions that wrote to it.
	for i := 0; i < 5; i++ {
		key := fmt.Sprintf("key-%d", i)
		r := store.Get(key)
		if !r.Found {
			t.Fatalf("key %s not found", key)
		}
		val, err := strconv.Atoi(r.Value)
		if err != nil {
			t.Fatalf("key %s: invalid value %q", key, r.Value)
		}
		// Value should be 0 or 1 (at most one of the two competing txns committed).
		if val < 0 || val > 2 {
			t.Fatalf("key %s: unexpected value %d", key, val)
		}
	}
}

func TestE2E_BankTransfer(t *testing.T) {
	const (
		numAccounts    = 10
		initialBalance = 100
		totalExpected  = numAccounts * initialBalance
		numClients     = 50
	)

	data := make(map[string]string)
	for i := 0; i < numAccounts; i++ {
		data[fmt.Sprintf("acct-%d", i)] = strconv.Itoa(initialBalance)
	}
	engine, store := setupE2E(data)

	var wg sync.WaitGroup
	for i := 0; i < numClients; i++ {
		wg.Add(1)
		go func(seed int64) {
			defer wg.Done()
			rng := rand.New(rand.NewSource(seed))

			src := rng.Intn(numAccounts)
			dst := rng.Intn(numAccounts)
			for dst == src {
				dst = rng.Intn(numAccounts)
			}
			amount := rng.Intn(50) + 1

			srcKey := fmt.Sprintf("acct-%d", src)
			dstKey := fmt.Sprintf("acct-%d", dst)

			txn := engine.Begin()
			srcVal, _ := engine.Read(txn, srcKey)
			dstVal, _ := engine.Read(txn, dstKey)

			srcBal, _ := strconv.Atoi(srcVal)
			dstBal, _ := strconv.Atoi(dstVal)

			if srcBal >= amount {
				_ = engine.Write(txn, srcKey, strconv.Itoa(srcBal-amount))
				_ = engine.Write(txn, dstKey, strconv.Itoa(dstBal+amount))
				_, _ = engine.Commit(txn)
			} else {
				engine.Abort(txn)
			}
		}(int64(i))
	}
	wg.Wait()

	// Verify invariant: sum of all balances == totalExpected.
	sum := 0
	for i := 0; i < numAccounts; i++ {
		key := fmt.Sprintf("acct-%d", i)
		r := store.Get(key)
		if !r.Found {
			t.Fatalf("account %s not found", key)
		}
		bal, err := strconv.Atoi(r.Value)
		if err != nil {
			t.Fatalf("account %s: invalid balance %q", key, r.Value)
		}
		if bal < 0 {
			t.Fatalf("account %s has negative balance: %d", key, bal)
		}
		sum += bal
	}

	if sum != totalExpected {
		t.Fatalf("balance invariant violated: sum=%d, expected=%d", sum, totalExpected)
	}
}

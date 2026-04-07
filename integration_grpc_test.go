package ccas_test

import (
	"fmt"
	"math/rand"
	"net"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/vichcraft/ccas/pkg/ccaas"
	"github.com/vichcraft/ccas/pkg/execution"
	"github.com/vichcraft/ccas/pkg/storage"
	pb "github.com/vichcraft/ccas/proto/ccaspb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// setupGRPCE2E starts in-process storage and CCaaS gRPC servers,
// returns an Engine wired through gRPC clients and the storage client for assertions.
func setupGRPCE2E(t *testing.T, data map[string]string, epochMs int) (*execution.Engine, *storage.GRPCStoreClient) {
	t.Helper()

	// Storage server.
	memStore := storage.NewMemStore()
	if data != nil {
		memStore.LoadInitialData(data)
	}
	storageSrv := grpc.NewServer()
	pb.RegisterStorageServiceServer(storageSrv, storage.NewGRPCStorageServer(memStore))
	storageLis, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		t.Fatalf("storage listen: %v", err)
	}
	go func() { _ = storageSrv.Serve(storageLis) }()
	t.Cleanup(storageSrv.GracefulStop)

	storageConn, err := grpc.NewClient(storageLis.Addr().String(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("storage connect: %v", err)
	}
	t.Cleanup(func() { storageConn.Close() })
	storeClient := storage.NewGRPCStoreClient(storageConn)

	// CCaaS server.
	var certifier ccaas.Certifier
	if epochMs > 0 {
		ec := ccaas.NewEpochCertifier(storeClient, time.Duration(epochMs)*time.Millisecond)
		ec.Start()
		t.Cleanup(ec.Stop)
		certifier = ec
	} else {
		certifier = ccaas.NewImmediateCertifier(storeClient)
	}

	ccaasSrv := grpc.NewServer()
	pb.RegisterCertifierServiceServer(ccaasSrv, ccaas.NewGRPCCertifierServer(certifier))
	ccaasLis, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		t.Fatalf("ccaas listen: %v", err)
	}
	go func() { _ = ccaasSrv.Serve(ccaasLis) }()
	t.Cleanup(ccaasSrv.GracefulStop)

	ccaasConn, err := grpc.NewClient(ccaasLis.Addr().String(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("ccaas connect: %v", err)
	}
	t.Cleanup(func() { ccaasConn.Close() })
	certClient := ccaas.NewGRPCCertifierClient(ccaasConn)

	engine := execution.NewEngine(storeClient, certClient)
	return engine, storeClient
}

func TestGRPC_E2E_SingleTxnReadWrite(t *testing.T) {
	engine, store := setupGRPCE2E(t, map[string]string{"x": "old"}, 0)

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

func TestGRPC_E2E_WriteConflict(t *testing.T) {
	engine, _ := setupGRPCE2E(t, map[string]string{"x": "v0"}, 0)

	txnA := engine.Begin()
	txnB := engine.Begin()

	_, _ = engine.Read(txnA, "x")
	_, _ = engine.Read(txnB, "x")

	_ = engine.Write(txnA, "x", "vA")
	_ = engine.Write(txnB, "x", "vB")

	resultA, _ := engine.Commit(txnA)
	resultB, _ := engine.Commit(txnB)

	if !resultA.Committed {
		t.Fatalf("txn A should commit: %s", resultA.Reason)
	}
	if resultB.Committed {
		t.Fatal("txn B should abort due to stale read")
	}
}

func TestGRPC_E2E_BankTransfer(t *testing.T) {
	const (
		accounts    = 10
		initBalance = 100
		expected    = accounts * initBalance
		clients     = 20
	)

	data := make(map[string]string, accounts)
	for i := range accounts {
		data[fmt.Sprintf("acct-%d", i)] = strconv.Itoa(initBalance)
	}
	engine, store := setupGRPCE2E(t, data, 0)

	var wg sync.WaitGroup
	for i := range clients {
		wg.Add(1)
		go func(seed int64) {
			defer wg.Done()
			rng := rand.New(rand.NewSource(seed))

			for j := 0; j < 20; j++ {
				src := rng.Intn(accounts)
				dst := rng.Intn(accounts)
				for dst == src {
					dst = rng.Intn(accounts)
				}
				amount := rng.Intn(30) + 1

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
			}
		}(int64(i))
	}
	wg.Wait()

	sum := 0
	for i := range accounts {
		key := fmt.Sprintf("acct-%d", i)
		r := store.Get(key)
		bal, _ := strconv.Atoi(r.Value)
		sum += bal
	}

	if sum != expected {
		t.Fatalf("balance invariant violated: sum=%d, expected=%d", sum, expected)
	}
}

func TestGRPC_E2E_EpochMode(t *testing.T) {
	engine, store := setupGRPCE2E(t, map[string]string{"a": "1", "b": "2"}, 10)

	// Two txns on disjoint keys should both commit.
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

	ra := store.Get("a")
	rb := store.Get("b")
	if ra.Value != "10" {
		t.Fatalf("expected a=10, got %s", ra.Value)
	}
	if rb.Value != "20" {
		t.Fatalf("expected b=20, got %s", rb.Value)
	}
}

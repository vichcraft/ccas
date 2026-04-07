package ccaas

import (
	"net"
	"sync"
	"testing"

	"github.com/vichcraft/ccas/pkg/storage"
	"github.com/vichcraft/ccas/pkg/types"
	pb "github.com/vichcraft/ccas/proto/ccaspb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// setupGRPCCertifier starts an in-process storage server and CCaaS server,
// returns a gRPC certifier client and the storage client for setup.
func setupGRPCCertifier(t *testing.T, data map[string]string) (*GRPCCertifierClient, *storage.GRPCStoreClient) {
	t.Helper()

	// Start storage server.
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

	// Start CCaaS server using the remote store client.
	certifier := NewImmediateCertifier(storeClient)
	ccaasSrv := grpc.NewServer()
	pb.RegisterCertifierServiceServer(ccaasSrv, NewGRPCCertifierServer(certifier))

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

	return NewGRPCCertifierClient(ccaasConn), storeClient
}

func TestGRPC_Certifier_Commit(t *testing.T) {
	certClient, storeClient := setupGRPCCertifier(t, map[string]string{"x": "old"})

	result := certClient.SubmitCommit(types.TxnRequest{
		TxnID:    "txn-1",
		ReadSet:  []types.ReadEntry{{Key: "x", VersionSeen: 1}},
		WriteSet: []types.WriteEntry{{Key: "x", Value: "new"}},
	})

	if !result.Committed {
		t.Fatalf("expected commit, got abort: %s", result.Reason)
	}

	r := storeClient.Get("x")
	if r.Value != "new" {
		t.Fatalf("expected storage value %q, got %q", "new", r.Value)
	}
}

func TestGRPC_Certifier_StaleRead(t *testing.T) {
	certClient, _ := setupGRPCCertifier(t, map[string]string{"x": "v0"})

	// First commit succeeds.
	r1 := certClient.SubmitCommit(types.TxnRequest{
		TxnID:    "txn-1",
		ReadSet:  []types.ReadEntry{{Key: "x", VersionSeen: 1}},
		WriteSet: []types.WriteEntry{{Key: "x", Value: "v1"}},
	})
	if !r1.Committed {
		t.Fatalf("first commit should succeed: %s", r1.Reason)
	}

	// Second commit with stale read aborts.
	r2 := certClient.SubmitCommit(types.TxnRequest{
		TxnID:    "txn-2",
		ReadSet:  []types.ReadEntry{{Key: "x", VersionSeen: 1}},
		WriteSet: []types.WriteEntry{{Key: "x", Value: "v2"}},
	})
	if r2.Committed {
		t.Fatal("second commit with stale read should abort")
	}
}

func TestGRPC_Certifier_Concurrent(t *testing.T) {
	data := make(map[string]string, 10)
	for i := range 10 {
		data[string(rune('a'+i))] = "0"
	}
	certClient, _ := setupGRPCCertifier(t, data)

	var wg sync.WaitGroup
	results := make([]types.TxnResult, 10)

	for i := range 10 {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			key := string(rune('a' + idx))
			results[idx] = certClient.SubmitCommit(types.TxnRequest{
				TxnID:    key + "-txn",
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
	// All write unique keys, so all should commit.
	if committed != 10 {
		t.Fatalf("expected 10 commits, got %d", committed)
	}
}

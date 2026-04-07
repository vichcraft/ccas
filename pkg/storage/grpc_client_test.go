package storage

import (
	"net"
	"testing"

	"github.com/vichcraft/ccas/pkg/types"
	pb "github.com/vichcraft/ccas/proto/ccaspb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func setupGRPCStore(t *testing.T) *GRPCStoreClient {
	t.Helper()

	store := NewMemStore()
	srv := grpc.NewServer()
	pb.RegisterStorageServiceServer(srv, NewGRPCStorageServer(store))

	lis, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		t.Fatalf("failed to listen: %v", err)
	}

	go func() { _ = srv.Serve(lis) }()
	t.Cleanup(srv.GracefulStop)

	conn, err := grpc.NewClient(lis.Addr().String(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	t.Cleanup(func() { conn.Close() })

	return NewGRPCStoreClient(conn)
}

func TestGRPC_Get_Empty(t *testing.T) {
	client := setupGRPCStore(t)

	result := client.Get("nonexistent")
	if result.Found {
		t.Fatal("expected Found=false for missing key")
	}
}

func TestGRPC_LoadAndGet(t *testing.T) {
	client := setupGRPCStore(t)

	client.LoadInitialData(map[string]string{"a": "100", "b": "200"})

	ra := client.Get("a")
	if !ra.Found || ra.Value != "100" || ra.Version != 1 {
		t.Fatalf("key a: got found=%v value=%q version=%d", ra.Found, ra.Value, ra.Version)
	}

	rb := client.Get("b")
	if !rb.Found || rb.Value != "200" || rb.Version != 1 {
		t.Fatalf("key b: got found=%v value=%q version=%d", rb.Found, rb.Value, rb.Version)
	}
}

func TestGRPC_ApplyCommit(t *testing.T) {
	client := setupGRPCStore(t)

	client.LoadInitialData(map[string]string{"x": "old"})

	err := client.ApplyCommit(types.CommitRecord{
		CommitSeq: 1,
		TxnID:     "txn-1",
		Writes:    []types.CommitWrite{{Key: "x", Value: "new", NewVersion: 2}},
	})
	if err != nil {
		t.Fatalf("apply commit failed: %v", err)
	}

	r := client.Get("x")
	if r.Value != "new" || r.Version != 2 {
		t.Fatalf("expected value=%q version=2, got value=%q version=%d", "new", r.Value, r.Version)
	}
}

func TestGRPC_DumpState(t *testing.T) {
	client := setupGRPCStore(t)

	client.LoadInitialData(map[string]string{"a": "1", "b": "2"})

	state := client.DumpState()
	if len(state) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(state))
	}
	if state["a"].Value != "1" || state["a"].Version != 1 {
		t.Fatalf("key a: got value=%q version=%d", state["a"].Value, state["a"].Version)
	}
	if state["b"].Value != "2" || state["b"].Version != 1 {
		t.Fatalf("key b: got value=%q version=%d", state["b"].Value, state["b"].Version)
	}
}

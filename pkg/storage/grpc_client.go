package storage

import (
	"context"
	"errors"
	"log"

	"github.com/vichcraft/ccas/pkg/types"
	pb "github.com/vichcraft/ccas/proto/ccaspb"
	"google.golang.org/grpc"
)

// GRPCStoreClient implements Store via gRPC calls to a remote StorageService.
type GRPCStoreClient struct {
	client pb.StorageServiceClient
}

// NewGRPCStoreClient creates a Store backed by a gRPC connection.
func NewGRPCStoreClient(conn *grpc.ClientConn) *GRPCStoreClient {
	return &GRPCStoreClient{client: pb.NewStorageServiceClient(conn)}
}

func (c *GRPCStoreClient) Get(key string) types.ReadResult {
	resp, err := c.client.Get(context.Background(), &pb.GetRequest{Key: key})
	if err != nil {
		return types.ReadResult{Found: false}
	}
	return types.ReadResult{
		Value:   resp.Value,
		Version: resp.Version,
		Found:   resp.Found,
	}
}

func (c *GRPCStoreClient) ApplyCommit(record types.CommitRecord) error {
	writes := make([]*pb.CommitWrite, len(record.Writes))
	for i, w := range record.Writes {
		writes[i] = &pb.CommitWrite{
			Key:        w.Key,
			Value:      w.Value,
			NewVersion: w.NewVersion,
		}
	}

	resp, err := c.client.ApplyCommit(context.Background(), &pb.ApplyCommitRequest{
		CommitSeq: record.CommitSeq,
		TxnId:     record.TxnID,
		Writes:    writes,
	})
	if err != nil {
		return err
	}
	if resp.Error != "" {
		return errors.New(resp.Error)
	}
	return nil
}

func (c *GRPCStoreClient) LoadInitialData(data map[string]string) {
	if _, err := c.client.LoadInitialData(context.Background(), &pb.LoadInitialDataRequest{Data: data}); err != nil {
		log.Printf("storage.LoadInitialData(%d records) failed: %v", len(data), err)
	}
}

func (c *GRPCStoreClient) DumpState() map[string]types.ReadResult {
	resp, err := c.client.DumpState(context.Background(), &pb.DumpStateRequest{})
	if err != nil {
		log.Printf("storage.DumpState failed: %v", err)
		return nil
	}
	result := make(map[string]types.ReadResult, len(resp.Entries))
	for k, e := range resp.Entries {
		result[k] = types.ReadResult{
			Value:   e.Value,
			Version: e.Version,
			Found:   true,
		}
	}
	return result
}

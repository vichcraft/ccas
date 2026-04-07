package storage

import (
	"context"

	"github.com/vichcraft/ccas/pkg/types"
	pb "github.com/vichcraft/ccas/proto/ccaspb"
)

// GRPCStorageServer implements the StorageService gRPC server by wrapping a Store.
type GRPCStorageServer struct {
	pb.UnimplementedStorageServiceServer
	store Store
}

// NewGRPCStorageServer creates a gRPC server handler wrapping the given Store.
func NewGRPCStorageServer(store Store) *GRPCStorageServer {
	return &GRPCStorageServer{store: store}
}

func (s *GRPCStorageServer) Get(_ context.Context, req *pb.GetRequest) (*pb.GetResponse, error) {
	r := s.store.Get(req.Key)
	return &pb.GetResponse{
		Value:   r.Value,
		Version: r.Version,
		Found:   r.Found,
	}, nil
}

func (s *GRPCStorageServer) ApplyCommit(_ context.Context, req *pb.ApplyCommitRequest) (*pb.ApplyCommitResponse, error) {
	record := types.CommitRecord{
		CommitSeq: req.CommitSeq,
		TxnID:     req.TxnId,
		Writes:    make([]types.CommitWrite, len(req.Writes)),
	}
	for i, w := range req.Writes {
		record.Writes[i] = types.CommitWrite{
			Key:        w.Key,
			Value:      w.Value,
			NewVersion: w.NewVersion,
		}
	}

	errMsg := ""
	if err := s.store.ApplyCommit(record); err != nil {
		errMsg = err.Error()
	}
	return &pb.ApplyCommitResponse{Error: errMsg}, nil
}

func (s *GRPCStorageServer) LoadInitialData(_ context.Context, req *pb.LoadInitialDataRequest) (*pb.LoadInitialDataResponse, error) {
	s.store.LoadInitialData(req.Data)
	return &pb.LoadInitialDataResponse{}, nil
}

func (s *GRPCStorageServer) DumpState(_ context.Context, _ *pb.DumpStateRequest) (*pb.DumpStateResponse, error) {
	state := s.store.DumpState()
	entries := make(map[string]*pb.DumpStateEntry, len(state))
	for k, v := range state {
		entries[k] = &pb.DumpStateEntry{
			Value:   v.Value,
			Version: v.Version,
		}
	}
	return &pb.DumpStateResponse{Entries: entries}, nil
}

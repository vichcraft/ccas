package ccaas

import (
	"context"

	"github.com/vichcraft/ccas/pkg/types"
	pb "github.com/vichcraft/ccas/proto/ccaspb"
)

// GRPCCertifierServer implements the CertifierService gRPC server by wrapping a Certifier.
type GRPCCertifierServer struct {
	pb.UnimplementedCertifierServiceServer
	certifier Certifier
}

// NewGRPCCertifierServer creates a gRPC server handler wrapping the given Certifier.
func NewGRPCCertifierServer(certifier Certifier) *GRPCCertifierServer {
	return &GRPCCertifierServer{certifier: certifier}
}

func (s *GRPCCertifierServer) SyncFromStorage(_ context.Context, _ *pb.SyncFromStorageRequest) (*pb.SyncFromStorageResponse, error) {
	s.certifier.SyncFromStorage()
	return &pb.SyncFromStorageResponse{}, nil
}

func (s *GRPCCertifierServer) SubmitCommit(_ context.Context, req *pb.SubmitCommitRequest) (*pb.SubmitCommitResponse, error) {
	txnReq := types.TxnRequest{
		TxnID:    req.TxnId,
		ReadSet:  make([]types.ReadEntry, len(req.ReadSet)),
		WriteSet: make([]types.WriteEntry, len(req.WriteSet)),
	}
	for i, r := range req.ReadSet {
		txnReq.ReadSet[i] = types.ReadEntry{Key: r.Key, VersionSeen: r.VersionSeen}
	}
	for i, w := range req.WriteSet {
		txnReq.WriteSet[i] = types.WriteEntry{Key: w.Key, Value: w.Value}
	}

	result := s.certifier.SubmitCommit(txnReq)

	return &pb.SubmitCommitResponse{
		TxnId:     result.TxnID,
		Committed: result.Committed,
		CommitSeq: result.CommitSeq,
		Reason:    result.Reason,
	}, nil
}
